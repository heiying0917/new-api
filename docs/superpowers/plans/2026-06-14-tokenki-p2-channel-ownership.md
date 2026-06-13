# P2 渠道归属 & 供应商自助 Implementation Plan

> 执行：subagent-driven。提交策略：每单元跑通后停下，按阶段提交（禁 push/部署）。后端 TDD（model 层内存 SQLite）；前端 typecheck+lint+build。

**Goal:** 渠道与供应商关联（`supplier_id`）+ 渠道成本价（`cost_price`，¥/$），供应商可自助增删改查**自己**的渠道并报价；管理员可见全部渠道的归属与成本价、可调任意渠道优先级。

**Architecture（低风险、纯叠加）:** 不改 admin 渠道热路径。新增专用供应商渠道控制器 `controller/supplier_channel.go`，作用域 = `c.GetInt("id")`，复用 `model.Channel` 的 `Insert/Update/Delete`（自动同步 Ability）。v1 供应商单渠道创建（批量/多 Key 后置）。

**关键事实（勘探确认）:** Channel 无 owner 字段；`channel.Insert()`→AddAbilities、`channel.Update()`→UpdateAbilities、`channel.Delete()`→DeleteAbilities；controller 取用户 `c.GetInt("id")`/`c.GetInt("role")`；channelRoute 为 AdminAuth；model 有 `GetChannelById(id, selectAll)`。

---

## 单元 P2-BE-1：Channel 字段 + 中间件 + model 查询（TDD）

### Files
- Modify `model/channel.go`（Channel 结构体加 2 字段）
- Modify `middleware/auth.go`（加 `SupplierAuth`）
- Modify `model/channel.go`（加 `GetChannelsBySupplier` / `SearchChannelsBySupplier`）
- Test `model/supplier_channel_test.go`（新建）

- [ ] **Step 1 — Channel 加字段**：在 `Channel` 结构体加
```go
	SupplierId int      `json:"supplier_id" gorm:"index;default:0"`
	CostPrice  *float64 `json:"cost_price" gorm:"default:0"` // 成本价 ¥/$
```
（Channel 已在 migrateDB/migrateDBFast 的 AutoMigrate 列表，自动加列，三库兼容。）

- [ ] **Step 2 — SupplierAuth 中间件**：在 `middleware/auth.go` 仿 `AdminAuth` 加
```go
func SupplierAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelper(c, common.RoleSupplierUser)
	}
}
```

- [ ] **Step 3 — 写失败测试** `model/supplier_channel_test.go`：
```go
package model

import (
	"testing"
	"github.com/stretchr/testify/require"
)

func setupSupplierChannelTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Channel{}, &Ability{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	require.NoError(t, DB.Exec("DELETE FROM abilities").Error)
}

func TestChannelSupplierFields(t *testing.T) {
	setupSupplierChannelTables(t)
	cp := 2.5
	ch := &Channel{Name: "c1", Key: "k1", SupplierId: 7, CostPrice: &cp, Models: "gpt-4", Group: "default"}
	require.NoError(t, DB.Create(ch).Error)
	var got Channel
	require.NoError(t, DB.First(&got, "id = ?", ch.Id).Error)
	require.Equal(t, 7, got.SupplierId)
	require.NotNil(t, got.CostPrice)
	require.Equal(t, 2.5, *got.CostPrice)
}

func TestGetChannelsBySupplier(t *testing.T) {
	setupSupplierChannelTables(t)
	cp := 2.0
	require.NoError(t, DB.Create(&Channel{Name: "a", Key: "k1", SupplierId: 7, CostPrice: &cp, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "b", Key: "k2", SupplierId: 7, CostPrice: &cp, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "c", Key: "k3", SupplierId: 9, CostPrice: &cp, Models: "m", Group: "g"}).Error)

	list, total, err := GetChannelsBySupplier(7, 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, list, 2)
	for _, c := range list {
		require.Equal(t, 7, c.SupplierId)
		require.Equal(t, "", c.Key) // key omitted
	}
}

func TestSearchChannelsBySupplier(t *testing.T) {
	setupSupplierChannelTables(t)
	cp := 2.0
	require.NoError(t, DB.Create(&Channel{Name: "alpha", Key: "k1", SupplierId: 7, CostPrice: &cp, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Name: "beta", Key: "k2", SupplierId: 7, CostPrice: &cp, Models: "m", Group: "g"}).Error)
	list, total, err := SearchChannelsBySupplier(7, "alpha", 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, list, 1)
	require.Equal(t, "alpha", list[0].Name)
}
```

