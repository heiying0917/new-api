# P3 供应商层级调度 + 健康检测 + 级联禁用 Implementation Plan

> 执行：subagent-driven。**最高风险阶段（请求调度热路径）**：强 TDD + 对抗评审；交付报告须注明"上线前在 staging 集成/压测"。提交策略：每单元跑通后停下汇报。

**Goal:** 在不重写现有渠道选择逻辑的前提下，叠加供应商层级调度（全局开关 `priority`/`bidding`）、禁用供应商的渠道排除出调度、渠道/供应商不可用级联禁用与恢复、定时健康检测。

**核心策略（外科手术式）:** 现有 `model/channel_cache.go:GetRandomSatisfiedChannel` 用 `channel.GetPriority()` 做"优先级分层 + 同层 weight 随机"。仅把分层用的优先级值替换为 `dispatchEffectivePriority(channel, strategy)`：
- `priority`：`SupplierPriority*1e9 + channelPriority`（全 0 时等同现状 → 向后兼容）
- `bidding`：`-round(cost_price*1000)`（价低→高层→先被选）
缓存里 Channel 已是完整对象；在 `InitChannelCache` 批量加载供应商，给缓存渠道填瞬态 `SupplierPriority`/`SupplierEnabled`，并把"禁用供应商"的渠道排除出候选。

**关键事实（勘探）:** `group2model2channels map[group]map[model][]int`(enabled 候选)、`channelsIDM map[int]*Channel`(全量)；`InitChannelCache()` 周期重建；`MemoryCacheEnabled` 为开关（关则走 `GetChannel`）。自动禁用 `service/channel.go` `DisableChannel/EnableChannel/ShouldDisableChannel/ShouldEnableChannel`；`model.UpdateChannelStatus` 改状态+ability+缓存。定时任务模式见 `AutomaticallyTestChannels`(controller/channel-test.go) / `SyncChannelCache`(main.go)。`testChannel(channel, userID, model, endpoint, isStream)` 探测单渠道。Option 读写 `common.OptionMap`+`model.UpdateOption`。

---

## 单元 P3-1：有效优先级 + 供应商缓存 + 调度策略（TDD，热路径核心）

### Files
- Modify `model/channel.go`（Channel 加瞬态字段 + `dispatchEffectivePriority` 纯函数）
- Modify `model/channel_cache.go`（InitChannelCache 填充供应商信息 + 候选排除禁用供应商；GetRandomSatisfiedChannel 用有效优先级）
- Modify `setting/`/`model/option.go`（DispatchStrategy 选项，默认 "priority"）
- Test `model/dispatch_test.go`（新建）

- [ ] **Step 1 — Channel 瞬态字段 + 纯函数**（`model/channel.go`）：
```go
// 瞬态（仅缓存用，不持久化）
SupplierPriority int  `json:"-" gorm:"-"`
SupplierEnabled  bool `json:"-" gorm:"-"`
```
```go
// DispatchStrategyPriority/Bidding 全局策略
func GetDispatchStrategy() string {
	common.OptionMapRWMutex.RLock()
	v := common.OptionMap["DispatchStrategy"]
	common.OptionMapRWMutex.RUnlock()
	if v == "bidding" {
		return "bidding"
	}
	return "priority"
}

// dispatchEffectivePriority 计算分层用有效优先级
func dispatchEffectivePriority(ch *Channel, strategy string) int64 {
	if strategy == "bidding" {
		cost := 0.0
		if ch.CostPrice != nil {
			cost = *ch.CostPrice
		}
		return -int64(cost * 1000) // 价低→值大→高层
	}
	// priority：供应商优先级为主，渠道优先级为辅
	return int64(ch.SupplierPriority)*1_000_000_000 + ch.GetPriority()
}
```
> `ch.GetPriority()` 返回 int64（核对）。assume channelPriority < 1e9。

