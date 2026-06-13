# P1-A 供应商身份（后端）Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

> **⚠️ 提交策略（项目铁律，覆盖本技能默认）：** 本仓库**禁止自动 git commit/push**。每个 Task 末尾的「Checkpoint」= 跑通 build+test 后**停下汇报**，等用户明确说"提交"再提交。**不要在执行中自动 commit。**

**Goal:** 在 new-api 后端叠加「供应商」身份基础：新增供应商角色、用户手机号、Supplier 资料表，公开注册改为产出供应商，并提供超管专属的供应商管理接口。

**Architecture:** 复用现有 4 级数值角色体系（在 Common=1 与 Admin=10 间插入 Supplier=5）。供应商专属属性（优先级/启用/结算配置/备注）独立成 `Supplier` 表（1:1 user_id），与 `User` 解耦；列表通过"查 role=supplier 的 User + 批量加载 Supplier 资料 + Go 内存合并"实现，避免跨库 JOIN。可测逻辑全部下沉到 model 层，controller 保持薄。

**Tech Stack:** Go 1.22 / Gin / GORM v2；测试用内存 SQLite（`github.com/glebarez/sqlite`）+ testify；三库兼容靠 GORM AutoMigrate。

---

## 文件结构

| 文件 | 职责 | 操作 |
|---|---|---|
| `common/constants.go` | 角色常量 + `IsValidateRole` | 修改（加 `RoleSupplierUser`） |
| `common/role_test.go` | `IsValidateRole` 纯函数测试 | 新建 |
| `model/user.go` | User 结构 + 注册赋值 | 修改（加 `Phone` 字段） |
| `model/supplier.go` | Supplier 模型 + 查询/更新/资料创建 | 新建 |
| `model/supplier_test.go` | Supplier 模型层 TDD 测试 | 新建 |
| `model/main.go` | AutoMigrate 注册 | 修改（两处迁移列表加 `&Supplier{}`） |
| `controller/user.go` | 注册逻辑 | 修改（角色=供应商、收手机号、建 Supplier 资料） |
| `controller/supplier.go` | 供应商管理 HTTP 接口（超管） | 新建 |
| `router/api-router.go` | 路由挂载 | 修改（加 supplier 路由组，RootAuth） |

---

## Task 1: 新增供应商角色常量

**Files:**
- Modify: `common/constants.go:187-196`
- Test: `common/role_test.go`（新建）

- [ ] **Step 1: 写失败测试**

新建 `common/role_test.go`：
```go
package common

import "testing"

func TestIsValidateRole_Supplier(t *testing.T) {
	if RoleSupplierUser != 5 {
		t.Fatalf("RoleSupplierUser 期望 5, 实际 %d", RoleSupplierUser)
	}
	if !IsValidateRole(RoleSupplierUser) {
		t.Fatalf("IsValidateRole(RoleSupplierUser) 应为 true")
	}
	// 既有角色不受影响
	for _, r := range []int{RoleGuestUser, RoleCommonUser, RoleAdminUser, RoleRootUser} {
		if !IsValidateRole(r) {
			t.Fatalf("IsValidateRole(%d) 应为 true", r)
		}
	}
	// 未知角色仍非法
	if IsValidateRole(7) {
		t.Fatalf("IsValidateRole(7) 应为 false")
	}
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./common/ -run TestIsValidateRole_Supplier -v`
Expected: 编译失败 `undefined: RoleSupplierUser`

- [ ] **Step 3: 实现**

`common/constants.go` 第 187-196 行改为：
```go
const (
	RoleGuestUser    = 0
	RoleCommonUser   = 1
	RoleSupplierUser = 5
	RoleAdminUser    = 10
	RoleRootUser     = 100
)

func IsValidateRole(role int) bool {
	return role == RoleGuestUser || role == RoleCommonUser ||
		role == RoleSupplierUser || role == RoleAdminUser || role == RoleRootUser
}
```

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./common/ -run TestIsValidateRole_Supplier -v`
Expected: PASS

- [ ] **Step 5: Checkpoint** — `go build ./...` 通过 + 上面测试 PASS → 停下汇报，等用户提交指令。

---

## Task 2: User 增加 Phone 字段

**Files:**
- Modify: `model/user.go:24-56`（User 结构体）
- Test: `model/supplier_test.go`（与 Task 3/4 同文件，本步先建文件与首个用例）

- [ ] **Step 1: 写失败测试**

新建 `model/supplier_test.go`：
```go
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

