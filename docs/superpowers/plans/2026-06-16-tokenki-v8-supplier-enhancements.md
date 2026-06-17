# TokenKi 第八版 · 供应商体系增强 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 在供应商管理页加排序+立即结算+顶部汇总、渠道页按供应商名搜索、新建管理员可见的「供应商概览」紧凑网格页。

**Architecture:** 纯叠加。后端新增只读聚合查询(`model/supplier_stats.go`)、一个复用 `CreateSettlement` 的 admin initiate 接口、给 supplier/channel 列表接口加参数;前端在 classic 主题(`web/classic`)增改 3 块页面,复用已存在的 `ConfirmModal`。不改计费/调度/结算核心口径,不新增表/列。

**Tech Stack:** Go 1.22 + Gin + GORM v2(SQLite/MySQL/PG 三库兼容);React 18 + Vite + Semi Design + i18next;Bun。

**Spec:** `docs/superpowers/specs/2026-06-16-tokenki-v8-supplier-enhancements-design.md`

---

## ⚠️ 提交纪律(全局铁律)

本项目**未经用户明确指令不得 `git commit` / `push` / 部署**。计划中每个 Task 末尾的「提交点」是**建议提交边界**:执行到该点请**停下汇报**,等用户说「提交」再执行对应 `git commit`。在获得指令前,只完成「写代码 + 跑测试 + 通过」,不要自动提交。每完成一个 Task 追加 `docs/superpowers/WORKLOG.md`(何时/做了什么/改了哪些文件/如何验证)。

## 关键约定

- JSON 一律走 `common.Marshal/Unmarshal`(本计划接口返回用 `common.ApiSuccess`,无需手动 marshal)。
- 三库兼容:只用 GORM `Select/Where/Group/Order/Pluck`,金额用 `COALESCE(SUM(...),0)`,`LIKE` 通用,不写方言函数。
- 渠道「可用」判定 = `status = common.ChannelStatusEnabled`(=1)。
- 供应商角色值 = `model.RoleSupplierUser`(=5)。
- 结算状态:`model.SettlementStatusApplied`(1)/`SettlementStatusSettled`(2)/`SettlementStatusCancelled`(3)。
- 日志聚合用 `LOG_DB`,渠道/结算/用户用 `DB`(参照 `model/settlement_query.go:13`)。
- 后端测试命令统一:`go test ./model/... ./controller/...`(沿用现有 SQLite 测试装置)。前端:`cd web/classic && bun run build`。

## 文件结构(创建/修改)

**后端**
- 修改 `model/supplier_stats.go` — 新增 `GetAllSuppliersPendingStat`、`GetAllSuppliersSettledTotal`、`GetSettlementTotalsByStatus`、`GetSupplierOverview`(+ 类型)。
- 修改 `model/supplier.go` — `GetAllSuppliers` 支持 `sortBy/sortOrder`(计算列全量排序后分页)。
- 修改 `controller/supplier.go` — `GetAllSuppliers`/`SearchSuppliers` 读排序参数;新增 `GetSupplierSummary`、`GetSupplierOverviewAdmin`。
- 修改 `controller/settlement.go` — 新增 `AdminInitiateSettlement`。
- 修改 `controller/channel.go` + `model/channel.go` — `GetAllChannels`/`SearchChannels` 支持 `supplier_name` 过滤。
- 修改 `router/api-router.go` — 注册 `/supplier/summary`、`/admin/settlement/initiate`、`/admin/supplier-overview`。
- 新增测试:`model/supplier_summary_test.go`、`model/supplier_overview_test.go`、`model/supplier_sort_test.go`、`controller/settlement_initiate_test.go`、`model/channel_supplier_filter_test.go`。

**前端(web/classic)**
- 修改 `src/hooks/suppliers/useSuppliersData.jsx` — summary、排序、立即结算+确认弹窗 state。
- 修改 `src/components/table/suppliers/SuppliersColumnDefs.jsx` — 列排序 + 「立即结算」按钮。
- 修改 `src/components/table/suppliers/index.jsx` — 顶部汇总条 + 挂 `ConfirmModal`。
- 新增 `src/components/table/suppliers/SuppliersSummaryBar.jsx`。
- 修改 `src/components/table/suppliers/SuppliersTable.jsx` — 透传 `onChange`(排序)。
- 修改 `src/components/table/channels/ChannelsFilters.jsx` + `src/hooks/channels/useChannelsData.jsx` — 供应商搜索框。
- 新增 `src/pages/SupplierOverviewAdmin/index.jsx` + `src/hooks/supplier-overview-admin/useSupplierOverviewData.jsx` + `src/components/supplier-overview-admin/{TypeCard.jsx,TypeDetailSheet.jsx}`。
- 修改 `src/App.jsx`(路由)、`src/components/layout/SiderBar.jsx`(菜单)、`src/i18n/locales/zh-CN.json`(文案)。

---

# Phase A — 后端聚合:汇总条数据(需求1)

### Task 1: per-supplier + 全局 待结算聚合

**Files:**
- Modify: `model/supplier_stats.go`
- Test: `model/supplier_summary_test.go`

- [ ] **Step 1: 写失败测试**

```go
package model

import "testing"

func TestGetAllSuppliersPendingStat(t *testing.T) {
	setupTestDB(t) // 复用现有测试装置(见其它 *_test.go 中的初始化方式)
	// 供应商 901 两条渠道、902 一条渠道
	seedSupplierChannel(t, 901, 7001, 2.0) // supplierId, channelId, costPrice(仅占位)
	seedSupplierChannel(t, 901, 7002, 2.0)
	seedSupplierChannel(t, 902, 7003, 3.0)
	// 未结算消费日志:official_usd, cost_price_snapshot
	seedConsumeLog(t, 7001, 10.0, 2.0, 0) // channelId, officialUsd, snapshot, settlementId
	seedConsumeLog(t, 7002, 5.0, 2.0, 0)
	seedConsumeLog(t, 7003, 4.0, 3.0, 0)
	seedConsumeLog(t, 7003, 1.0, 3.0, 88) // 已结算(settlement_id!=0)→不计

	perSupplier, global, err := GetAllSuppliersPendingStat()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got := perSupplier[901].OfficialUsd; got != 15.0 {
		t.Fatalf("901 official_usd = %v, want 15", got)
	}
	if got := perSupplier[901].PayableCNY; got != 30.0 { // 10*2 + 5*2
		t.Fatalf("901 payable = %v, want 30", got)
	}
	if got := perSupplier[902].PayableCNY; got != 12.0 { // 4*3
		t.Fatalf("902 payable = %v, want 12", got)
	}
	if global.OfficialUsd != 19.0 || global.PayableCNY != 42.0 {
		t.Fatalf("global = %+v, want usd 19 cny 42", global)
	}
}
```