- [ ] **Step 2 — 写失败测试**（`model/dispatch_test.go`）：纯函数与排序行为
```go
package model

import (
	"testing"
	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestDispatchEffectivePriority_BackwardCompat(t *testing.T) {
	p := int64(5)
	ch := &Channel{Priority: &p, SupplierPriority: 0}
	// priority 策略、供应商优先级 0 → 等同渠道优先级
	require.Equal(t, int64(5), dispatchEffectivePriority(ch, "priority"))
}
func TestDispatchEffectivePriority_SupplierDominates(t *testing.T) {
	p1, p2 := int64(100), int64(1)
	a := &Channel{Priority: &p1, SupplierPriority: 1} // 低供应商优先级但高渠道优先级
	b := &Channel{Priority: &p2, SupplierPriority: 2} // 高供应商优先级
	require.Greater(t, dispatchEffectivePriority(b, "priority"), dispatchEffectivePriority(a, "priority"))
}
func TestDispatchEffectivePriority_Bidding(t *testing.T) {
	c1, c2 := 2.5, 2.0
	a := &Channel{CostPrice: &c1}
	b := &Channel{CostPrice: &c2} // 更便宜
	require.Greater(t, dispatchEffectivePriority(b, "bidding"), dispatchEffectivePriority(a, "bidding"))
}
```
Run `go test ./model/ -run Dispatch -v` → 编译失败。

- [ ] **Step 3 — 实现 Step 1 代码** → 测试通过。确认 `common.OptionMap`/`OptionMapRWMutex` 可在 model 包访问（option.go 同包）。

- [ ] **Step 4 — InitChannelCache 填充供应商 + 排除禁用供应商**（`model/channel_cache.go`）：
在 `InitChannelCache()` 加载 channels 后、构建 `group2model2channels` 前：
```go
	// 批量加载供应商资料，填充瞬态字段
	supplierIds := make([]int, 0)
	for _, ch := range newChannelId2channel {
		if ch.SupplierId > 0 {
			supplierIds = append(supplierIds, ch.SupplierId)
		}
	}
	supplierMap := map[int]Supplier{}
	if len(supplierIds) > 0 {
		var sups []Supplier
		DB.Where("user_id IN ?", supplierIds).Find(&sups)
		for _, s := range sups {
			supplierMap[s.UserId] = s
		}
	}
	for _, ch := range newChannelId2channel {
		if ch.SupplierId == 0 {
			ch.SupplierEnabled = true // 无供应商（管理员渠道）始终可用
			ch.SupplierPriority = 0
		} else if s, ok := supplierMap[ch.SupplierId]; ok {
			ch.SupplierEnabled = s.Enabled
			ch.SupplierPriority = s.Priority
		} else {
			ch.SupplierEnabled = true // 找不到资料：放行（避免误杀）
		}
	}
```
在构建 `newGroup2model2channels` 候选列表时，跳过 `!channel.SupplierEnabled` 的渠道（即禁用供应商的渠道不进入候选）。注意：候选来自 enabled abilities，需在加入候选前查 `newChannelId2channel[abilityChannelId].SupplierEnabled`。

- [ ] **Step 5 — GetRandomSatisfiedChannel 用有效优先级**：在该函数开头读 `strategy := GetDispatchStrategy()`；把其中 3 处 `channel.GetPriority()`（提取 uniquePriorities、匹配 targetPriority、收集 targetChannels）替换为 `dispatchEffectivePriority(channel, strategy)`。**其余逻辑（weight 加权、retry→tier、smoothing）完全不动。** InitChannelCache 末尾对候选的 `sort.Slice ... GetPriority()` **保持不变**（GetRandomSatisfiedChannel 自行从候选集重算分层，不依赖输入顺序，少改少风险）。

- [ ] **Step 6 — DispatchStrategy 选项**：`model/option.go` `InitOptionMap()` 加 `common.OptionMap["DispatchStrategy"] = "priority"`。（前端系统设置开关在 P3-FE 或后续补；本期至少可经 option 持久化。）

- [ ] **Step 7 — 测试与回归**：`go test ./model/ -run 'Dispatch' -v`；`go test ./model/ -count=1`；`go vet ./model/`。
- [ ] **Step 8 — Checkpoint**。

## 单元 P3-2：级联禁用/恢复

### Files
- Modify `service/channel.go` 或 `model/channel.go`：加 `CascadeSupplierBySupplierId(supplierId)` 与在禁用/启用后调用
- Test `model/supplier_cascade_test.go`

