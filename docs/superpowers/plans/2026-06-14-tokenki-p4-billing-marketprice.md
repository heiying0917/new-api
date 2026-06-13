# P4 官方价记账 + 市场价 Implementation Plan

> 执行 subagent-driven。后端 TDD；提交按阶段。计费路径为准热路径，**纯叠加**（只多记一个数，不改下游扣费）。

**Goal:** 为每条供应商渠道的消费按**官方价**（不含分组折扣 groupRatio）记录美元金额 `OfficialUsd`，作为给供应商结算的数据基石；提供按供应商聚合的待结算统计与按分组的市场最低价。

**关键事实（勘探确认）:** `Log` 有 `Other` JSON + 字段 ChannelId/Quota/PromptTokens/CompletionTokens；`RecordConsumeLog(c, userId, RecordConsumeLogParams{...Other map})`（model/log.go:222）。`PostTextConsumeQuota`（service/text_quota.go:322）算出 `summary{ModelRatio,GroupRatio,CompletionRatio,ModelPrice,PromptTokens,CompletionTokens,Quota}` 与 `relayInfo.PriceData.UsePrice`，在 ~462 调 RecordConsumeLog。`model.CacheGetChannel(relayInfo.ChannelId)` → SupplierId/CostPrice。官方价：usePrice=false → `(prompt+completion×completionRatio)×modelRatio / QuotaPerUnit`；usePrice=true → `modelPrice`。`common.QuotaPerUnit=500000`。

---

## 单元 P4-BE-1：OfficialUsd 记账（Log 字段 + 纯函数 + 注入）

### Files
- Modify `model/log.go`（Log +`OfficialUsd float64`, +`SettlementId int`；RecordConsumeLogParams +`OfficialUsd`；写入 Log）
- Create `service/supplier_billing.go`（纯函数 `ComputeOfficialUsd`）
- Modify `service/text_quota.go`（PostTextConsumeQuota 调 RecordConsumeLog 前算 OfficialUsd）
- Test `service/supplier_billing_test.go`

- [ ] **Step 1 — Log 字段**：`model/log.go` Log 结构体加
```go
	OfficialUsd  float64 `json:"official_usd" gorm:"default:0;index"` // 官方价美元(不含分组折扣), 供应商结算用
	SettlementId int     `json:"settlement_id" gorm:"default:0;index"` // 0=未结算(P5 用)
```
Log 在 `LOG_DB.AutoMigrate(&Log{})`（model/main.go:372 附近），自动加列。RecordConsumeLogParams 加 `OfficialUsd float64`；RecordConsumeLog 构造 Log 时设 `OfficialUsd: params.OfficialUsd`。

- [ ] **Step 2（TDD）— 纯函数**：`service/supplier_billing_test.go`
```go
package service
import ("testing"; "github.com/QuantumNous/new-api/common"; "github.com/stretchr/testify/require")
func TestComputeOfficialUsd_Ratio(t *testing.T) {
	// prompt 1000, completion 1000, modelRatio 2(=$0.004/1k... 由 QuotaPerUnit 决定), completionRatio 3
	// officialQuota=(1000+1000*3)*2=8000; usd=8000/500000=0.016
	usd := ComputeOfficialUsd(1000, 1000, 2, 3, 0, false)
	require.InDelta(t, 8000.0/common.QuotaPerUnit, usd, 1e-9)
}
func TestComputeOfficialUsd_Price(t *testing.T) {
	require.InDelta(t, 0.04, ComputeOfficialUsd(0, 0, 0, 0, 0.04, true), 1e-9)
}
func TestComputeOfficialUsd_NoGroupDiscount(t *testing.T) {
	// 与 groupRatio 无关：函数签名里就没有 groupRatio
	require.Greater(t, ComputeOfficialUsd(100, 100, 1, 1, 0, false), 0.0)
}
```
- [ ] **Step 3 — 实现** `service/supplier_billing.go`：
```go
package service
import "github.com/QuantumNous/new-api/common"
// ComputeOfficialUsd 计算一次消费的官方价美元（不含分组折扣 groupRatio）。
func ComputeOfficialUsd(promptTokens, completionTokens int, modelRatio, completionRatio, modelPrice float64, usePrice bool) float64 {
	if usePrice {
		return modelPrice
	}
	officialQuota := (float64(promptTokens) + float64(completionTokens)*completionRatio) * modelRatio
	if officialQuota < 0 {
		officialQuota = 0
	}
	return officialQuota / common.QuotaPerUnit
}
```
测试通过。

- [ ] **Step 4 — 注入 PostTextConsumeQuota**：读 `service/text_quota.go` 找到 summary 计算完、RecordConsumeLog 调用前的位置（~462）。在那里加：
```go
	// 供应商渠道：按官方价记账（不含分组折扣）
	var officialUsd float64
	if relayInfo.ChannelId > 0 {
		if ch, err := model.CacheGetChannel(relayInfo.ChannelId); err == nil && ch.SupplierId > 0 {
			officialUsd = ComputeOfficialUsd(summary.PromptTokens, summary.CompletionTokens,
				summary.ModelRatio, summary.CompletionRatio, summary.ModelPrice, relayInfo.PriceData.UsePrice)
		}
	}
```
并把 `OfficialUsd: officialUsd` 加入构造的 `RecordConsumeLogParams`（找到 params 构造处）。核对 summary 字段真实名（ModelRatio/CompletionRatio/ModelPrice/PromptTokens/CompletionTokens）与 relayInfo.PriceData.UsePrice 真实路径，按实修正。
> 仅覆盖文本路径（主消费）；音频/wss 路径 OfficialUsd=0（v1 限制，报告注明）。