> 注:`setupTestDB/seedSupplierChannel/seedConsumeLog` 若现有测试已有等价 helper 直接复用;没有则在本测试文件内用 `DB.Create(&Channel{...})` / `LOG_DB.Create(&Log{...})` 内联构造(参照 `model/settlement_query_test.go` 的构造方式)。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./model/ -run TestGetAllSuppliersPendingStat -v`
Expected: FAIL（`GetAllSuppliersPendingStat` 未定义）。

- [ ] **Step 3: 实现**

在 `model/supplier_stats.go` 追加:

```go
// GetAllSuppliersPendingStat 一次聚合所有供应商未结算消费,返回 per-supplier map 与全局合计。
// cross-DB safe, no JOIN:渠道→供应商映射(DB) + 日志按 channel_id 聚合(LOG_DB) → Go 折叠。
func GetAllSuppliersPendingStat() (map[int]SupplierPendingStat, SupplierPendingStat, error) {
	type chanRow struct {
		Id         int
		SupplierId int
	}
	var rows []chanRow
	if err := DB.Model(&Channel{}).Select("id, supplier_id").
		Where("supplier_id > 0").Scan(&rows).Error; err != nil {
		return nil, SupplierPendingStat{}, err
	}
	perSupplier := make(map[int]SupplierPendingStat)
	var global SupplierPendingStat
	if len(rows) == 0 {
		return perSupplier, global, nil
	}
	chanToSupplier := make(map[int]int, len(rows))
	channelIds := make([]int, 0, len(rows))
	for _, r := range rows {
		chanToSupplier[r.Id] = r.SupplierId
		channelIds = append(channelIds, r.Id)
	}
	type agg struct {
		ChannelId   int
		OfficialUsd float64
		PayableCNY  float64
		LogCount    int64
	}
	var aggs []agg
	if err := LOG_DB.Model(&Log{}).
		Select("channel_id AS channel_id, " +
			"COALESCE(SUM(official_usd),0) AS official_usd, " +
			"COALESCE(SUM(official_usd * cost_price_snapshot),0) AS payable_cny, " +
			"COUNT(*) AS log_count").
		Where("type = ? AND settlement_id = 0 AND channel_id IN ?", LogTypeConsume, channelIds).
		Group("channel_id").Scan(&aggs).Error; err != nil {
		return nil, SupplierPendingStat{}, err
	}
	for _, a := range aggs {
		sid := chanToSupplier[a.ChannelId]
		s := perSupplier[sid]
		s.OfficialUsd += a.OfficialUsd
		s.PayableCNY += a.PayableCNY
		s.LogCount += a.LogCount
		perSupplier[sid] = s
		global.OfficialUsd += a.OfficialUsd
		global.PayableCNY += a.PayableCNY
		global.LogCount += a.LogCount
	}
	return perSupplier, global, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./model/ -run TestGetAllSuppliersPendingStat -v`
Expected: PASS。

- [ ] **Step 5: 提交点(等用户指令)**

```bash
git add model/supplier_stats.go model/supplier_summary_test.go
git commit -m "feat(supplier): 全量待结算聚合 GetAllSuppliersPendingStat"
```

---

### Task 2: 结算单分桶合计(已申请/已结算)+ per-supplier 已结算

**Files:**
- Modify: `model/supplier_stats.go`
- Test: `model/supplier_summary_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestGetSettlementTotalsByStatus(t *testing.T) {
	setupTestDB(t)
	// 已申请(1):official 20, computed 40
	DB.Create(&Settlement{SupplierId: 901, Status: SettlementStatusApplied, OfficialUsd: 20, ComputedCNY: 40})
	// 已结算(2):official 10, computed 25, 实付 30 CNY
	DB.Create(&Settlement{SupplierId: 901, Status: SettlementStatusSettled, OfficialUsd: 10, ComputedCNY: 25, ActualAmount: 30, ActualCurrency: "CNY"})
	// 已结算(2):official 5, computed 12, 实付 6 USD
	DB.Create(&Settlement{SupplierId: 902, Status: SettlementStatusSettled, OfficialUsd: 5, ComputedCNY: 12, ActualAmount: 6, ActualCurrency: "USD"})

	applied, err := GetSettlementTotalsByStatus(SettlementStatusApplied)
	if err != nil {
		t.Fatal(err)
	}
	if applied.OfficialUsd != 20 || applied.ComputedCNY != 40 || applied.Count != 1 {
		t.Fatalf("applied=%+v", applied)
	}
	settled, _ := GetSettlementTotalsByStatus(SettlementStatusSettled)
	if settled.OfficialUsd != 15 || settled.ActualCNY != 30 || settled.ActualUSD != 6 || settled.Count != 2 {
		t.Fatalf("settled=%+v", settled)
	}
}

func TestGetAllSuppliersSettledTotal(t *testing.T) {
	setupTestDB(t)
	DB.Create(&Settlement{SupplierId: 901, Status: SettlementStatusSettled, ComputedCNY: 25})
	DB.Create(&Settlement{SupplierId: 901, Status: SettlementStatusSettled, ComputedCNY: 5})
	DB.Create(&Settlement{SupplierId: 902, Status: SettlementStatusApplied, ComputedCNY: 99}) // 非已结算→不计
	m, err := GetAllSuppliersSettledTotal()
	if err != nil {
		t.Fatal(err)
	}
	if m[901] != 30 {
		t.Fatalf("901=%v want 30", m[901])
	}
	if _, ok := m[902]; ok {
		t.Fatalf("902 should be absent")
	}
}
```

> 若 `Settlement` 字段名与此处不符(`ActualAmount/ActualCurrency/ComputedCNY/OfficialUsd`),以 `model/settlement.go:25` 实际定义为准并同步调整测试与实现。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./model/ -run 'TestGetSettlementTotalsByStatus|TestGetAllSuppliersSettledTotal' -v`
Expected: FAIL（函数未定义）。

- [ ] **Step 3: 实现**

在 `model/supplier_stats.go` 追加:

```go
// SettlementTotals 某状态结算单的合计。
type SettlementTotals struct {
	OfficialUsd float64 `json:"official_usd"`
	ComputedCNY float64 `json:"computed_cny"`
	ActualCNY   float64 `json:"actual_cny"`
	ActualUSD   float64 `json:"actual_usd"`
	Count       int64   `json:"count"`
}

// GetSettlementTotalsByStatus 汇总指定状态结算单;actual_* 仅对已结算有意义,按币种拆分。
func GetSettlementTotalsByStatus(status int) (SettlementTotals, error) {
	var t SettlementTotals
	var base struct {
		OfficialUsd float64
		ComputedCNY float64
		Count       int64
	}
	if err := DB.Model(&Settlement{}).
		Select("COALESCE(SUM(official_usd),0) AS official_usd, " +
			"COALESCE(SUM(computed_cny),0) AS computed_cny, " +
			"COUNT(*) AS count").
		Where("status = ?", status).Scan(&base).Error; err != nil {
		return t, err
	}
	t.OfficialUsd, t.ComputedCNY, t.Count = base.OfficialUsd, base.ComputedCNY, base.Count

	type curRow struct {
		ActualCurrency string
		Amount         float64
	}
	var cur []curRow
	if err := DB.Model(&Settlement{}).
		Select("actual_currency AS actual_currency, COALESCE(SUM(actual_amount),0) AS amount").
		Where("status = ?", status).Group("actual_currency").Scan(&cur).Error; err != nil {
		return t, err
	}
	for _, c := range cur {
		if c.ActualCurrency == "USD" {
			t.ActualUSD += c.Amount
		} else {
			t.ActualCNY += c.Amount // CNY 或空币种归入人民币
		}
	}
	return t, nil
}

// GetAllSuppliersSettledTotal per-supplier 已结算 computed_cny 合计。
func GetAllSuppliersSettledTotal() (map[int]float64, error) {
	type row struct {
		SupplierId int
		Total      float64
	}
	var rows []row
	if err := DB.Model(&Settlement{}).
		Select("supplier_id AS supplier_id, COALESCE(SUM(computed_cny),0) AS total").
		Where("status = ?", SettlementStatusSettled).
		Group("supplier_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	m := make(map[int]float64, len(rows))
	for _, r := range rows {
		m[r.SupplierId] = r.Total
	}
	return m, nil
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./model/ -run 'TestGetSettlementTotalsByStatus|TestGetAllSuppliersSettledTotal' -v`
Expected: PASS。

- [ ] **Step 5: 提交点(等用户指令)**

```bash
git add model/supplier_stats.go model/supplier_summary_test.go
git commit -m "feat(supplier): 结算分桶合计 + per-supplier 已结算"
```

---

### Task 3: 汇总接口 `GET /api/supplier/summary`

**Files:**
- Modify: `controller/supplier.go`, `router/api-router.go`

- [ ] **Step 1: 实现 controller**

在 `controller/supplier.go` 追加:

```go
// GetSupplierSummary 返回全局三组指标(待结算/已申请/已结算),供供应商管理页顶部汇总条。
func GetSupplierSummary(c *gin.Context) {
	_, pendingGlobal, err := model.GetAllSuppliersPendingStat()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	applied, err := model.GetSettlementTotalsByStatus(model.SettlementStatusApplied)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	settled, err := model.GetSettlementTotalsByStatus(model.SettlementStatusSettled)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 有未结算量的供应商数
	perSupplier, _, _ := model.GetAllSuppliersPendingStat()
	supplierCount := 0
	for _, s := range perSupplier {
		if s.LogCount > 0 {
			supplierCount++
		}
	}
	common.ApiSuccess(c, gin.H{
		"pending": gin.H{
			"official_usd":   pendingGlobal.OfficialUsd,
			"payable_cny":    pendingGlobal.PayableCNY,
			"supplier_count": supplierCount,
			"log_count":      pendingGlobal.LogCount,
		},
		"applied": gin.H{
			"official_usd": applied.OfficialUsd,
			"computed_cny": applied.ComputedCNY,
			"count":        applied.Count,
		},
		"settled": gin.H{
			"official_usd": settled.OfficialUsd,
			"actual_cny":   settled.ActualCNY,
			"actual_usd":   settled.ActualUSD,
			"computed_cny": settled.ComputedCNY,
			"count":        settled.Count,
		},
	})
}
```

> 优化:`GetAllSuppliersPendingStat` 被调两次,可改为调一次取 `perSupplier+global` 复用。这里为可读性分开;若在意性能合并即可。

- [ ] **Step 2: 注册路由**

`router/api-router.go` 在 `supplierAdminRoute` 组(`:153` 附近,RootAuth)内追加:

```go
supplierAdminRoute.GET("/summary", controller.GetSupplierSummary)
```

- [ ] **Step 3: 编译验证**

Run: `go build ./...`
Expected: 成功无错误。

- [ ] **Step 4: 提交点(等用户指令)**

```bash
git add controller/supplier.go router/api-router.go
git commit -m "feat(supplier): GET /api/supplier/summary 全局结算汇总"
```

---

# Phase B — 后端:供应商列表排序(需求1)

### Task 4: `GetAllSuppliers` 支持 sort_by/sort_order

**Files:**
- Modify: `model/supplier.go`, `controller/supplier.go`
- Test: `model/supplier_sort_test.go`

- [ ] **Step 1: 先读现状**

读 `model/supplier.go` 的 `GetAllSuppliers` 现签名与实现(约 `:150-200`,内部对每个返回项调 `GetSupplierPendingStat`/`GetSupplierSettledStats`)。确认其分页方式与 `SupplierListItem` 字段名(`PendingCNY`/`SettledCNY` 等),据此对齐下面代码。

- [ ] **Step 2: 写失败测试**

```go
func TestGetAllSuppliersSortByPending(t *testing.T) {
	setupTestDB(t)
	// 造三个供应商,pending 分别 30/12/0
	seedSupplierWithPending(t, 901, 30)
	seedSupplierWithPending(t, 902, 12)
	seedSupplierWithPending(t, 903, 0)

	items, _, err := GetAllSuppliers(0, 100, "pending_cny", "desc")
	if err != nil {
		t.Fatal(err)
	}
	if len(items) < 3 || items[0].UserId != 901 || items[1].UserId != 902 {
		t.Fatalf("desc order wrong: %+v", items)
	}
	itemsAsc, _, _ := GetAllSuppliers(0, 100, "pending_cny", "asc")
	if itemsAsc[0].UserId != 903 {
		t.Fatalf("asc order wrong: %+v", itemsAsc)
	}
}
```

> `seedSupplierWithPending` = 建 1 个 role=5 用户 + Supplier 记录 + 1 渠道 + 未结算日志,使 payable=指定值。复用 Task1 的内联构造。`GetAllSuppliers` 现签名若为 `(startIdx, pageSize int)`,本任务把它扩展为 `(startIdx, pageSize int, sortBy, sortOrder string)`,并更新所有调用点。

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./model/ -run TestGetAllSuppliersSortByPending -v`
Expected: FAIL（签名不符/未排序）。

- [ ] **Step 4: 实现**

把 `GetAllSuppliers` 改为接受 `sortBy, sortOrder string`。逻辑:
- `sortBy == ""` 或 `"id"`:维持原 `id desc` + DB 分页(原行为)。
- `sortBy ∈ {pending_cny, pending_usd, settled_cny}`(计算列):取**全量** role=5 列表 → 用 `GetAllSuppliersPendingStat()`/`GetAllSuppliersSettledTotal()` 填充每项的 `PendingCNY/PendingUsd/SettledCNY` → `sort.Slice` 按方向排序 → Go 内存 `startIdx/pageSize` 分页。
- `sortBy == "priority"`:DB `Order("priority " + dir)` + 分页。

实现骨架(按实读签名对齐字段名):

```go
func GetAllSuppliers(startIdx, pageSize int, sortBy, sortOrder string) ([]*SupplierListItem, int64, error) {
	dir := "desc"
	if sortOrder == "asc" {
		dir = "asc"
	}
	computed := sortBy == "pending_cny" || sortBy == "pending_usd" || sortBy == "settled_cny"

	if !computed {
		order := "id desc"
		if sortBy == "priority" {
			order = "priority " + dir
		}
		// ... 原有 DB 分页查询,把 ORDER BY 换成 order,然后逐项补 stats(原逻辑) ...
		return pagedItemsWithStats(startIdx, pageSize, order)
	}

	// 计算列:全量取出 → 补 stats → 排序 → 内存分页
	all, total, err := allSuppliersWithStats() // 取全部 role=5 + 填 pending/settled
	if err != nil {
		return nil, 0, err
	}
	less := func(i, j int) bool {
		var a, b float64
		switch sortBy {
		case "pending_usd":
			a, b = all[i].PendingUsd, all[j].PendingUsd
		case "settled_cny":
			a, b = all[i].SettledCNY, all[j].SettledCNY
		default: // pending_cny
			a, b = all[i].PendingCNY, all[j].PendingCNY
		}
		if dir == "asc" {
			return a < b
		}
		return a > b
	}
	sort.Slice(all, less)
	end := startIdx + pageSize
	if startIdx > len(all) {
		startIdx = len(all)
	}
	if end > len(all) {
		end = len(all)
	}
	return all[startIdx:end], total, nil
}
```

> `allSuppliersWithStats` / `pagedItemsWithStats`:把现有「补 stats」逻辑抽成 helper 复用。计算列时用 `GetAllSuppliersPendingStat()` 的 map 一次性填充,避免 N 次单查。若 `SupplierListItem` 尚无 `PendingUsd` 字段则补一个 `json:"pending_usd"`。

- [ ] **Step 5: 改 controller 读参数**

`controller/supplier.go` 的 `GetAllSuppliers`/`SearchSuppliers`:

```go
sortBy := c.Query("sort_by")
sortOrder := c.Query("sort_order")
// 传入 model.GetAllSuppliers(startIdx, pageSize, sortBy, sortOrder)
```

(`SearchSuppliers` 若走不同 model 函数,同样透传或暂只在非搜索列表支持排序——搜索态可不排序。)

- [ ] **Step 6: 跑测试 + 编译**

Run: `go test ./model/ -run TestGetAllSuppliersSortByPending -v && go build ./...`
Expected: PASS + 编译通过。

- [ ] **Step 7: 提交点(等用户指令)**

```bash
git add model/supplier.go controller/supplier.go model/supplier_sort_test.go
git commit -m "feat(supplier): 供应商列表服务端排序(优先级/待结算/已结算)"
```

---

# Phase C — 后端:管理员立即结算(需求1)

### Task 5: `POST /api/admin/settlement/initiate`

**Files:**
- Modify: `controller/settlement.go`, `router/api-router.go`
- Test: `controller/settlement_initiate_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestAdminInitiateSettlement(t *testing.T) {
	setupTestDB(t)
	seedSupplierChannel(t, 901, 7001, 2.0)
	seedConsumeLog(t, 7001, 10.0, 2.0, 0) // 未结算 official 10 → computed 20

	s, err := model.CreateSettlementForAdmin(901, 999) // supplierId, operatorAdminId
	if err != nil {
		t.Fatal(err)
	}
	if s.Status != model.SettlementStatusApplied || s.OfficialUsd != 10 || s.ComputedCNY != 20 {
		t.Fatalf("settlement=%+v", s)
	}
	// 台账写了 admin create
	// 再次发起:无未结算日志 → 报错
	if _, err := model.CreateSettlementForAdmin(901, 999); err == nil {
		t.Fatalf("expected error on empty pending")
	}
}
```

> 若决定不新增 model helper、直接在 controller 内复用 `model.CreateSettlement` + `RecordSettlementLedger`,则把本测试改为针对 controller 的 HTTP 测试(用现有 controller 测试装置发请求)。二选一,推荐下面的「controller 内复用」更省。

- [ ] **Step 2: 实现 controller(复用 `CreateSettlement`)**

`controller/settlement.go` 追加(对照 `SupplierCreateSettlement:17` 的写法,把 operator 改成 admin):

```go
type adminInitiateReq struct {
	SupplierId int `json:"supplier_id"`
}

// AdminInitiateSettlement 管理员为指定供应商立即发起结算单(status=已申请),返回新单供前端打开确认弹窗。
func AdminInitiateSettlement(c *gin.Context) {
	var req adminInitiateReq
	if err := c.ShouldBindJSON(&req); err != nil || req.SupplierId <= 0 {
		common.ApiErrorMsg(c, "invalid supplier_id")
		return
	}
	// 校验目标是供应商
	u, err := model.GetUserById(req.SupplierId, false)
	if err != nil || u == nil || u.Role != model.RoleSupplierUser {
		common.ApiErrorMsg(c, "目标不是供应商")
		return
	}
	s, err := model.CreateSettlement(req.SupplierId, "manual", common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err) // 含「无待结算消费」错误透传
		return
	}
	model.RecordSettlementLedger(&model.SettlementLedger{
		SettlementId: s.Id, SupplierId: s.SupplierId, Action: "create",
		OfficialUsd: s.OfficialUsd, ComputedCNY: s.ComputedCNY,
		OperatorId: c.GetInt("id"), OperatorIsAdmin: true,
		SnapshotHash: model.SettlementSnapshotHash(s),
	})
	common.ApiSuccess(c, s)
}
```

> 验证 `model.GetUserById` 签名与 `User.Role` 字段名(对照 `model/user.go`);若 helper 名不同则替换。`CreateSettlement` 对无未结算日志已应返回错误(P5 实现);若它返回的是空单而非错误,则在此判断 `s.LogCount == 0` → `common.ApiErrorMsg(c, "无待结算消费")` 并回滚(读 `model.CreateSettlement` 实现确认行为)。

- [ ] **Step 3: 注册路由**

`router/api-router.go` `settlementAdminRoute` 组(`:196`,RootAuth)内追加:

```go
settlementAdminRoute.POST("/initiate", controller.AdminInitiateSettlement)
```

- [ ] **Step 4: 测试 + 编译**

Run: `go test ./controller/ -run TestAdminInitiateSettlement -v && go build ./...`
Expected: PASS + 编译通过。

- [ ] **Step 5: 提交点(等用户指令)**

```bash
git add controller/settlement.go router/api-router.go controller/settlement_initiate_test.go
git commit -m "feat(settlement): 管理员立即发起结算 POST /api/admin/settlement/initiate"
```

---

# Phase D — 后端:渠道按供应商名搜索(需求2)

### Task 6: `supplier_name` 过滤

**Files:**
- Modify: `controller/channel.go`, `model/channel.go`
- Test: `model/channel_supplier_filter_test.go`

- [ ] **Step 1: 先读现状**

读 `model/channel.go` 的 `GetAllChannels` 与 `SearchChannels` 现签名;读 `controller/channel.go:125,294` 它们如何读 query。确定把过滤做在 model(加参数)还是 controller 预解析 user_ids 再传 `supplier_ids`。推荐:controller 解析 `supplier_name` → user_ids;model 新增可选 `supplierIds []int` 过滤参数。

- [ ] **Step 2: 写失败测试(model 层 helper)**

```go
func TestResolveSupplierIdsByName(t *testing.T) {
	setupTestDB(t)
	DB.Create(&User{Username: "alpha_supplier", Role: RoleSupplierUser})
	DB.Create(&User{Username: "beta_supplier", Email: "beta@x.com", Role: RoleSupplierUser})
	DB.Create(&User{Username: "normaluser", Role: 1})

	ids, err := ResolveSupplierIdsByName("supplier")
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("want 2 got %d (%v)", len(ids), ids)
	}
	none, _ := ResolveSupplierIdsByName("nomatch_xyz")
	if len(none) != 0 {
		t.Fatalf("want 0 got %v", none)
	}
}
```

- [ ] **Step 3: 跑测试确认失败**

Run: `go test ./model/ -run TestResolveSupplierIdsByName -v`
Expected: FAIL（未定义）。

- [ ] **Step 4: 实现 helper + 接入查询**

`model/channel.go`(或 `model/supplier.go`)追加:

```go
// ResolveSupplierIdsByName 模糊匹配供应商(role=5)的用户名/邮箱/手机号,返回 user_id 列表。
func ResolveSupplierIdsByName(keyword string) ([]int, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, nil
	}
	like := "%" + keyword + "%"
	var ids []int
	if err := DB.Model(&User{}).
		Where("role = ? AND (username LIKE ? OR email LIKE ? OR phone LIKE ?)",
			RoleSupplierUser, like, like, like).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}