- [ ] **Step 1 — 写失败测试**：
```go
func TestCascadeSupplier_AllDisabled(t *testing.T) {
	// 供应商 7 有 2 渠道，全 disabled → supplier.enabled=false
	// 任一 enabled → supplier.enabled=true
}
```
（建表 channels+suppliers，造数据，调 `CascadeSupplierBySupplierId(7)`，断言 supplier.enabled。）

- [ ] **Step 2 — 实现** `model/supplier.go`：
```go
func CascadeSupplierBySupplierId(supplierId int) error {
	if supplierId <= 0 {
		return nil
	}
	var enabledCount int64
	if err := DB.Model(&Channel{}).
		Where("supplier_id = ? AND status = ?", supplierId, common.ChannelStatusEnabled).
		Count(&enabledCount).Error; err != nil {
		return err
	}
	return DB.Model(&Supplier{}).Where("user_id = ?", supplierId).
		Update("enabled", enabledCount > 0).Error
}
```
- [ ] **Step 3 — 调用点**：在 `model.UpdateChannelStatus` 成功改状态后（渠道有 supplier_id 时）调用 `CascadeSupplierBySupplierId(channel.SupplierId)`。读 UpdateChannelStatus 找合适位置（拿到 channel 后）。
- [ ] **Step 4 — 测试 + vet**。Checkpoint。

## 单元 P3-3：定时健康检测任务

### Files
- Create `controller/supplier_health.go`（或加入 channel-test.go）
- Modify `main.go`（注册任务）
- Modify `model/option.go`（SupplierHealthCheckEnabled/IntervalMinutes 选项）

- [ ] **Step 1 — 选项**：OptionMap 加 `SupplierHealthCheckEnabled="false"`、`SupplierHealthCheckIntervalMinutes="30"`。
- [ ] **Step 2 — 任务**：仿 `StartChannelUpstreamModelUpdateTask`（sync.Once + IsMasterNode + ticker），首次延迟后周期执行：遍历**供应商渠道**（supplier_id>0 且 status=enabled），用 `testChannel` 探测；失败→`service.DisableChannel(...)`（构造 ChannelError，AutoBan 视为 true 以便自动禁用）；成功且当前为 AutoDisabled→`service.EnableChannel(...)`；每个渠道处理后 `model.CascadeSupplierBySupplierId(ch.SupplierId)`。读 `testChannel`、`DisableChannel`、`EnableChannel`、`resolveChannelTestUserID` 的真实签名照用。
- [ ] **Step 3 — main.go 注册**：仿现有 `go controller.AutomaticallyTestChannels()` 加 `controller.StartSupplierHealthCheckTask()`。
- [ ] **Step 4 — 校验**：`go vet ./controller/ ./model/`；`go build ./controller/`（避开 main 的 embed，用包级）；`go build ./` 会因 embed 失败属预期——改用 `go vet ./...`? 不行（embed）。用 `go build ./controller/ ./model/ ./service/` + `go vet`。`go test ./model/ -count=1`。
- [ ] **Step 5 — Checkpoint**。

## 验收（P3）
- DispatchStrategy=priority：供应商优先级高的渠道先被调度，同供应商内按渠道优先级；全 0 时与改造前行为一致（向后兼容）。
- DispatchStrategy=bidding：同候选集内成本价低者优先。
- 禁用的供应商其渠道不参与调度（缓存重建后生效）。
- 渠道全挂→供应商自动禁用；任一恢复→供应商自动启用（级联）。
- 定时健康检测可开关，按间隔探测供应商渠道并禁用/恢复。
- `go test ./model/` 全过；无回归。

## 风险（重点）
- **热路径**：GetRandomSatisfiedChannel 改动若有误会影响所有转发。务必只替换优先级取值、不动 weight/retry/smoothing；强 TDD + 对抗评审；**上线前 staging 集成/压测**。
- 缓存一致性：SupplierPriority/Enabled 在 InitChannelCache 重建时刷新；管理员改供应商优先级/启用后，需等下次缓存同步（SyncFrequency）生效——可接受；如需即时，UpdateSupplier 后调 InitChannelCache（可选）。
- MemoryCacheEnabled=false 时走 `GetChannel`（model/ability.go），本期 priority/bidding 仅在缓存路径实现；非缓存路径保持原行为（须在报告注明该限制）。
