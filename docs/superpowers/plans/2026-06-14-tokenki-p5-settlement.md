# P5 结算系统 Implementation Plan

> 执行 subagent-driven。后端 TDD（model 内存 SQLite）。提交按单元。核心=手动结算流程；自动结算 cron 为 stretch。

**Goal:** 供应商发起结算（打包未结算消费日志成账单、可撤销）→ 超管审核确认实付（¥/$、方式、备注）→ 锁定账单；账单列表/详情/导出。

**数据基石（P4 已就位）:** `Log.OfficialUsd float64`、`Log.SettlementId int`(0=未结算)、`LogTypeConsume=2`；`model.GetChannelsBySupplier`、`model.GetSupplierPendingStat`；`Channel.SupplierId/CostPrice`；`Supplier{UserId,Priority,Enabled,SettlementMode,SettlementCycle}`。logs 在 `LOG_DB`(测试中=DB)。

---

## 单元 P5-BE-1：Settlement 模型 + 发起/撤销/确认/查询（TDD，核心）

### Files
- Create `model/settlement.go`
- Test `model/settlement_test.go`

### 模型
```go
type Settlement struct {
	Id             int     `json:"id"`
	SupplierId     int     `json:"supplier_id" gorm:"index"`
	Status         int     `json:"status" gorm:"index;default:1"` // 1待审核 2已结算 3已撤销
	PeriodStart    int64   `json:"period_start"`
	PeriodEnd      int64   `json:"period_end"`
	OfficialUsd    float64 `json:"official_usd"`     // 打包的官方价总额($)
	ComputedCNY    float64 `json:"computed_cny"`     // 应付人民币 Σ(渠道 officialUsd×cost_price)
	ActualAmount   float64 `json:"actual_amount"`    // 超管确认实付
	ActualCurrency string  `json:"actual_currency"`  // CNY|USD
	SettleMethod   string  `json:"settle_method"`
	Remark         string  `json:"remark"`
	Source         string  `json:"source" gorm:"default:'manual'"` // manual|auto
	LogCount       int64   `json:"log_count"`
	CreatedAt      int64   `json:"created_at" gorm:"autoCreateTime"`
	SettledAt      int64   `json:"settled_at"`
}
const (
	SettlementStatusApplied   = 1
	SettlementStatusSettled   = 2
	SettlementStatusCancelled = 3
)
```
加入迁移：`model/main.go` 的 `migrateDB` AutoMigrate 列表 + `migrateDBFast` migrations 切片各加 `&Settlement{}` / `{&Settlement{}, "Settlement"}`。

### 函数
```go
// CreateSettlement 事务打包供应商所有未结算消费日志成一张待审核账单。now 传入便于测试。
// 无可结算日志时返回 (nil, error)。
func CreateSettlement(supplierId int, source string, now int64) (*Settlement, error)

// CancelSettlement 撤销待审核账单（status=1→3），释放其日志(settlement_id 归0)。事务。
// 仅 status=1 可撤销；operatorIsAdmin=false 时校验 ownerSupplierId==supplierId。
func CancelSettlement(settlementId int, supplierId int, operatorIsAdmin bool) error

// ConfirmSettlement 超管确认结算（status=1→2），写实付金额/方式/备注/时间。
func ConfirmSettlement(settlementId int, actualAmount float64, currency, method, remark string, now int64) error

// GetSettlementsBySupplier 供应商自己的账单（分页，倒序）。
func GetSettlementsBySupplier(supplierId, startIdx, num int) ([]*Settlement, int64, error)
// ListSettlements 超管：按 status 过滤(0=全部)分页。
func ListSettlements(status, startIdx, num int) ([]*Settlement, int64, error)
// GetSettlementById 单账单。
func GetSettlementById(id int) (*Settlement, error)
// GetSettlementLogs 账单明细日志（settlement_id=id 的 logs，分页）。
func GetSettlementLogs(settlementId, startIdx, num int) ([]*Log, int64, error)
```

### CreateSettlement 实现要点（事务、原子打包）
1. `tx := DB.Begin()`。取供应商渠道 id→cost_price（`tx.Where("supplier_id=?", supplierId).Find(&channels)`）。无渠道→回滚返回 err。
2. 先建账单占位 `s := &Settlement{SupplierId, Status:Applied, Source, PeriodEnd:now}`；`tx.Create(s)` 得 `s.Id`。
3. 对每个渠道：`tx.Model(&Log{}).Where("type=? AND settlement_id=0 AND channel_id=?", LogTypeConsume, ch.Id).Update("settlement_id", s.Id)`（注意 logs 在 LOG_DB；若 LOG_DB≠DB，事务跨库不可行——见下"风险"。生产默认 LOG_DB 可能独立。本期假设同库或用 LOG_DB 事务）。
4. 打包后统计：`SELECT COALESCE(SUM(official_usd),0), COUNT(*), MIN(created_at) FROM logs WHERE settlement_id=s.Id`；computedCNY 需按渠道分别 Σ(渠道内 sum×cost_price)→对每个渠道 `SUM(official_usd) WHERE settlement_id=s.Id AND channel_id=ch.Id` × cost_price 累加。
5. 若 count==0 → 回滚、返回 err "无可结算消费"。
6. 回填 `s.OfficialUsd/ComputedCNY/LogCount/PeriodStart`，`tx.Save(s)`；commit。