```

接入:在 `GetAllChannels`/`SearchChannels` 增加可选 `supplierIds []int` 形参,非 nil 时 `.Where("supplier_id IN ?", supplierIds)`(空切片 → 立刻返回空结果,不发查询)。

`controller/channel.go` 两个 handler:

```go
var supplierIds []int
if sn := strings.TrimSpace(c.Query("supplier_name")); sn != "" {
	ids, err := model.ResolveSupplierIdsByName(sn)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if len(ids) == 0 {
		// 无匹配 → 返回空分页结果(构造 total=0 的空 items 直接 ApiSuccess)
		common.ApiSuccess(c, emptyChannelPage(c)) // 或复用现有空响应构造
		return
	}
	supplierIds = ids
}
// 传入 model.GetAllChannels(..., supplierIds) / model.SearchChannels(..., supplierIds)
```

> 确保 `strings` 已 import。`emptyChannelPage` 若无现成构造,直接 `common.ApiSuccess(c, gin.H{"items": []any{}, "total": 0, "type_counts": gin.H{}})`(对齐前端解构字段)。

- [ ] **Step 5: 测试 + 编译**

Run: `go test ./model/ -run TestResolveSupplierIdsByName -v && go build ./...`
Expected: PASS + 编译通过。

- [ ] **Step 6: 提交点(等用户指令)**

```bash
git add model/channel.go controller/channel.go model/channel_supplier_filter_test.go
git commit -m "feat(channel): 渠道列表按供应商名模糊过滤 supplier_name"
```

---

# Phase E — 后端:供应商概览聚合(需求3/4/5)

### Task 7: `GetSupplierOverview`

**Files:**
- Modify: `model/supplier_stats.go`
- Test: `model/supplier_overview_test.go`

- [ ] **Step 1: 写失败测试**

```go
func TestGetSupplierOverview(t *testing.T) {
	setupTestDB(t)
	// 类型14(Anthropic):供应商901两条(1可用1停)、供应商902一条可用,价 2.0/2.5
	seedChannelFull(t, 7001, 14, "claude-official", ChannelStatusEnabled, 901, 2.0)
	seedChannelFull(t, 7002, 14, "claude-official", 2 /*disabled*/, 901, 2.2)
	seedChannelFull(t, 7003, 14, "claude-official", ChannelStatusEnabled, 902, 2.5)
	// 类型1(OpenAI):供应商901一条可用 价1.8
	seedChannelFull(t, 7004, 1, "gpt-official", ChannelStatusEnabled, 901, 1.8)

	ov, err := GetSupplierOverview()
	if err != nil {
		t.Fatal(err)
	}
	if ov.Summary.ChannelTotal != 4 || ov.Summary.ChannelAvailable != 3 || ov.Summary.ChannelUnavailable != 1 {
		t.Fatalf("summary=%+v", ov.Summary)
	}
	var anth *SupplierTypeStat
	for i := range ov.ByType {
		if ov.ByType[i].Type == 14 {
			anth = &ov.ByType[i]
		}
	}
	if anth == nil || anth.SupplierCount != 2 || anth.ChannelCount != 3 || anth.Available != 2 || anth.LowestPrice != 2.0 {
		t.Fatalf("anthropic=%+v", anth)
	}
}
```

> `seedChannelFull(channelId, type, group, status, supplierId, costPrice)` 内联建 `Channel`。`ChannelStatusEnabled` 用 `common.ChannelStatusEnabled`(model 包内若有别名用别名)。

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./model/ -run TestGetSupplierOverview -v`
Expected: FAIL。

