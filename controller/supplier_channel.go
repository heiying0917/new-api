package controller

import (
	"bytes"
	"io"
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// validateSupplierChannelBaseURL 对供应商提交的 base_url 强制 SSRF 校验，
// 阻断指向私网 / 环回 / 链路本地 / 云元数据(169.254.169.254) 等内部地址的渠道。
// 供应商为半可信用户，这里强制开启保护、禁止私网，不依赖全局 fetch 设置。
// 空 base_url 表示使用提供商默认公网端点，放行。
func validateSupplierChannelBaseURL(baseURL *string) error {
	if baseURL == nil {
		return nil
	}
	u := strings.TrimSpace(*baseURL)
	if u == "" {
		return nil
	}
	// applyIPFilterForDomain=true：对域名 base_url 也解析 DNS 并逐个解析 IP 校验私网，
	// 否则 http://metadata.attacker.com（A 记录指向 169.254.169.254）这类域名可绕过校验。
	return common.ValidateURLWithFetchSetting(u, true, false, false, false, nil, nil, nil, true)
}

// SupplierListChannels 列出当前供应商自己的渠道。复用管理员列表核心(强制 supplier_id=本人),
// 因此分组/模型/类型/状态筛选、排序、标签模式、类型计数与管理员完全一致;并回填成本/应收款。
func SupplierListChannels(c *gin.Context) {
	listChannelsCore(c, c.GetInt("id"))
}

// SupplierSearchChannels 搜索当前供应商自己的渠道(复用管理员搜索核心,强制 supplier_id=本人)。
func SupplierSearchChannels(c *gin.Context) {
	searchChannelsCore(c, c.GetInt("id"))
}

// backfillSupplierUnsettled 为渠道列表回填未结算 official_usd(USD)与 receivable(应收款¥),
// 应收款按「每条日志冻结的成交价」累加,与结算口径一致、免疫事后改价。
func backfillSupplierUnsettled(channels []*model.Channel) {
	if len(channels) == 0 {
		return
	}
	ids := make([]int, 0, len(channels))
	for _, ch := range channels {
		ids = append(ids, ch.Id)
	}
	usdByChannel, _ := model.GetUnsettledOfficialUsdByChannels(ids)
	receivableByChannel, _ := model.GetUnsettledReceivableByChannels(ids)
	for _, ch := range channels {
		ch.OfficialUsd = usdByChannel[ch.Id]
		ch.Receivable = receivableByChannel[ch.Id]
	}
}

// SupplierGetChannel 取自己的单个渠道（不含明文 key；明文 key 走 /:id/key 受 2FA 守卫的接口）
func SupplierGetChannel(c *gin.Context) {
	supplierId := c.GetInt("id")
	id, _ := strconv.Atoi(c.Param("id"))
	// selectAll=false → Omit("key")，详情不下发明文 key，避免未经 2FA 即可读取
	ch, err := model.GetChannelById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch.SupplierId != supplierId {
		common.ApiErrorMsg(c, "forbidden: not your channel")
		return
	}
	common.ApiSuccess(c, ch)
}

// SupplierGetChannelKey 取自己渠道的明文 key。路由已挂 RequireTwoFAEnabled：
// 只有已开启 2FA/Passkey 的供应商才能调用；此处再校验渠道归属本人。
func SupplierGetChannelKey(c *gin.Context) {
	supplierId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	ch, err := model.GetChannelById(id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch.SupplierId != supplierId {
		common.ApiErrorMsg(c, "forbidden: not your channel")
		return
	}
	common.ApiSuccess(c, gin.H{"key": ch.Key})
}

// SupplierAddChannel 创建渠道（强制归属本人 + 成本价必填 > 0）
func SupplierAddChannel(c *gin.Context) {
	supplierId := c.GetInt("id")
	var ch model.Channel
	if err := c.ShouldBindJSON(&ch); err != nil {
		common.ApiError(c, err)
		return
	}
	if ch.CostPrice == nil || *ch.CostPrice <= 0 {
		common.ApiErrorMsg(c, "cost_price is required and must be > 0")
		return
	}
	if ch.Name == "" || ch.Key == "" || ch.Models == "" {
		common.ApiErrorMsg(c, "name, key and models are required")
		return
	}
	if err := validateSupplierChannelBaseURL(ch.BaseURL); err != nil {
		common.ApiErrorMsg(c, "base_url 不被允许（疑似指向内部地址）："+err.Error())
		return
	}
	ch.Id = 0
	ch.SupplierId = supplierId
	ch.CreatedBy = supplierId // 供应商建的渠道，创建者即本人
	ch.Status = common.ChannelStatusEnabled
	if ch.Group == "" {
		ch.Group = "default"
	}
	if err := ch.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"id": ch.Id})
}