> **LOG_DB 注意**：若 `LOG_DB` 与 `DB` 不同实例（独立日志库），跨库事务不可行。实现时：若用到 logs 的写，统一用 `LOG_DB`；Settlement 表建议放主 `DB`。打包 UPDATE 用 `LOG_DB`，账单建在 `DB`——则不能用单一事务覆盖两库。**本期策略**：先建账单(DB)，再 LOG_DB 打包 UPDATE，再统计回填；失败有补偿（撤销）。测试中 LOG_DB==DB 不受影响。报告注明该一致性边界。

### 测试（model/settlement_test.go）
建 channels(supplier 7, cost 2.5/2.0)+ logs(official 0.1/0.2 未结算)；
- `CreateSettlement(7,"manual",now)` → 账单 OfficialUsd=0.3, ComputedCNY=0.25+0.40, LogCount=2, Status=1；且 logs.settlement_id 被设为账单 id；再次 Create→err(无可结算)。
- `CancelSettlement(id,7,false)` → status=3，logs.settlement_id 归 0；非 owner 撤销→err。
- 重新 Create → `ConfirmSettlement(id, 0.6, "CNY","转账","ok",now)` → status=2，actual_amount=0.6，settled_at=now。
- `GetSettlementsBySupplier(7,0,20)`、`ListSettlements(1,0,20)`、`GetSettlementLogs(id,0,20)` 断言。

- [ ] Step 1 写测试→失败  - [ ] Step 2 实现模型+迁移+函数→通过  - [ ] Step 3 `go test ./model/ -run Settlement -v` + `-count=1` + vet  - [ ] Checkpoint

## 单元 P5-BE-2：接口 + 路由 + 导出
### Files: `controller/settlement.go`, `router/api-router.go`
- 供应商(SupplierAuth, /api/supplier/self/settlement):
  - `POST /` 发起结算（CreateSettlement(c.GetInt("id"),"manual",now)）
  - `GET /` 自己的账单列表
  - `POST /:id/cancel` 撤销自己的（CancelSettlement(id, uid, false)）
  - `GET /:id` 详情（校验归属）+ `GET /:id/logs` 明细 + `GET /:id/export` 导出 CSV
- 超管(RootAuth, /api/admin/settlement 或 /api/supplier/review):
  - `GET /` 列表（status 过滤）+ `GET /:id` 详情 + `GET /:id/logs`
  - `POST /:id/confirm`（actual_amount/currency/method/remark）+ `POST /:id/cancel`
  - `GET /:id/export` CSV
- 导出：CSV（免依赖；Content-Type text/csv; 文件名 settlement-<id>.csv）：账单头 + 每条日志(时间/模型/渠道/prompt/completion/official_usd)。`now` 用 `common.GetTimestamp()`。
- [ ] 实现 + vet/build + Checkpoint。

## 单元 P5-FE：账单结算页 + 超管审核页
### Files: `web/default/src/features/settlements/`(供应商) + `web/default/src/features/settlement-review/`(超管) + 路由 + 菜单 + i18n
- 供应商「账单结算」(/settlements, role>=SUPPLIER)：顶部待结算金额(调 /pending) + 「发起结算」按钮(确认弹窗→POST)；历史账单表(日期/周期/官方价$/应付¥/实付/状态)；行操作：详情、撤销(status=1)。详情抽屉/页含明细 + 导出按钮。
- 超管「结算审核」(/settlement-review, role>=SUPER_ADMIN, 菜单 minRole SUPER_ADMIN)：待审核+全部账单列表(供应商/金额/状态)；详情；「确认结算」弹窗(实付金额+币种+方式+备注)；撤销。
- 镜像 suppliers/my-channels 的 table/drawer 模式。typecheck+lint+build。

## 单元 P5-BE-3（stretch）：自动结算 cron
- 选项已有 Supplier.SettlementMode(manual/auto)/SettlementCycle(day/week/month)。
- 定时任务（仿 health check）：每小时检查；按 Asia/Shanghai，跨越日/周一/每月1号 0 点时，对 mode=auto 且 cycle 匹配的供应商执行 `CreateSettlement(sid,"auto",now)`（仅生成待审核）。需防重复（记录上次生成或按 period 去重）。**若时间不足，记为未完成，文档化。**

## 验收（P5 核心）
- 发起结算原子打包未结算日志成待审核账单（金额正确）；撤销释放日志；超管确认写实付并锁定。
- 供应商只见/撤销自己的；超管见全部、可确认。
- 账单详情含逐条日志；可导出 CSV。
- 后端 `go test ./model/` 全过。

## 风险/限制
- **LOG_DB 跨库**：若生产 logs 独立库，结算打包与账单非单一事务（补偿式）；测试同库不受影响——报告注明，建议生产同库或加补偿校验。
- 金额浮点累加误差；以超管确认的实付为准。
- 自动结算 cron 为 stretch，可能仅文档化。
- 导出先 CSV（满足"导出表格"），xlsx 后置。