- [ ] **Step 3: 实现**

`model/supplier_stats.go` 追加:

```go
type SupplierOverviewSummary struct {
	SupplierTotal      int64 `json:"supplier_total"`
	SupplierEnabled    int64 `json:"supplier_enabled"`
	ChannelTotal       int64 `json:"channel_total"`
	ChannelAvailable   int64 `json:"channel_available"`
	ChannelUnavailable int64 `json:"channel_unavailable"`
}

type SupplierTypeGroup struct {
	Group       string  `json:"group"`
	LowestPrice float64 `json:"lowest_price"`
}

type SupplierTypeStat struct {
	Type          int                 `json:"type"`
	TypeName      string              `json:"type_name"`
	SupplierCount int                 `json:"supplier_count"`
	ChannelCount  int                 `json:"channel_count"`
	Available     int                 `json:"available"`
	Unavailable   int                 `json:"unavailable"`
	LowestPrice   float64             `json:"lowest_price"`
	Groups        []SupplierTypeGroup `json:"groups"`
}

type SupplierOverview struct {
	Summary SupplierOverviewSummary `json:"summary"`
	ByType  []SupplierTypeStat      `json:"by_type"`
}

// GetSupplierOverview 全局供应商概览:总数 + 按官key类型聚合供应情况。
func GetSupplierOverview() (SupplierOverview, error) {
	var ov SupplierOverview

	// 供应商总数/启用数(role=5 用户 + Supplier.enabled)
	DB.Model(&User{}).Where("role = ?", RoleSupplierUser).Count(&ov.Summary.SupplierTotal)
	DB.Model(&Supplier{}).Where("enabled = ?", true).Count(&ov.Summary.SupplierEnabled)

	// 供应商渠道精简列
	type chRow struct {
		Type       int
		Group      string
		Status     int
		SupplierId int
		CostPrice  float64
	}
	var rows []chRow
	if err := DB.Model(&Channel{}).
		Select("type, `group` AS `group`, status, supplier_id, cost_price").
		Where("supplier_id > 0").Scan(&rows).Error; err != nil {
		return ov, err
	}
	// 注:`group` 为保留字,用 commonGroupCol 拼接更稳妥(见 model/main.go);此处示意。

	type acc struct {
		suppliers   map[int]struct{}
		channels    int
		available   int
		unavailable int
		lowest      float64
		groupLowest map[string]float64
	}
	byType := map[int]*acc{}
	for _, r := range rows {
		ov.Summary.ChannelTotal++
		a := byType[r.Type]
		if a == nil {
			a = &acc{suppliers: map[int]struct{}{}, groupLowest: map[string]float64{}}
			byType[r.Type] = a
		}
		a.channels++
		a.suppliers[r.SupplierId] = struct{}{}
		if r.Status == ChannelStatusEnabled {
			a.available++
			ov.Summary.ChannelAvailable++
			if r.CostPrice > 0 && (a.lowest == 0 || r.CostPrice < a.lowest) {
				a.lowest = r.CostPrice
			}
			if r.CostPrice > 0 {
				if g, ok := a.groupLowest[r.Group]; !ok || r.CostPrice < g {
					a.groupLowest[r.Group] = r.CostPrice
				}
			}
		} else {
			a.unavailable++
			ov.Summary.ChannelUnavailable++
		}
	}
	for typ, a := range byType {
		groups := make([]SupplierTypeGroup, 0, len(a.groupLowest))
		for g, p := range a.groupLowest {
			groups = append(groups, SupplierTypeGroup{Group: g, LowestPrice: p})
		}
		sort.Slice(groups, func(i, j int) bool { return groups[i].LowestPrice < groups[j].LowestPrice })
		ov.ByType = append(ov.ByType, SupplierTypeStat{
			Type:          typ,
			TypeName:      constant.GetChannelTypeName(typ),
			SupplierCount: len(a.suppliers),
			ChannelCount:  a.channels,
			Available:     a.available,
			Unavailable:   a.unavailable,
			LowestPrice:   a.lowest,
			Groups:        groups,
		})
	}
	// 按渠道数降序,稳定展示
	sort.Slice(ov.ByType, func(i, j int) bool { return ov.ByType[i].ChannelCount > ov.ByType[j].ChannelCount })
	return ov, nil
}
```