- [ ] **Step 4 — 运行确认失败**：`go test ./model/ -run 'ChannelSupplier|ChannelsBySupplier' -v` → 编译失败（字段/函数未定义）。

- [ ] **Step 5 — 实现 model 查询**（`model/channel.go` 追加）：
```go
func GetChannelsBySupplier(supplierId, startIdx, num int) ([]*Channel, int64, error) {
	var channels []*Channel
	var total int64
	if err := DB.Model(&Channel{}).Where("supplier_id = ?", supplierId).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := DB.Where("supplier_id = ?", supplierId).Omit("key").Order("id desc").
		Limit(num).Offset(startIdx).Find(&channels).Error
	return channels, total, err
}

func SearchChannelsBySupplier(supplierId int, keyword string, startIdx, num int) ([]*Channel, int64, error) {
	var channels []*Channel
	var total int64
	like := "%" + keyword + "%"
	q := DB.Model(&Channel{}).Where("supplier_id = ?", supplierId).
		Where("name LIKE ? OR models LIKE ?", like, like)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := q.Omit("key").Order("id desc").Limit(num).Offset(startIdx).Find(&channels).Error
	return channels, total, err
}
```

- [ ] **Step 6 — 运行确认通过**：`go test ./model/ -run 'ChannelSupplier|ChannelsBySupplier' -v` → PASS；`go vet ./model/ ./middleware/`。
- [ ] **Step 7 — Checkpoint**。

## 单元 P2-BE-2：供应商渠道控制器 + 路由

### Files
- Create `controller/supplier_channel.go`
- Modify `router/api-router.go`（加 `/api/supplier/channel` 组，SupplierAuth）

- [ ] **Step 1 — controller/supplier_channel.go**：
```go
package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// SupplierListChannels 列出当前供应商自己的渠道
func SupplierListChannels(c *gin.Context) {
	supplierId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")
	var (
		list  []*model.Channel
		total int64
		err   error
	)
	if keyword != "" {
		list, total, err = model.SearchChannelsBySupplier(supplierId, keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	} else {
		list, total, err = model.GetChannelsBySupplier(supplierId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(list)
	common.ApiSuccess(c, pageInfo)
}

// SupplierGetChannel 取自己的单个渠道（含 key）
func SupplierGetChannel(c *gin.Context) {
	supplierId := c.GetInt("id")
	id, _ := strconv.Atoi(c.Param("id"))
	ch, err := model.GetChannelById(id, true)
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

// SupplierAddChannel 创建渠道（强制归属本人 + 成本价必填）
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
	ch.Id = 0
	ch.SupplierId = supplierId
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
	// 强制归属与防越权字段
	patch.SupplierId = supplierId
	if err := patch.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, nil)
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
```
> 实现注意：`patch.Update()` 用 GORM `Updates(channel)`，对 `*` 指针/零值的更新行为需核对——若担心客户端漏传字段导致清空，可在 update 前用 existing 兜底（读 controller/channel.go:880-890 的 ChannelInfo 兜底模式照做）。本 v1 假设前端提交完整渠道对象。

- [ ] **Step 2 — 路由**：在 `router/api-router.go` 既有 `supplierAdminRoute`（/supplier，RootAuth）之后，新增 SupplierAuth 作用域组：
```go
		supplierSelfRoute := apiRouter.Group("/supplier/channel")
		supplierSelfRoute.Use(middleware.SupplierAuth())
		{
			supplierSelfRoute.GET("/", controller.SupplierListChannels)
			supplierSelfRoute.GET("/:id", controller.SupplierGetChannel)
			supplierSelfRoute.POST("/", controller.SupplierAddChannel)
			supplierSelfRoute.PUT("/", controller.SupplierUpdateChannel)
			supplierSelfRoute.DELETE("/:id", controller.SupplierDeleteChannel)
		}
```
> 注意 `/supplier/channel` 与既有 `/supplier`（RootAuth）路径前缀不同段，Gin 分组互不冲突。确认 `middleware`、`controller` 已 import。