// SupplierUpdateChannel 更新自己的渠道（保持归属不变）
func SupplierUpdateChannel(c *gin.Context) {
	supplierId := c.GetInt("id")
	var patch model.Channel
	if err := c.ShouldBindJSON(&patch); err != nil {
		common.ApiError(c, err)
		return
	}
	existing, err := model.GetChannelById(patch.Id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if existing.SupplierId != supplierId {
		common.ApiErrorMsg(c, "forbidden: not your channel")
		return
	}
	if patch.CostPrice != nil && *patch.CostPrice <= 0 {
		common.ApiErrorMsg(c, "cost_price must be > 0")
		return
	}
	if err := validateSupplierChannelBaseURL(patch.BaseURL); err != nil {
		common.ApiErrorMsg(c, "base_url 不被允许（疑似指向内部地址）："+err.Error())
		return
	}
	patch.SupplierId = supplierId
	patch.CreatedBy = existing.CreatedBy // 创建者不可篡改
	if err := patch.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	// 返回更新后的渠道(Update 内已回填全字段),供前端 manageChannel 就地刷新行状态;不回传 key。
	patch.Key = ""
	common.ApiSuccess(c, patch)
}

// SupplierDeleteChannel 删除自己的渠道
func SupplierDeleteChannel(c *gin.Context) {
	supplierId := c.GetInt("id")
	id, _ := strconv.Atoi(c.Param("id"))
	existing, err := model.GetChannelById(id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if existing.SupplierId != supplierId {
		common.ApiErrorMsg(c, "forbidden: not your channel")
		return
	}
	if err := existing.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, nil)
}

// supplierOwnsChannelParam 校验 URL :id 渠道归当前供应商所有;不属于则写错误并返回 false。
func supplierOwnsChannelParam(c *gin.Context) bool {
	supplierId := c.GetInt("id")
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	ch, err := model.GetChannelById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if ch.SupplierId != supplierId {
		common.ApiErrorMsg(c, "forbidden: not your channel")
		return false
	}
	return true
}

// supplierOwnsChannelBody 从 JSON body 的 id / channel_id 字段取渠道并校验归属;
// 读后将 body 复位,以便后续(被委托的管理员 handler)重新解析。
func supplierOwnsChannelBody(c *gin.Context) bool {
	supplierId := c.GetInt("id")
	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(raw))
	var probe struct {
		Id        int `json:"id"`
		ChannelId int `json:"channel_id"`
	}
	if err := common.Unmarshal(raw, &probe); err != nil {
		common.ApiError(c, err)
		return false
	}
	id := probe.Id
	if id == 0 {
		id = probe.ChannelId
	}
	ch, err := model.GetChannelById(id, false)
	if err != nil {
		common.ApiError(c, err)
		return false
	}
	if ch.SupplierId != supplierId {
		common.ApiErrorMsg(c, "forbidden: not your channel")
		return false
	}
	return true
}

// 以下供应商作用域操作:先校验「本人渠道」,再委托给现成的管理员 handler 完整复用其解析/逻辑/响应。

// SupplierTestChannel 测试本人单个渠道 GET /api/supplier/channel/test/:id
func SupplierTestChannel(c *gin.Context) {
	if !supplierOwnsChannelParam(c) {
		return
	}
	TestChannel(c)
}

// SupplierUpdateChannelBalance 刷新本人单个渠道余额 GET /api/supplier/channel/update_balance/:id
func SupplierUpdateChannelBalance(c *gin.Context) {
	if !supplierOwnsChannelParam(c) {
		return
	}
	UpdateChannelBalance(c)
}

// SupplierCopyChannel 复制本人渠道 POST /api/supplier/channel/copy/:id
// clone 为 origin 的浅拷贝,supplier_id/created_by 随之保持为本人(已通过归属校验)。
func SupplierCopyChannel(c *gin.Context) {
	if !supplierOwnsChannelParam(c) {
		return
	}
	CopyChannel(c)
}

// SupplierManageMultiKeys 管理本人渠道的多 Key POST /api/supplier/channel/multi_key/manage
func SupplierManageMultiKeys(c *gin.Context) {
	if !supplierOwnsChannelBody(c) {
		return
	}
	ManageMultiKeys(c)
}

// SupplierDetectChannelUpstreamModelUpdates 检测本人渠道的上游模型更新 POST /api/supplier/channel/upstream_updates/detect
func SupplierDetectChannelUpstreamModelUpdates(c *gin.Context) {
	if !supplierOwnsChannelBody(c) {
		return
	}
	DetectChannelUpstreamModelUpdates(c)
}

// SupplierApplyChannelUpstreamModelUpdates 应用本人渠道的上游模型更新 POST /api/supplier/channel/upstream_updates/apply
func SupplierApplyChannelUpstreamModelUpdates(c *gin.Context) {
	if !supplierOwnsChannelBody(c) {
		return
	}
	ApplyChannelUpstreamModelUpdates(c)
}