> 确认 `Supplier` 模型存在且字段 `Enabled bool`(`model/supplier.go`);`ChannelStatusEnabled` 与 `constant` 包 import。保留字 `group` 用 `commonGroupCol` 更安全:`Select("type, " + commonGroupCol + " AS \"group\", status, supplier_id, cost_price")` —— 执行时按 `model/main.go` 既有写法对齐。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./model/ -run TestGetSupplierOverview -v`
Expected: PASS。

- [ ] **Step 5: 提交点(等用户指令)**

```bash
git add model/supplier_stats.go model/supplier_overview_test.go
git commit -m "feat(supplier): 供应商概览聚合 GetSupplierOverview"
```

---

### Task 8: `GET /api/admin/supplier-overview`(AdminAuth)

**Files:**
- Modify: `controller/supplier.go`, `router/api-router.go`

- [ ] **Step 1: 实现 controller**

`controller/supplier.go` 追加:

```go
// GetSupplierOverviewAdmin 管理员/超管可见的全局供应商概览。
func GetSupplierOverviewAdmin(c *gin.Context) {
	ov, err := model.GetSupplierOverview()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, ov)
}
```

- [ ] **Step 2: 注册路由(AdminAuth,满足"管理员+超管")**

`router/api-router.go` 新增组(放在 channel 组附近,**AdminAuth**):

```go
supplierOverviewRoute := apiRouter.Group("/admin/supplier-overview")
supplierOverviewRoute.Use(middleware.AdminAuth())
{
	supplierOverviewRoute.GET("/", controller.GetSupplierOverviewAdmin)
}
```

- [ ] **Step 3: 编译**

Run: `go build ./...`
Expected: 成功。

- [ ] **Step 4: 提交点(等用户指令)**

```bash
git add controller/supplier.go router/api-router.go
git commit -m "feat(supplier): GET /api/admin/supplier-overview(AdminAuth)"
```

---

# Phase F — 前端:供应商管理页(需求1)

### Task 9: `useSuppliersData` 加 summary / 排序 / 立即结算 state

**Files:**
- Modify: `web/classic/src/hooks/suppliers/useSuppliersData.jsx`

- [ ] **Step 1: 加 summary + 排序 + 确认弹窗 state**

在 `useSuppliersData` 内:
1. 新增 state:`const [summary, setSummary] = useState(null);`、`const [sortBy, setSortBy] = useState('');`、`const [sortOrder, setSortOrder] = useState('desc');`、`const [showConfirm, setShowConfirm] = useState(false);`、`const [confirmRecord, setConfirmRecord] = useState(null);`。
2. 新增 `loadSummary`:
```js
const loadSummary = async () => {
  const res = await API.get('/api/supplier/summary');
  const { success, data } = res.data;
  if (success) setSummary(data);
};
```
3. `loadSuppliers`/`searchSuppliers` 的 URL 追加排序参数:
```js
const sortParam = sortBy ? `&sort_by=${sortBy}&sort_order=${sortOrder}` : '';
// `/api/supplier/?p=${startIdx}&page_size=${pageSize}${sortParam}`
```
4. 新增排序变更入口:
```js
const handleSortChange = (nextSortBy, nextOrder) => {
  setSortBy(nextSortBy);
  setSortOrder(nextOrder);
  // 立即用新排序重载第一页
  loadSuppliers(0, pageSize, nextSortBy, nextOrder);
};
```
(把 `loadSuppliers/searchSuppliers` 签名扩展为可接收 `sortBy/sortOrder`,缺省取 state。)
5. 立即结算:
```js
const initiateSettlement = async (record) => {
  const res = await API.post('/api/admin/settlement/initiate', { supplier_id: record.user_id });
  const { success, message, data } = res.data;
  if (!success) { showError(message); return; }
  setConfirmRecord(data);   // 后端返回的新结算单(含 id / computed_cny)
  setShowConfirm(true);
};
const confirmSettlement = async (id, values) => {
  const res = await API.post(`/api/admin/settlement/${id}/confirm`, values);
  const { success, message } = res.data;
  if (success) { showSuccess(t('结算确认成功！')); await refresh(); await loadSummary(); return true; }
  showError(message); return false;
};
const closeConfirm = () => { setShowConfirm(false); setTimeout(() => setConfirmRecord(null), 300); };
```
6. `useEffect` 初始化追加 `loadSummary()`。
7. `return` 暴露:`summary, loadSummary, sortBy, sortOrder, handleSortChange, initiateSettlement, confirmSettlement, showConfirm, confirmRecord, closeConfirm`。

- [ ] **Step 2: 构建验证**

Run: `cd web/classic && bun run build`
Expected: 构建成功(后续 Task 接好 UI 再整体验证)。

- [ ] **Step 3: 提交点(等用户指令)**

```bash
git add web/classic/src/hooks/suppliers/useSuppliersData.jsx
git commit -m "feat(supplier-fe): useSuppliersData 接入汇总/排序/立即结算"
```

---

### Task 10: 列排序 + 「立即结算」按钮

**Files:**
- Modify: `web/classic/src/components/table/suppliers/SuppliersColumnDefs.jsx`, `SuppliersTable.jsx`

- [ ] **Step 1: 列定义加 sorter + 按钮**

`SuppliersColumnDefs.jsx`:
1. `getSuppliersColumns` 入参加 `onInitiateSettlement`。
2. 给「优先级/待结算/已结算」列加 `sorter: true`(服务端排序,Semi 会在 onChange 给出 sorter)。为映射列→`sort_by`,给这些列加自定义 `dataIndex` 已是 `priority`/`pending_cny`/`settled_cny`,与后端 `sort_by` 取值一致 ✅。
3. `renderOperations` 增加按钮:
```jsx
<Button
  type='primary' theme='light' size='small'
  onClick={() => onInitiateSettlement(record)}