- [ ] **Step 5 — 校验**：`go test ./service/ -run OfficialUsd -v`；`go vet ./service/ ./model/`；`go build ./service/ ./model/ ./controller/`；`go test ./model/ ./service/ -count=1`。
- [ ] **Step 6 — Checkpoint**。

## 单元 P4-BE-2：聚合 + 市场价 + 接口

### Files
- Modify `model/log.go` / `model/supplier.go`（聚合查询）
- Modify `model/channel.go`（市场价查询）
- Create/Modify `controller/supplier_channel.go` 或新 `controller/supplier_market.go`（接口）
- Modify `router/api-router.go`
- Test `model/*_test.go`

- [ ] **Step 1（TDD）— 待结算聚合**：`model` 加
```go
// SupplierPendingStat 供应商待结算统计
type SupplierPendingStat struct {
	OfficialUsd  float64 `json:"official_usd"`  // 未结算官方价总额($)
	PayableCNY   float64 `json:"payable_cny"`   // 应付人民币 = Σ(各渠道 officialUsd × cost_price)
	LogCount     int64   `json:"log_count"`
}
// GetSupplierPendingStat 汇总某供应商所有渠道未结算(settlement_id=0)消费的官方价与应付金额。
func GetSupplierPendingStat(supplierId int) (SupplierPendingStat, error)
```
实现：先取该供应商渠道（id→cost_price 映射，GetChannelsBySupplier 或直接查 channels），再对每个渠道 `SELECT SUM(official_usd), COUNT(*) FROM logs WHERE type=2 AND settlement_id=0 AND channel_id=?`，累加 officialUsd，应付 += sum×cost_price。测试：造 channels+logs，断言。
（注意 logs 在 LOG_DB；测试 TestMain 里 LOG_DB=DB，同库，可用。）

- [ ] **Step 2（TDD）— 市场价**：`model/channel.go` 加
```go
// GetGroupMarketPrices 返回每个分组当前启用渠道的最低 cost_price（>0）。
func GetGroupMarketPrices() (map[string]float64, error)
```
实现：查 status=enabled 且 cost_price>0 的渠道，Go 层 split group(逗号)，取每组 min。测试断言。

- [ ] **Step 3 — 接口**：
  - `GET /api/supplier/pending`（SupplierAuth）→ 当前供应商 `GetSupplierPendingStat(c.GetInt("id"))`。
  - `GET /api/supplier/market-price`（SupplierAuth）→ `GetGroupMarketPrices()`（供应商+管理员可见）。
  挂在 `/api/supplier` 作用域（注意与 RootAuth 的 /supplier 管理组区分；这两个是 SupplierAuth，可放 supplierSelfRoute 同组或新建 `supplierSelfRoute2 := apiRouter.Group("/supplier")` + SupplierAuth——但 /supplier 已被 RootAuth 占用前缀？Gin 允许同前缀不同 path 不同中间件分组吗？**为避免冲突，把这两个放到 `/api/supplier/channel` 同级的新路径**，如 `/api/supplier/self/pending`、`/api/supplier/self/market-price`，用 SupplierAuth 组。实现时确认无路由冲突。）

- [ ] **Step 4 — 校验 + Checkpoint**：vet/build/test。

## 单元 P4-FE：市场价卡片 + 待结算金额

### Files
- 控制台首页/概览加"市场价"卡片（仅 role>=SUPPLIER 可见）：调 market-price，按分组列出最低价。
- "我的渠道"页或概览显示当前供应商待结算金额（调 pending）。
- i18n。

- [ ] 读现有 dashboard/overview 首页组件，加一个卡片（role>=ROLE.SUPPLIER 才渲染）。typecheck+lint+build。Checkpoint。

## 验收（P4）
- 供应商渠道每次文本消费记录正确 OfficialUsd（=官方价，不受分组折扣影响）；非供应商渠道为 0。
- `GET .../pending` 返回供应商未结算官方价总额与应付人民币（Σ 渠道 officialUsd×cost_price）。
- `GET .../market-price` 返回各分组最低成本价。
- 首页市场价卡片仅供应商+管理员可见。
- 后端 `go test ./model/ ./service/` 全过。

## 限制/风险
- 仅文本 post-consume 记 OfficialUsd；音频/实时(wss)路径暂为 0（报告注明，后续补 PostAudio/PostWss）。
- official_usd 用 float64：作为$金额累加，量大时有浮点误差；结算金额最终以管理员确认的"实际金额"为准（P5），可接受。
- SettlementId 本期加列默认 0，pending 查询按 settlement_id=0 过滤，为 P5 预留。