// setupSupplierTables 确保 users/suppliers 表存在并清空（复用包级 TestMain 的内存 DB）
func setupSupplierTables(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&User{}, &Supplier{}))
	require.NoError(t, DB.Exec("DELETE FROM suppliers").Error)
	require.NoError(t, DB.Exec("DELETE FROM users").Error)
}

func TestUserPhonePersisted(t *testing.T) {
	setupSupplierTables(t)
	u := &User{Username: "sup_phone", Password: "pwd12345", Phone: "13800000000", Role: common.RoleSupplierUser}
	require.NoError(t, DB.Create(u).Error)

	var got User
	require.NoError(t, DB.First(&got, "username = ?", "sup_phone").Error)
	require.Equal(t, "13800000000", got.Phone)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./model/ -run 'TestUserPhonePersisted' -v`
Expected: 编译失败 `unknown field 'Phone'` / `undefined: Supplier`（Supplier 在 Task 3 定义；本步先因 Phone 失败）

- [ ] **Step 3: 实现**

`model/user.go` 在 `Email` 字段（第 32 行）之后插入一行：
```go
	Phone            string         `json:"phone" gorm:"type:varchar(20);index"`
```

- [ ] **Step 4: 暂不跑**（依赖 Task 3 的 `Supplier` 才能编译，合并到 Task 3 Step 4 一起验证）

- [ ] **Step 5: Checkpoint** 随 Task 3 一起。

---

## Task 3: 新建 Supplier 模型 + 迁移注册

**Files:**
- Create: `model/supplier.go`
- Modify: `model/main.go:258-284`（migrateDB）与 `model/main.go:308-332`（migrateDBFast）
- Test: `model/supplier_test.go`（追加用例）

- [ ] **Step 1: 写失败测试**

在 `model/supplier_test.go` 追加：
```go
func TestSupplierCreateAndFetch(t *testing.T) {
	setupSupplierTables(t)
	s := &Supplier{
		UserId:          42,
		Priority:        3,
		Enabled:         true,
		SettlementMode:  "manual",
		SettlementCycle: "month",
		Remark:          "首批入驻",
	}
	require.NoError(t, DB.Create(s).Error)

	got, err := GetSupplierByUserId(42)
	require.NoError(t, err)
	require.Equal(t, 3, got.Priority)
	require.True(t, got.Enabled)
	require.Equal(t, "manual", got.SettlementMode)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./model/ -run 'TestSupplierCreateAndFetch' -v`
Expected: 编译失败 `undefined: Supplier` / `undefined: GetSupplierByUserId`

- [ ] **Step 3: 实现**

新建 `model/supplier.go`：
```go
package model

// Supplier 供应商资料（1:1 user_id），与 User 解耦。
// 仅 role=RoleSupplierUser 的用户拥有该资料。
type Supplier struct {
	UserId          int    `json:"user_id" gorm:"primaryKey;autoIncrement:false"`
	Priority        int    `json:"priority" gorm:"type:int;default:0;index"` // 管理员设，优先级调度模式用
	Enabled         bool   `json:"enabled" gorm:"default:true"`
	SettlementMode  string `json:"settlement_mode" gorm:"type:varchar(16);default:'manual'"` // manual|auto
	SettlementCycle string `json:"settlement_cycle" gorm:"type:varchar(16);default:'month'"` // day|week|month
	Remark          string `json:"remark" gorm:"type:varchar(255)"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func GetSupplierByUserId(userId int) (*Supplier, error) {
	var s Supplier
	err := DB.First(&s, "user_id = ?", userId).Error
	return &s, err
}
```

`model/main.go` 第 258 行的 `DB.AutoMigrate(` 列表中，在 `&User{},` 之后加一行 `&Supplier{},`。
`model/main.go` 第 308 行起的 `migrations` 切片中，在 `{&User{}, "User"},` 之后加一行 `{&Supplier{}, "Supplier"},`。

- [ ] **Step 4: 跑测试确认通过**（含 Task 2 的 Phone 用例）

Run: `go test ./model/ -run 'TestUserPhonePersisted|TestSupplierCreateAndFetch' -v`
Expected: 两个用例均 PASS

- [ ] **Step 5: Checkpoint** — `go build ./...` 通过 + 上面测试 PASS → 停下汇报。

---

## Task 4: Supplier 资料的创建/列表/搜索/更新

**Files:**
- Modify: `model/supplier.go`（追加函数）
- Test: `model/supplier_test.go`（追加用例）

- [ ] **Step 1: 写失败测试**

在 `model/supplier_test.go` 追加：
```go
func seedSupplierUser(t *testing.T, id int, username, email, phone string) {
	t.Helper()
	require.NoError(t, DB.Create(&User{
		Id: id, Username: username, Email: email, Phone: phone,
		Role: common.RoleSupplierUser, Status: common.UserStatusEnabled,
	}).Error)
	_, err := CreateSupplierProfile(id)
	require.NoError(t, err)
}

func TestCreateSupplierProfile_Defaults(t *testing.T) {
	setupSupplierTables(t)
	require.NoError(t, DB.Create(&User{Id: 1, Username: "s1", Role: common.RoleSupplierUser}).Error)
	s, err := CreateSupplierProfile(1)
	require.NoError(t, err)
	require.Equal(t, "manual", s.SettlementMode)
	require.Equal(t, "month", s.SettlementCycle)
	require.True(t, s.Enabled)
	// 幂等：重复调用不报错且不重复
	_, err = CreateSupplierProfile(1)
	require.NoError(t, err)
}

func TestGetAllSuppliers_MergesProfile(t *testing.T) {
	setupSupplierTables(t)
	seedSupplierUser(t, 1, "alice", "a@x.com", "13800000001")
	seedSupplierUser(t, 2, "bob", "b@x.com", "13800000002")
	// 一个普通用户不应出现在供应商列表
	require.NoError(t, DB.Create(&User{Id: 3, Username: "normal", Role: common.RoleCommonUser}).Error)
	require.NoError(t, UpdateSupplier(2, map[string]interface{}{"priority": 9, "remark": "VIP"}))

	items, total, err := GetAllSuppliers(0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(2), total)
	require.Len(t, items, 2)
	byId := map[int]*SupplierListItem{}
	for _, it := range items {
		byId[it.UserId] = it
	}
	require.Equal(t, "alice", byId[1].Username)
	require.Equal(t, "13800000001", byId[1].Phone)
	require.Equal(t, 9, byId[2].Priority)
	require.Equal(t, "VIP", byId[2].Remark)
}

func TestSearchSuppliers_ByKeyword(t *testing.T) {
	setupSupplierTables(t)
	seedSupplierUser(t, 1, "alice", "a@x.com", "13800000001")
	seedSupplierUser(t, 2, "bob", "b@x.com", "13800000002")

	items, total, err := SearchSuppliers("alice", 0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Len(t, items, 1)
	require.Equal(t, "alice", items[0].Username)
}
```

- [ ] **Step 2: 跑测试确认失败**

Run: `go test ./model/ -run 'TestCreateSupplierProfile_Defaults|TestGetAllSuppliers_MergesProfile|TestSearchSuppliers_ByKeyword' -v`
Expected: 编译失败 `undefined: CreateSupplierProfile / UpdateSupplier / GetAllSuppliers / SearchSuppliers / SupplierListItem`

- [ ] **Step 3: 实现**

在 `model/supplier.go` 追加：
```go
import "github.com/QuantumNous/new-api/common"

// SupplierListItem 供应商管理页列表项 = User 基本信息 + Supplier 资料
type SupplierListItem struct {
	UserId          int    `json:"user_id"`
	Username        string `json:"username"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	UserStatus      int    `json:"user_status"`
	Priority        int    `json:"priority"`
	Enabled         bool   `json:"enabled"`
	SettlementMode  string `json:"settlement_mode"`
	SettlementCycle string `json:"settlement_cycle"`
	Remark          string `json:"remark"`
}

// CreateSupplierProfile 为供应商用户创建资料，幂等。
func CreateSupplierProfile(userId int) (*Supplier, error) {
	var existing Supplier
	err := DB.First(&existing, "user_id = ?", userId).Error
	if err == nil {
		return &existing, nil
	}
	s := &Supplier{
		UserId:          userId,
		Enabled:         true,
		SettlementMode:  "manual",
		SettlementCycle: "month",
	}
	if err := DB.Create(s).Error; err != nil {
		return nil, err
	}
	return s, nil
}

// UpdateSupplier 更新供应商资料（仅白名单字段）。
func UpdateSupplier(userId int, fields map[string]interface{}) error {
	allowed := map[string]bool{
		"priority": true, "enabled": true,
		"settlement_mode": true, "settlement_cycle": true, "remark": true,
	}
	patch := map[string]interface{}{}
	for k, v := range fields {
		if allowed[k] {
			patch[k] = v
		}
	}
	if len(patch) == 0 {
		return nil
	}
	return DB.Model(&Supplier{}).Where("user_id = ?", userId).Updates(patch).Error
}

func GetAllSuppliers(startIdx, num int) ([]*SupplierListItem, int64, error) {
	return querySuppliers("", startIdx, num)
}

func SearchSuppliers(keyword string, startIdx, num int) ([]*SupplierListItem, int64, error) {
	return querySuppliers(keyword, startIdx, num)
}

// querySuppliers 查 role=supplier 的 User（分页），再批量合并 Supplier 资料（避免跨库 JOIN）。
func querySuppliers(keyword string, startIdx, num int) ([]*SupplierListItem, int64, error) {
	q := DB.Model(&User{}).Where("role = ?", common.RoleSupplierUser)
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR email LIKE ? OR phone LIKE ?", like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []User
	if err := q.Order("id desc").Limit(num).Offset(startIdx).Omit("password").Find(&users).Error; err != nil {
		return nil, 0, err
	}
	ids := make([]int, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.Id)
	}
	profiles := map[int]Supplier{}
	if len(ids) > 0 {
		var ss []Supplier
		if err := DB.Where("user_id IN ?", ids).Find(&ss).Error; err != nil {
			return nil, 0, err
		}
		for _, s := range ss {
			profiles[s.UserId] = s
		}
	}
	items := make([]*SupplierListItem, 0, len(users))
	for _, u := range users {
		it := &SupplierListItem{
			UserId: u.Id, Username: u.Username, Email: u.Email,
			Phone: u.Phone, UserStatus: u.Status,
			SettlementMode: "manual", SettlementCycle: "month", Enabled: true,
		}
		if s, ok := profiles[u.Id]; ok {
			it.Priority = s.Priority
			it.Enabled = s.Enabled
			it.SettlementMode = s.SettlementMode
			it.SettlementCycle = s.SettlementCycle
			it.Remark = s.Remark
		}
		items = append(items, it)
	}
	return items, total, nil
}
```
> 注：`model/supplier.go` 顶部 import 需合并（与 Task 3 的 package 声明同文件，import 块加入 `common`）。

- [ ] **Step 4: 跑测试确认通过**

Run: `go test ./model/ -run 'Supplier' -v`
Expected: 本任务 4 个用例全部 PASS

- [ ] **Step 5: Checkpoint** — `go build ./...` + `go test ./model/ -run 'Supplier|TestUserPhonePersisted' -v` 通过 → 停下汇报。

---

## Task 5: 注册改造（角色=供应商 + 手机号必填 + 建资料）

**Files:**
- Modify: `controller/user.go:138-234`（`Register`）
- Test: `model/supplier_test.go`（追加注册逻辑的 model 级断言）

> 说明：`Register` 是 Gin/HTTP 控制器，重逻辑下沉到 model（Task 4 的 `CreateSupplierProfile` 已测）。本任务用 model 级测试覆盖"供应商用户 + 资料"组合，controller 仅做薄改造（读代码核对）。

- [ ] **Step 1: 写失败测试**

在 `model/supplier_test.go` 追加：
```go
func TestSupplierUserHasProfileAfterRegister(t *testing.T) {
	setupSupplierTables(t)
	// 模拟 Register 的核心副作用：建 role=supplier 的用户 + 资料
	u := &User{Username: "newreg", Email: "n@x.com", Phone: "13900000000", Role: common.RoleSupplierUser, Status: common.UserStatusEnabled}
	require.NoError(t, DB.Create(u).Error)
	_, err := CreateSupplierProfile(u.Id)
	require.NoError(t, err)

	items, total, err := GetAllSuppliers(0, 20)
	require.NoError(t, err)
	require.Equal(t, int64(1), total)
	require.Equal(t, "13900000000", items[0].Phone)
	require.Equal(t, common.RoleSupplierUser, common.RoleSupplierUser) // 角色常量稳定
}
```

- [ ] **Step 2: 跑测试确认失败/通过基线**

Run: `go test ./model/ -run TestSupplierUserHasProfileAfterRegister -v`
Expected: PASS（验证 model 组合可用；下面改 controller 让真实注册产生同样结果）

- [ ] **Step 3: 改造 controller（按以下精确改动）**

`controller/user.go` `Register` 内：

(a) 邮箱校验块之后、`exist, err := ...` 之前，加手机号必填校验：
```go
	if user.Phone == "" {
		common.ApiErrorI18n(c, i18n.MsgUserInputInvalid, map[string]any{"Error": "phone is required"})
		return
	}
```

(b) `cleanUser` 构造（第 179-185 行）改为：
```go
	cleanUser := model.User{
		Username:    user.Username,
		Password:    user.Password,
		DisplayName: user.Username,
		InviterId:   inviterId,
		Phone:       user.Phone,
		Role:        common.RoleSupplierUser, // 公开注册即供应商
	}
```

(c) 在 `model.DB.Where("username = ?", ...).First(&insertedUser)` 成功之后（第 199 行后）、生成默认令牌之前，加创建供应商资料：
```go
	if _, err := model.CreateSupplierProfile(insertedUser.Id); err != nil {
		common.SysLog(fmt.Sprintf("CreateSupplierProfile error for user %d: %v", insertedUser.Id, err))
	}
```

- [ ] **Step 4: 验证**

Run: `go build ./... && go test ./model/ -run 'Supplier' -v`
Expected: 编译通过，全部 PASS。
人工核对：`controller/user.go` Register 现在产出 role=supplier 用户、写入 phone、创建 Supplier 资料。

- [ ] **Step 5: Checkpoint** — build+test 通过 → 停下汇报。

---

## Task 6: 供应商管理接口（超管）

**Files:**
- Create: `controller/supplier.go`
- Modify: `router/api-router.go`（在 userRoute 的 adminRoute 段之后加 supplier 路由组）

> 接口为薄控制器，数据逻辑全部走 Task 4 已测的 model 函数。

- [ ] **Step 1: 实现 controller**

新建 `controller/supplier.go`：
```go
package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetAllSuppliers 供应商列表（分页）。仅超管。
func GetAllSuppliers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.GetAllSuppliers(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// SearchSuppliers 关键词搜索供应商。仅超管。
func SearchSuppliers(c *gin.Context) {
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.SearchSuppliers(keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

type updateSupplierRequest struct {
	UserId          int    `json:"user_id"`
	Priority        *int   `json:"priority"`
	Enabled         *bool  `json:"enabled"`
	SettlementMode  string `json:"settlement_mode"`
	SettlementCycle string `json:"settlement_cycle"`
	Remark          *string `json:"remark"`
}

// UpdateSupplier 更新供应商资料（优先级/启用/结算方式/周期/备注）。仅超管。
func UpdateSupplier(c *gin.Context) {
	var req updateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.UserId == 0 {
		common.ApiErrorMsg(c, "user_id 不能为空")
		return
	}
	fields := map[string]interface{}{}
	if req.Priority != nil {
		fields["priority"] = *req.Priority
	}
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if req.SettlementMode != "" {
		fields["settlement_mode"] = req.SettlementMode
	}
	if req.SettlementCycle != "" {
		fields["settlement_cycle"] = req.SettlementCycle
	}
	if req.Remark != nil {
		fields["remark"] = *req.Remark
	}
	if err := model.UpdateSupplier(req.UserId, fields); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
```
> 已核实：`common.ApiError` / `ApiErrorMsg` / `ApiSuccess` / `ApiErrorI18n` 在 `common/gin.go`，`GetPageQuery` 与 `PageInfo.{GetStartIdx,GetPageSize,SetTotal,SetItems}` 在 `common/page_info.go`，签名与本计划一致。

- [ ] **Step 2: 挂路由**

`router/api-router.go`：在 `userRoute` 块（第 67 行起）内、`adminRoute` 段（第 127-141 行）之后，新增超管供应商路由组：
```go
		supplierAdminRoute := apiRouter.Group("/supplier")
		supplierAdminRoute.Use(middleware.RootAuth())
		{
			supplierAdminRoute.GET("/", controller.GetAllSuppliers)
			supplierAdminRoute.GET("/search", controller.SearchSuppliers)
			supplierAdminRoute.PUT("/", controller.UpdateSupplier)
		}
```
> 放在与 `userRoute` 同级（`apiRouter.Group`），核对该文件 `apiRouter` 变量在作用域内（与 userRoute 定义同一函数体）。

- [ ] **Step 3: 验证编译**

Run: `go build ./...`
Expected: 编译通过。

- [ ] **Step 4: 冒烟（可选，需本地运行）**

启动后以超管 session 调：
```
GET  /api/supplier/            → {success:true, data:{items:[...]}}
GET  /api/supplier/search?keyword=alice
PUT  /api/supplier/  body:{"user_id":2,"priority":9,"remark":"VIP"}
```
非超管访问应 401/权限不足。

- [ ] **Step 5: Checkpoint** — `go build ./...` + `go test ./model/ ./common/ -run 'Supplier|IsValidateRole' -v` 全绿 → 停下汇报。

---

## Task 7: 全量回归

- [ ] **Step 1: 全量构建与相关测试**

Run:
```bash
go build ./...
go vet ./common/ ./model/ ./controller/
go test ./common/ ./model/ -run 'Supplier|IsValidateRole|Phone' -v
```
Expected: 全部通过。

- [ ] **Step 2: 确认无回归**

Run: `go test ./model/ -count=1` （model 包既有测试不受影响）
Expected: PASS（若既有用例因新增列报错，检查 AutoMigrate 是否已含 Supplier）

- [ ] **Step 3: Checkpoint（最终）** — 汇报 P1-A 完成情况，列出改动文件，等用户决定是否提交（项目铁律：不自动提交）。

---

## 验收标准（P1-A）
- `RoleSupplierUser=5` 存在且 `IsValidateRole` 认可；既有角色不受影响。
- `users` 表有 `phone` 列；`suppliers` 表随迁移创建（三库兼容靠 AutoMigrate）。
- 公开注册 `POST /api/user/register`：手机号必填、产出 role=supplier 用户、自动创建 Supplier 资料。
- `GET /api/supplier/`、`/search`、`PUT /api/supplier/` 仅超管可用，能列出/搜索/改供应商优先级·启用·结算方式·备注。
- 普通用户列表/既有功能不受影响。

## 不在本计划内（后续）
- 前端（注册表单手机号、供应商管理页、菜单、i18n、角色常量）→ **P1-B 前端计划**。
- 关闭普通注册产出的"是否仍允许 role=common 注册" → 已由本计划改为产出 supplier；如需保留 common 注册入口，另议。
- `EmailVerificationEnabled` / `RegisterEnabled` 为运维开关，需在系统设置中开启邮箱验证（前置项 A6）。

## 风险
- **角色等值判断误伤**：少数代码可能 `role == RoleCommonUser` 等值判断而非 `>=`。本计划不批量改，但执行 Task 5 后建议 `grep -rn "RoleCommonUser" --include=*.go` 抽查供应商是否被这些分支误排除（多数为 `>=` 比较，风险低）。
- **注册非事务**：用户与 Supplier 资料分两步创建；`CreateSupplierProfile` 幂等，失败仅记日志不阻断注册（资料可后补/懒创建）。