>
  {t('立即结算')}
</Button>
```
(放在「编辑」前。)

- [ ] **Step 2: 表格接 onChange → 排序**

`SuppliersTable.jsx`:
1. 从 props 取 `handleSortChange`、`onInitiateSettlement`。
2. `getSuppliersColumns({..., onInitiateSettlement})`。
3. 给 `CardTable` 传 `onChange`:
```jsx
onChange={(changeInfo) => {
  const sorter = changeInfo?.sorter;
  if (sorter && sorter.dataIndex) {
    const order = sorter.sortOrder === 'ascend' ? 'asc' : 'desc';
    handleSortChange(sorter.dataIndex, order);
  }
}}
```
> 确认 `CardTable` 透传 Semi Table 的 `onChange`;若未透传,在 `CardTable` 增加该 prop 透传(`<Table onChange={onChange} ...>`)。

- [ ] **Step 3: 构建验证**

Run: `cd web/classic && bun run build`
Expected: 成功。

- [ ] **Step 4: 提交点(等用户指令)**

```bash
git add web/classic/src/components/table/suppliers/SuppliersColumnDefs.jsx web/classic/src/components/table/suppliers/SuppliersTable.jsx
git commit -m "feat(supplier-fe): 供应商列排序 + 立即结算按钮"
```

---

### Task 11: 顶部汇总条组件

**Files:**
- Create: `web/classic/src/components/table/suppliers/SuppliersSummaryBar.jsx`

- [ ] **Step 1: 新建组件**

```jsx
import React from 'react';
import { Card, Row, Col, Typography } from '@douyinfe/semi-ui';
import { Wallet, FileClock, CheckCircle2 } from 'lucide-react';

