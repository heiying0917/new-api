package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// 渠道 22（供应商 777，成本价 2.0）有 1 条未结算 + 1 条已结算日志；渠道 23 非供应商。
// 已消耗 = Σ official_usd（全部日志）= 3.0；应收款 = Σ official_usd × cost_price_snapshot = 1.0×2.0 + 2.0×2.2 = 6.4。
func seedConsumptionFixture(t *testing.T) {
	t.Helper()
	setupInitiateSettlementDB(t)
	model.InitColNames() // SearchChannels 拼接 commonKeyCol 等原生列名
	common.MemoryCacheEnabled = false
	cp := 2.0
	require.NoError(t, model.DB.Create(&model.User{Id: 777, Username: "davse", Role: common.RoleSupplierUser, Status: common.UserStatusEnabled, AffCode: "aff777", Group: "default"}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 22, Name: "op5-1", Key: "k", Type: 14, SupplierId: 777, CreatedBy: 777, CostPrice: &cp, Status: 1, Models: "m", Group: "g", UsedQuota: 12345678}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{Id: 23, Name: "plain", Key: "k", Type: 1, SupplierId: 0, Status: 1, Models: "m", Group: "g", UsedQuota: 999}).Error)
	require.NoError(t, model.LOG_DB.Create([]*model.Log{
		{Id: 1, UserId: 1, Type: model.LogTypeConsume, ChannelId: 22, SettlementId: 0, Quota: 500000, OfficialUsd: 1.0, CostPriceSnapshot: 2.0, ModelName: "m"},
		{Id: 2, UserId: 1, Type: model.LogTypeConsume, ChannelId: 22, SettlementId: 5, Quota: 1000000, OfficialUsd: 2.0, CostPriceSnapshot: 2.2, ModelName: "m"},
		{Id: 3, UserId: 1, Type: model.LogTypeConsume, ChannelId: 23, SettlementId: 0, Quota: 700000, OfficialUsd: 0, CostPriceSnapshot: 0, ModelName: "m"},
	}).Error)
}

type channelListResp struct {
	Success bool `json:"success"`
	Data    struct {
		Items []map[string]any `json:"items"`
		Total int64            `json:"total"`
	} `json:"data"`
}

func listItemsById(t *testing.T, w *httptest.ResponseRecorder) map[int]map[string]any {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, "resp=%s", w.Body.String())
	var resp channelListResp
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp), "resp=%s", w.Body.String())
	require.True(t, resp.Success, "resp=%s", w.Body.String())
	out := map[int]map[string]any{}
	for _, it := range resp.Data.Items {
		out[int(it["id"].(float64))] = it
	}
	return out
}

func newGetContext(userId int, path string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, path, nil)
	c.Set("id", userId)
	return c, w
}

// 管理员渠道列表：供应商渠道回填累计已消耗（official_usd）与应收款（receivable），保留含倍率的 used_quota；非供应商渠道为 0。
func TestAdminChannelList_AttachesCumulativeConsumptionForSupplierChannels(t *testing.T) {
	seedConsumptionFixture(t)

	c, w := newGetContext(1, "/api/channel/?p=1&page_size=20")
	GetAllChannels(c)
	items := listItemsById(t, w)

	require.Contains(t, items, 22)
	require.InDelta(t, 3.0, items[22]["official_usd"], 1e-9)
	require.InDelta(t, 6.4, items[22]["receivable"], 1e-9)
	require.EqualValues(t, 12345678, items[22]["used_quota"])
	require.Contains(t, items, 23)
	require.InDelta(t, 0, items[23]["official_usd"], 1e-9)
	require.InDelta(t, 0, items[23]["receivable"], 1e-9)
	require.EqualValues(t, 999, items[23]["used_quota"])
}

// 供应商渠道列表：只见自己的渠道，含倍率的 used_quota 一律抹为 0，已消耗/应收款为累计口径。
func TestSupplierChannelList_HidesUsedQuotaAndAttachesConsumption(t *testing.T) {
	seedConsumptionFixture(t)

	c, w := newGetContext(777, "/api/supplier/channel/?p=1&page_size=20")
	SupplierListChannels(c)
	items := listItemsById(t, w)

	require.Len(t, items, 1)
	require.Contains(t, items, 22)
	require.EqualValues(t, 0, items[22]["used_quota"])
	require.InDelta(t, 3.0, items[22]["official_usd"], 1e-9)
	require.InDelta(t, 6.4, items[22]["receivable"], 1e-9)
}

// 供应商搜索渠道：同样抹 used_quota 并回填已消耗/应收款。
func TestSupplierChannelSearch_HidesUsedQuotaAndAttachesConsumption(t *testing.T) {
	seedConsumptionFixture(t)

	c, w := newGetContext(777, "/api/supplier/channel/search?keyword=op5&p=1&page_size=20")
	SupplierSearchChannels(c)
	items := listItemsById(t, w)

	require.Contains(t, items, 22)
	require.EqualValues(t, 0, items[22]["used_quota"])
	require.InDelta(t, 3.0, items[22]["official_usd"], 1e-9)
	require.InDelta(t, 6.4, items[22]["receivable"], 1e-9)
}

// 供应商渠道详情：不下发含倍率的 used_quota。
func TestSupplierGetChannel_HidesUsedQuota(t *testing.T) {
	seedConsumptionFixture(t)

	c, w := newGetContext(777, "/api/supplier/channel/22")
	c.Params = gin.Params{{Key: "id", Value: "22"}}
	SupplierGetChannel(c)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Success bool           `json:"success"`
		Data    map[string]any `json:"data"`
	}
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp), "resp=%s", w.Body.String())
	require.True(t, resp.Success, "resp=%s", w.Body.String())
	require.EqualValues(t, 22, resp.Data["id"])
	require.EqualValues(t, 0, resp.Data["used_quota"])
}

type pricingResp struct {
	Success     bool               `json:"success"`
	Data        []any              `json:"data"`
	GroupRatio  map[string]float64 `json:"group_ratio"`
	UsableGroup map[string]string  `json:"usable_group"`
}

// 模型广场：供应商不得看到任何售价/分组倍率，返回空定价、空倍率、空可用分组；普通用户不受影响。
func TestGetPricing_SupplierSeesNoPricesOrRatios(t *testing.T) {
	seedConsumptionFixture(t)
	require.NoError(t, model.DB.Create(&model.User{Id: 5, Username: "normal", Role: common.RoleCommonUser, Status: common.UserStatusEnabled, AffCode: "aff5", Group: "default"}).Error)

	c, w := newGetContext(777, "/api/pricing")
	GetPricing(c)
	var sup pricingResp
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &sup), "resp=%s", w.Body.String())
	require.True(t, sup.Success)
	require.Empty(t, sup.Data, "供应商不得看到模型定价")
	require.Empty(t, sup.GroupRatio, "供应商不得看到分组倍率: %v", sup.GroupRatio)
	require.Empty(t, sup.UsableGroup)

	c2, w2 := newGetContext(5, "/api/pricing")
	GetPricing(c2)
	var usr pricingResp
	require.NoError(t, common.Unmarshal(w2.Body.Bytes(), &usr), "resp=%s", w2.Body.String())
	require.True(t, usr.Success)
	require.NotEmpty(t, usr.GroupRatio, fmt.Sprintf("普通用户应看到可用分组倍率: %s", w2.Body.String()))
}
