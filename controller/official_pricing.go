package controller

import (
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service/official_pricing"
	"github.com/QuantumNous/new-api/setting/ratio_setting"

	"github.com/gin-gonic/gin"
)

// officialPriceBookOptionKey persists the built book in the options table
// directly (via model.SaveRawOption), bypassing OptionMap so GetOptions never
// ships this large blob to the frontend.
const officialPriceBookOptionKey = "OfficialPriceBookCache"

// loadOfficialBook returns the in-memory book, lazily restoring it from
// persistence on a cold start. Returns nil if no book has ever been built.
func loadOfficialBook() *official_pricing.Book {
	if b := official_pricing.GetCachedBook(); b != nil {
		return b
	}
	raw, err := model.GetRawOptionValue(officialPriceBookOptionKey)
	if err != nil || raw == "" {
		return nil
	}
	var b official_pricing.Book
	if err := common.UnmarshalJsonStr(raw, &b); err != nil {
		common.SysError("failed to decode cached official price book: " + err.Error())
		return nil
	}
	official_pricing.SetCachedBook(&b)
	return &b
}

func currentRatios() official_pricing.CurrentRatios {
	return official_pricing.CurrentRatios{
		ModelRatio:       ratio_setting.GetModelRatioCopy(),
		CompletionRatio:  ratio_setting.GetCompletionRatioCopy(),
		CacheRatio:       ratio_setting.GetCacheRatioCopy(),
		CreateCacheRatio: ratio_setting.GetCreateCacheRatioCopy(),
	}
}

// RefreshOfficialPriceBook fetches models.dev, rebuilds the official price book,
// caches it in memory and persists it.
func RefreshOfficialPriceBook(c *gin.Context) {
	data, err := official_pricing.FetchModelsDev(c.Request.Context(), 20*time.Second)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "拉取 models.dev 失败：" + err.Error()})
		return
	}
	providers, err := official_pricing.ParseModelsDev(data)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析 models.dev 数据失败：" + err.Error()})
		return
	}
	book := official_pricing.BuildBook(providers, time.Now().Unix())
	official_pricing.SetCachedBook(book)

	if jsonBytes, mErr := common.Marshal(book); mErr == nil {
		if sErr := model.SaveRawOption(officialPriceBookOptionKey, string(jsonBytes)); sErr != nil {
			common.SysError("failed to persist official price book: " + sErr.Error())
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "", "data": book.Meta()})
}

// GetOfficialPriceBook returns the book metadata (never the full entries).
func GetOfficialPriceBook(c *gin.Context) {
	book := loadOfficialBook()
	if book == nil {
		c.JSON(http.StatusOK, gin.H{"success": true, "data": official_pricing.Meta{Source: official_pricing.SourceModelsDev}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": book.Meta()})
}

type officialFillScope struct {
	Kind      string   `json:"kind"` // all_missing | channel | models
	ChannelID int      `json:"channel_id"`
	Models    []string `json:"models"`
}

type officialFillPreviewRequest struct {
	Scope officialFillScope `json:"scope"`
	Mode  string            `json:"mode"`
}

func splitModels(models string) []string {
	parts := strings.Split(models, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// resolveFillTargets turns a scope into the concrete list of model names to fill.
func resolveFillTargets(scope officialFillScope) ([]string, error) {
	switch scope.Kind {
	case "channel":
		ch, err := model.GetChannelById(scope.ChannelID, true)
		if err != nil {
			return nil, err
		}
		return splitModels(ch.Models), nil
	case "models":
		return scope.Models, nil
	default: // all_missing
		return model.GetEnabledModels(), nil
	}
}

// PreviewOfficialPriceFill computes the proposed price changes for a scope+mode.
func PreviewOfficialPriceFill(c *gin.Context) {
	var req officialFillPreviewRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求参数格式错误"})
		return
	}
	book := loadOfficialBook()
	if book == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请先刷新官方价手册"})
		return
	}
	targets, err := resolveFillTargets(req.Scope)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "解析目标模型失败：" + err.Error()})
		return
	}
	res := official_pricing.BuildPreview(book, targets, currentRatios(), req.Mode)
	c.JSON(http.StatusOK, gin.H{"success": true, "data": res})
}

type officialFillApplyRequest struct {
	Models []string `json:"models"`
}

// ApplyOfficialPriceFill writes official prices for the given models, merging
// into the existing ratio maps and persisting only the option keys that changed.
func ApplyOfficialPriceFill(c *gin.Context) {
	var req officialFillApplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "请求参数格式错误"})
		return
	}
	if len(req.Models) == 0 {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "未选择任何模型"})
		return
	}
	book := loadOfficialBook()
	if book == nil {
		c.JSON(http.StatusOK, gin.H{"success": false, "message": "请先刷新官方价手册"})
		return
	}

	next, changed, applied := official_pricing.ApplyToMaps(book, req.Models, currentRatios())

	mapByKey := map[string]map[string]float64{
		official_pricing.KeyModelRatio:       next.ModelRatio,
		official_pricing.KeyCompletionRatio:  next.CompletionRatio,
		official_pricing.KeyCacheRatio:       next.CacheRatio,
		official_pricing.KeyCreateCacheRatio: next.CreateCacheRatio,
	}
	changedKeys := make([]string, 0, len(changed))
	for key := range changed {
		jsonBytes, mErr := common.Marshal(mapByKey[key])
		if mErr != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "序列化失败：" + mErr.Error()})
			return
		}
		if uErr := model.UpdateOption(key, string(jsonBytes)); uErr != nil {
			c.JSON(http.StatusOK, gin.H{"success": false, "message": "保存失败：" + uErr.Error()})
			return
		}
		changedKeys = append(changedKeys, key)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"applied":       applied,
			"applied_count": len(applied),
			"changed_keys":  changedKeys,
		},
	})
}