const { Text } = Typography;
const cny = (v) => `¥${(Number(v) || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
const usd = (v) => `$${(Number(v) || 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;

const StatCard = ({ icon: Icon, color, label, mainCny, subUsd, extra }) => (
  <Card className='!rounded-2xl w-full h-full' bordered={false}
    bodyStyle={{ padding: 16 }}
    style={{ border: '1px solid var(--semi-color-border)', background: 'var(--semi-color-bg-1)' }}>
    <div className='flex items-start gap-3'>
      <div className='flex items-center justify-center shrink-0'
        style={{ width: 40, height: 40, borderRadius: 11, background: `${color}22`, color }}>
        <Icon size={20} strokeWidth={2.2} />
      </div>
      <div className='min-w-0'>
        <Text type='tertiary' style={{ fontSize: 11, fontWeight: 600, letterSpacing: '0.05em' }}>{label}</Text>
        <div style={{ fontSize: 22, fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>{cny(mainCny)}</div>
        <Text type='tertiary' style={{ fontSize: 12 }}>{usd(subUsd)} {extra}</Text>
      </div>
    </div>
  </Card>
);

const SuppliersSummaryBar = ({ summary, t }) => {
  const pending = summary?.pending || {};
  const applied = summary?.applied || {};
  const settled = summary?.settled || {};
  return (
    <Row gutter={[12, 12]} className='mb-3 w-full'>
      <Col xs={24} sm={8}>
        <StatCard icon={Wallet} color='#f59e0b' label={t('待结算总额')}
          mainCny={pending.payable_cny} subUsd={pending.official_usd}
          extra={`· ${pending.supplier_count || 0} ${t('家')}`} />
      </Col>
      <Col xs={24} sm={8}>
        <StatCard icon={FileClock} color='#3b82f6' label={t('已申请结算')}
          mainCny={applied.computed_cny} subUsd={applied.official_usd}
          extra={`· ${applied.count || 0} ${t('单')}`} />
      </Col>
      <Col xs={24} sm={8}>
        <StatCard icon={CheckCircle2} color='#10b981' label={t('已结算')}
          mainCny={settled.actual_cny} subUsd={settled.official_usd}
          extra={settled.actual_usd > 0 ? `· ${t('另含')} $${(settled.actual_usd).toFixed(2)}` : `· ${settled.count || 0} ${t('单')}`} />
      </Col>
    </Row>
  );
};

export default SuppliersSummaryBar;
```

- [ ] **Step 2: 构建验证**

Run: `cd web/classic && bun run build`
Expected: 成功。

- [ ] **Step 3: 提交点(等用户指令)**

```bash
git add web/classic/src/components/table/suppliers/SuppliersSummaryBar.jsx
git commit -m "feat(supplier-fe): 供应商管理页顶部汇总条组件"
```

---

### Task 12: 组装供应商管理页(汇总条 + 确认弹窗)

**Files:**
- Modify: `web/classic/src/components/table/suppliers/index.jsx`

- [ ] **Step 1: 挂汇总条 + ConfirmModal**

`index.jsx`:
1. import:
```jsx
import SuppliersSummaryBar from './SuppliersSummaryBar';
import ConfirmModal from '../settlement-review/modals/ConfirmModal';
```
2. 从 `suppliersData` 解构 `summary, showConfirm, confirmRecord, confirmSettlement, closeConfirm`。
3. `CardPro` 的 `statsArea`(或在 `<CardPro>` 之上)插入 `<SuppliersSummaryBar summary={summary} t={t} />`。
4. 页面挂弹窗:
```jsx
<ConfirmModal
  visible={showConfirm}
  record={confirmRecord}
  onCancel={closeConfirm}
  confirmSettlement={confirmSettlement}
  t={t}
/>
```
5. 确保 `<SuppliersTable {...suppliersData} />` 已透传 `handleSortChange`、`onInitiateSettlement={suppliersData.initiateSettlement}`(在 index 里补 prop:`<SuppliersTable {...suppliersData} onInitiateSettlement={suppliersData.initiateSettlement} />`)。

- [ ] **Step 2: 构建验证**

Run: `cd web/classic && bun run build`
Expected: 成功。

- [ ] **Step 3: 提交点(等用户指令)**

```bash
git add web/classic/src/components/table/suppliers/index.jsx
git commit -m "feat(supplier-fe): 供应商管理页装配汇总条+确认弹窗"
```

---

# Phase G — 前端:渠道按供应商名搜索(需求2)

### Task 13: 渠道搜索框 + hook 接参

**Files:**
- Modify: `web/classic/src/components/table/channels/ChannelsFilters.jsx`, `web/classic/src/hooks/channels/useChannelsData.jsx`

- [ ] **Step 1: hook 接 searchSupplier**

`useChannelsData.jsx`:
1. `formInitValues`(`:126`)加 `searchSupplier: ''`。
2. `getFormValues`(`:312`)返回加 `searchSupplier: formValues.searchSupplier || ''`。
3. `loadChannels`(`:332`)取值与「有搜索条件」判断加入 `searchSupplier`:
```js
const { searchKeyword, searchGroup, searchModel, searchSupplier } = getFormValues();
if (searchKeyword !== '' || searchGroup !== '' || searchModel !== '' || searchSupplier !== '') {
```
4. `searchChannels`(`:386`)同样解构 `searchSupplier`,空判断(`:389`)加入它,URL(`:404`)追加:
```js
`...&supplier_name=${encodeURIComponent(searchSupplier)}`
```
5. `refresh`(`:427`)的空判断加入 `searchSupplier`。

- [ ] **Step 2: 过滤器加输入框**

`ChannelsFilters.jsx` 在 `searchModel` 输入框块后追加:
```jsx
<div className='w-full md:w-44'>
  <Form.Input
    size='small'
    field='searchSupplier'
    prefix={<IconSearch />}
    placeholder={t('供应商用户名/邮箱')}
    showClear
    pure
  />
</div>
```

- [ ] **Step 3: 构建验证**

Run: `cd web/classic && bun run build`
Expected: 成功。

- [ ] **Step 4: 提交点(等用户指令)**

```bash
git add web/classic/src/components/table/channels/ChannelsFilters.jsx web/classic/src/hooks/channels/useChannelsData.jsx
git commit -m "feat(channel-fe): 渠道管理页按供应商名搜索"
```

---

# Phase H — 前端:管理员供应商概览页(需求3/4/5)

### Task 14: 数据 hook

**Files:**
- Create: `web/classic/src/hooks/supplier-overview-admin/useSupplierOverviewData.jsx`

- [ ] **Step 1: 新建 hook**

```jsx
import { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { API, showError } from '../../helpers';

export const useSupplierOverviewData = () => {
  const { t } = useTranslation();
  const [loading, setLoading] = useState(true);
  const [data, setData] = useState(null);

  const load = async () => {
    setLoading(true);
    try {
      const res = await API.get('/api/admin/supplier-overview/');
      const { success, message, data: payload } = res.data;
      if (success) setData(payload); else showError(message);
    } catch (e) { showError(e); } finally { setLoading(false); }
  };

  useEffect(() => { load(); }, []);
  return { t, loading, data, refresh: load };
};
```

- [ ] **Step 2: 构建验证**

Run: `cd web/classic && bun run build`
Expected: 成功。

- [ ] **Step 3: 提交点(等用户指令)**

```bash
git add web/classic/src/hooks/supplier-overview-admin/useSupplierOverviewData.jsx
git commit -m "feat(supplier-fe): 管理员概览数据 hook"
```

---

### Task 15: 紧凑类型卡 + 下钻抽屉 + 页面

**Files:**
- Create: `web/classic/src/components/supplier-overview-admin/TypeCard.jsx`
- Create: `web/classic/src/components/supplier-overview-admin/TypeDetailSheet.jsx`
- Create: `web/classic/src/pages/SupplierOverviewAdmin/index.jsx`

- [ ] **Step 1: 紧凑类型卡 `TypeCard.jsx`**

```jsx
import React from 'react';
import { Card, Typography } from '@douyinfe/semi-ui';

const { Text } = Typography;

const TypeCard = ({ stat, onClick, t }) => {
  const available = stat.available || 0;
  const unavailable = stat.unavailable || 0;
  return (
    <Card
      className='!rounded-xl w-full h-full cursor-pointer transition-all duration-200 hover:-translate-y-0.5'
      bordered={false}
      bodyStyle={{ padding: 14 }}
      style={{ border: '1px solid var(--semi-color-border)', background: 'var(--semi-color-bg-1)' }}
      onClick={() => onClick(stat)}
    >
      <div className='flex items-center justify-between gap-2 mb-2'>
        <Text strong className='truncate' style={{ fontSize: 13 }}>{stat.type_name}</Text>
        <span className='shrink-0' style={{
          width: 8, height: 8, borderRadius: 999,
          background: unavailable > 0 ? 'var(--semi-color-warning)' : 'var(--semi-color-success)',
        }} />
      </div>
      <div className='flex items-baseline gap-1'>
        <span style={{ fontSize: 20, fontWeight: 700, fontVariantNumeric: 'tabular-nums' }}>{stat.supplier_count}</span>
        <Text type='tertiary' style={{ fontSize: 11 }}>{t('家供应')}</Text>
      </div>
      <div className='mt-1 flex items-center justify-between'>
        <Text type='tertiary' style={{ fontSize: 11 }}>
          {available}/{stat.channel_count} {t('可用')}
        </Text>
        <Text style={{ fontSize: 12, fontWeight: 600 }}>
          {stat.lowest_price > 0 ? `¥${stat.lowest_price.toFixed(2)}` : '—'}
        </Text>
      </div>
    </Card>
  );
};

export default TypeCard;
```

- [ ] **Step 2: 下钻抽屉 `TypeDetailSheet.jsx`**

```jsx
import React from 'react';
import { SideSheet, Table, Typography, Empty } from '@douyinfe/semi-ui';

const { Title, Text } = Typography;

const TypeDetailSheet = ({ visible, stat, onClose, t }) => {
  const groups = stat?.groups || [];
  const columns = [
    { title: t('分组'), dataIndex: 'group' },
    { title: t('最低价'), dataIndex: 'lowest_price',
      render: (v) => (v > 0 ? `¥${Number(v).toFixed(2)}` : '—') },
  ];
  return (
    <SideSheet
      title={<Title heading={5} className='!mb-0'>{stat?.type_name} · {t('供应明细')}</Title>}
      visible={visible}
      onCancel={onClose}
      width={420}
    >
      {stat ? (
        <div className='flex flex-col gap-3'>
          <div className='flex gap-4'>
            <Text type='tertiary'>{t('供应商')} <b>{stat.supplier_count}</b></Text>
            <Text type='tertiary'>{t('渠道')} <b>{stat.channel_count}</b></Text>
            <Text type='tertiary'>{t('可用')} <b>{stat.available}</b></Text>
          </div>
          {groups.length > 0
            ? <Table columns={columns} dataSource={groups} pagination={false} size='small' rowKey='group' />
            : <Empty description={t('暂无分组报价')} />}
        </div>
      ) : null}
    </SideSheet>
  );
};

export default TypeDetailSheet;
```

- [ ] **Step 3: 页面 `SupplierOverviewAdmin/index.jsx`(紧凑响应式网格)**

```jsx
import React, { useState } from 'react';
import { Row, Col, Card, Typography, Empty, Spin, Button } from '@douyinfe/semi-ui';
import { IconRefresh } from '@douyinfe/semi-icons';
import { Users, Plug } from 'lucide-react';
import { useSupplierOverviewData } from '../../hooks/supplier-overview-admin/useSupplierOverviewData';
import TypeCard from '../../components/supplier-overview-admin/TypeCard';
import TypeDetailSheet from '../../components/supplier-overview-admin/TypeDetailSheet';

const { Title, Text } = Typography;

const SummaryCard = ({ icon: Icon, color, label, value, sub }) => (
  <Card className='!rounded-2xl w-full h-full' bordered={false} bodyStyle={{ padding: 16 }}
    style={{ border: '1px solid var(--semi-color-border)', background: 'var(--semi-color-bg-1)' }}>
    <div className='flex items-center gap-3'>
      <div className='flex items-center justify-center shrink-0'
        style={{ width: 40, height: 40, borderRadius: 11, background: `${color}22`, color }}>
        <Icon size={20} strokeWidth={2.2} />
      </div>
      <div>
        <Text type='tertiary' style={{ fontSize: 11, fontWeight: 600 }}>{label}</Text>
        <div style={{ fontSize: 22, fontWeight: 700 }}>{value}</div>
        {sub ? <Text type='tertiary' style={{ fontSize: 12 }}>{sub}</Text> : null}
      </div>
    </div>
  </Card>
);

const SupplierOverviewAdmin = () => {
  const { t, loading, data, refresh } = useSupplierOverviewData();
  const [detail, setDetail] = useState(null);
  const summary = data?.summary || {};
  const byType = Array.isArray(data?.by_type) ? data.by_type : [];

  return (
    <div className='classic-page-fill px-4 md:px-6 pb-6 mt-[60px]'>
      <div className='flex items-center justify-between mb-4'>
        <Title heading={4} className='!mb-0'>{t('供应商概览')}</Title>
        <Button icon={<IconRefresh />} theme='light' type='tertiary' onClick={refresh} loading={loading}>
          {t('刷新')}
        </Button>
      </div>

      <Row gutter={[12, 12]} className='mb-4'>
        <Col xs={24} sm={12}>
          <SummaryCard icon={Users} color='#3b82f6' label={t('供应商')}
            value={summary.supplier_total || 0}
            sub={`${summary.supplier_enabled || 0} ${t('启用')}`} />
        </Col>
        <Col xs={24} sm={12}>
          <SummaryCard icon={Plug} color='#10b981' label={t('供应商渠道')}
            value={summary.channel_total || 0}
            sub={`${summary.channel_available || 0} ${t('可用')} · ${summary.channel_unavailable || 0} ${t('不可用')}`} />
        </Col>
      </Row>

      <Spin spinning={loading}>
        {byType.length === 0 ? (
          <Card className='!rounded-2xl' bordered={false}
            style={{ border: '1px solid var(--semi-color-border)' }}>
            <Empty description={t('暂无供应商渠道')} style={{ padding: '48px 0' }} />
          </Card>
        ) : (
          <Row gutter={[12, 12]}>
            {byType.map((stat) => (
              <Col key={stat.type} xs={12} sm={8} md={6} lg={4}>
                <TypeCard stat={stat} onClick={setDetail} t={t} />
              </Col>
            ))}
          </Row>
        )}
      </Spin>

      <TypeDetailSheet visible={!!detail} stat={detail} onClose={() => setDetail(null)} t={t} />
    </div>
  );
};

export default SupplierOverviewAdmin;
```

> 网格 `xs=12 sm=8 md=6 lg=4` → 移动 2 列、中屏 3~4 列、大屏 6 列;十几~几十种类型整齐换行,满足需求5。

- [ ] **Step 4: 构建验证**

Run: `cd web/classic && bun run build`
Expected: 成功(路由尚未接,组件先编译过)。

- [ ] **Step 5: 提交点(等用户指令)**

```bash
git add web/classic/src/components/supplier-overview-admin/ web/classic/src/pages/SupplierOverviewAdmin/
git commit -m "feat(supplier-fe): 管理员供应商概览页(紧凑卡片网格+下钻)"
```

---

### Task 16: 路由 + 侧栏菜单 + i18n

**Files:**
- Modify: `web/classic/src/App.jsx`, `web/classic/src/components/layout/SiderBar.jsx`, `web/classic/src/i18n/locales/zh-CN.json`

- [ ] **Step 1: 路由(AdminRoute)**

`App.jsx`:
1. 顶部 lazy:`const SupplierOverviewAdmin = lazy(() => import('./pages/SupplierOverviewAdmin'));`
2. 在 `Suppliers` 路由(`:418` 附近,`<AdminRoute>` 包裹)旁新增:
```jsx
<Route path='/console/supplier-overview' element={
  <AdminRoute><Suspense fallback={<Loading/>}><SupplierOverviewAdmin /></Suspense></AdminRoute>
} />
```
> 复制相邻 Suppliers 路由的写法对齐(Suspense/fallback 用现有同款)。

- [ ] **Step 2: 侧栏菜单**

`SiderBar.jsx`:
1. `routerMap`(`:33`)加 `supplier_overview: '/console/supplier-overview'`。
2. `adminItems`(`:209`)在「供应商管理」前/后追加:
```jsx
{ itemKey: 'supplier_overview', text: t('供应商概览'),
  itemKey2: 'supplier_overview', to: '/console/supplier-overview',
  className: isAdmin() ? '' : 'tableHiddle' },
```
> 字段结构对齐相邻 `suppliers` 菜单项(图标可选 lucide `LayoutGrid`/Semi 图标,沿用相邻项风格)。

- [ ] **Step 3: i18n 文案**

`zh-CN.json` 补齐(键即中文源串,值同):`供应商概览`、`待结算总额`、`已申请结算`、`已结算`、`家`、`单`、`另含`、`家供应`、`可用`、`不可用`、`供应商渠道`、`启用`、`供应明细`、`分组`、`最低价`、`暂无分组报价`、`暂无供应商渠道`、`立即结算`、`供应商用户名/邮箱`。逐个确认是否已存在,缺则添加。

- [ ] **Step 4: 构建验证**

Run: `cd web/classic && bun run build`
Expected: 成功。

- [ ] **Step 5: 提交点(等用户指令)**

```bash
git add web/classic/src/App.jsx web/classic/src/components/layout/SiderBar.jsx web/classic/src/i18n/locales/zh-CN.json
git commit -m "feat(supplier-fe): 供应商概览页 路由+菜单+i18n"
```

---

# Phase I — 集成验证

### Task 17: 全量测试 + 构建 + 本地实测

- [ ] **Step 1: 后端全量测试**

Run: `go test ./model/... ./controller/...`
Expected: 全部 PASS(含既有 157+ 测试,无回归)。

- [ ] **Step 2: 跨包编译**

Run: `go build ./...`
Expected: 成功。

- [ ] **Step 3: 前端构建**

Run: `cd web/classic && bun run build`
Expected: 成功无类型错误。

- [ ] **Step 4: 本地部署 + Playwright 实测(见 [[local-deploy-db-connection]])**

部署:`CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build`(打 VER 标记)→ `docker cp` → restart;classic 先 `bun run build`。
用 Playwright 在 5001 实测(超管登录):
1. 供应商管理页:顶部 3 张汇总卡显示(¥主数 + $小字);点列头排序(待结算/已结算 升降序);点某行「立即结算」→ 弹出确认弹窗(实付预填 computed_cny)→ 填写确认 → 提示成功 → 列表与汇总刷新。
2. 渠道管理页:输入供应商名 → 列表按 supplier_name 过滤;清空恢复。
3. 供应商概览页:侧栏「供应商概览」可进入;顶部汇总;紧凑卡片网格在窄屏(2 列)/宽屏(6 列)换行正常、无横向溢出;点卡片 → 抽屉显示分组报价。
4. (可选)以管理员(role=10)账号验证概览页可见、供应商管理页财务接口仍受 RootAuth 约束的预期行为。

- [ ] **Step 5: 记录 + 提交点(等用户指令)**

追加 `docs/superpowers/WORKLOG.md`(本次 v8 全量:何时/做了什么/改了哪些文件/如何验证/部署 VER)。
**等用户明确指令后**整体提交或按 Task 边界提交。

---

## Self-Review(已对照 spec)

- **需求1**:汇总条(Task1-3,11-12)、排序(Task4,10)、立即结算→确认弹窗(Task5,9,12,复用 `ConfirmModal`)✅
- **需求2**:渠道 supplier_name 过滤(Task6,13)✅
- **需求3/4/5**:`GetSupplierOverview` + AdminAuth 接口(Task7-8)、新页面紧凑网格 + 下钻 + 路由/菜单(Task14-16)✅;AdminAuth 满足"管理员+超管"✅;`xs=12…lg=4` 满足"十几种类型友好/卡片不过宽"✅
- **类型一致性**:`sort_by` 取值 `pending_cny/pending_usd/settled_cny/priority` 前后端一致;`SupplierTypeStat` 字段(`type/type_name/supplier_count/channel_count/available/unavailable/lowest_price/groups`)前后端一致;summary `pending/applied/settled` 三段字段前后端一致 ✅
- **占位扫描**:无 TBD;少量「先读现状/对齐字段名」是必要的真实约束(后端既有签名需按实对齐),非占位 ✅
- **三库兼容**:全 GORM + COALESCE + LIKE,无方言;`group` 保留字用 `commonGroupCol` 提示 ✅
- **提交纪律**:全程"等用户指令"门禁 ✅