- [ ] **Step 3 — 校验**：`go vet ./controller/ ./router/ ./model/`；`go build ./controller/ ./router/`；`go test ./model/ -count=1`。
- [ ] **Step 4 — Checkpoint**。

## 单元 P2-FE：供应商「我的渠道」页（前端）
> 镜像 `features/channels` 过于复杂；v1 用**简化渠道表单**覆盖核心字段。仅供应商可见。

### Files（新建 `web/default/src/features/my-channels/`）
- `types.ts`、`api.ts`（接 `/api/supplier/channel/`）、`index.tsx`、`components/{provider,columns,table,mutate-drawer}.tsx`
- 路由 `web/default/src/routes/_authenticated/my-channels/index.tsx`（守卫 `role >= ROLE.SUPPLIER`）
- 菜单：`use-sidebar-data.ts` 在 personal 或新「供应商」组加「我的渠道」(`minRole: ROLE.SUPPLIER`，且对 admin 也可见？——设 `minRole: ROLE.SUPPLIER`，但 admin 用 /channels，故仅希望供应商可见。用专门判断：见下)
- i18n

### 字段（简化渠道表单）
渠道类型 `type`(下拉，复用 channels 的类型选项)、名称 `name`、`base_url`(可选)、分组 `group`(下拉/输入)、模型 `models`(多选或逗号文本)、密钥 `key`、优先级 `priority`、**成本价 `cost_price`(必填, ¥/$)**、备注 `remark`、模型重定向 `model_mapping`(可选 JSON 文本)。

- [ ] **Step 1** — `api.ts`：`getMyChannels({p,page_size,keyword})` GET `/api/supplier/channel/?...`；`getMyChannel(id)` GET `/api/supplier/channel/:id`；`addMyChannel(channel)` POST；`updateMyChannel(channel)` PUT；`deleteMyChannel(id)` DELETE。返回 `res.data`。
- [ ] **Step 2** — 列表页（镜像 suppliers 页的 table/columns/provider 模式，列：ID/名称/类型/分组/模型/优先级/成本价/状态/操作[编辑·删除]）。
- [ ] **Step 3** — 编辑/新建抽屉（RHF+zod，上面的字段；cost_price 必填>0）。
- [ ] **Step 4** — 路由（守卫 role>=ROLE.SUPPLIER）+ 菜单项 + i18n。
- [ ] **Step 5** — 校验：`bun run typecheck && bun run lint && bun run build`。手动冒烟：供应商登录→「我的渠道」→新建（填 key+成本价）→列表出现→编辑→删除；只能看到自己的。
- [ ] **Step 6 — Checkpoint**。

## 验收（P2）
- Channel 有 `supplier_id`、`cost_price` 列；供应商经 `/api/supplier/channel/*` 仅能增删改查自己的渠道，新建必填成本价>0；越权访问被拒。
- 渠道增改删正确同步 Ability（沿用 Insert/Update/Delete）。
- 管理员渠道列表能看到 supplier_id/cost_price，可调任意渠道优先级（沿用现有 admin 接口）。
- 后端 `go test ./model/` 全过；前端 typecheck+lint+build 过。

## 风险/简化
- v1 供应商建渠道为**单渠道**（不支持批量/多 Key 模式）；后续可扩展。
- `SupplierUpdateChannel` 依赖前端提交较完整的渠道对象；若出现字段被清空问题，按 controller/channel.go 的兜底模式加载 existing 合并。
- 菜单"我的渠道"对 admin 是否展示：admin 用 /channels；如不希望 admin 看到"我的渠道"，用精确判断（见 FE 实现，必要时加 maxRole 或专用过滤）。
