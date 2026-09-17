# 观察员角色 + 供应商端隐藏售价 + 角色下拉框 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** 新增只读"观察员"角色（role=3）供客户核对平台渠道与全站日志；堵住供应商日志接口泄露售价的路径；用户角色改为编辑弹窗下拉框，删除"提升/降级"。

**Architecture:** 后端沿用"角色常量 + 鉴权中间件 + 作用域路由组 + 复用管理员核心 + 输出前脱敏"的供应商范式；前端沿用 `ChannelsPage mode=` / `useLogsData` 的 mode 参数化范式，只改 `web/classic`。角色持久化补进 `User.Edit`，校验集中在 `UpdateUser`。

**Tech Stack:** Go 1.22 + Gin + GORM（测试用 glebarez/sqlite 内存库 + testify）；React 18 + Semi Design + Vite（Bun）。

**Spec:** `docs/superpowers/specs/2026-09-17-tokenki-viewer-role-supplier-price-hiding-design.md`

**提交纪律:** 用户已授权"自测通过后 commit"。整包一次 commit（不 push、不发版）。下方各 Task 末尾不单独 commit，最后 Task 11 统一提交。

---

## 文件结构

| 文件 | 职责 | 动作 |
|---|---|---|
| `common/constants.go` | `RoleViewerUser=3`、`IsValidateRole` | 改 |
| `common/role_test.go` | 角色合法性测试 | 新 |
| `middleware/auth.go` | `authHelperWithRoleCheck` 抽取、`ViewerAuth()` | 改 |
| `middleware/viewer_auth_test.go` | ViewerAuth 放行/拒绝矩阵 | 新 |
| `controller/channel.go` | `channelListOptions`、core 接受 options、viewer 映射 | 改 |
| `controller/viewer_channel.go` | `viewerChannelView` 白名单 DTO + `ViewerListChannels/ViewerSearchChannels` | 新 |
| `controller/viewer_channel_test.go` | 白名单序列化测试 | 新 |
| `controller/viewer_log.go` | `blankViewerLog` + `ViewerListLogs/ViewerLogsStat` | 新 |
| `controller/viewer_log_test.go` | 日志脱敏测试 | 新 |
| `controller/supplier_channel.go` | 调用改为 options | 改 |
| `controller/supplier_logs.go` | `blankSellingPrice`、stat 改 official_usd | 改 |
| `controller/supplier_logs_test.go` | 售价抹除测试 | 改 |
| `model/log.go` | `SumSupplierOfficialUsd` | 改 |
| `model/supplier_official_usd_test.go` | 汇总测试 | 新 |
| `model/user.go` | `UpdateUserRole` 单列更新 + 缓存失效（`Edit` 不动） | 改 |
| `controller/user.go` | `validateRoleChange`、删 promote/demote | 改 |
| `controller/user_role_test.go`、`model/user_role_test.go` | 角色变更规则表 / 单列更新 | 新 |
| `i18n/keys.go`、`i18n/locales/{en,zh-CN,zh-TW}.yaml` | 3 个新键 | 改 |
| `router/api-router.go` | `/api/viewer/*` 路由组 | 改 |
| `web/classic/src/helpers/utils.jsx` | `isViewer()` | 改 |
| `web/classic/src/helpers/auth.jsx` | `ViewerRoute` | 改 |
| `web/classic/src/hooks/channels/useChannelsData.jsx` | `isViewerMode`、apiBase、groups url、handleRow | 改 |
| `web/classic/src/components/table/channels/{index,ChannelsTable,ChannelsColumnDefs,ChannelsActions,ChannelsFilters}.jsx` | viewer 门控 | 改 |
| `web/classic/src/pages/ViewerChannels/index.jsx`、`pages/ViewerLogs/index.jsx` | 页面 | 新 |
| `web/classic/src/hooks/usage-logs/useUsageLogsData.jsx` | `useLogsData({mode})`、viewer URL、supplier stat | 改 |
| `web/classic/src/components/table/usage-logs/{index,UsageLogsTable,UsageLogsColumnDefs,UsageLogsFilters,UsageLogsActions}.jsx` | viewer/supplier 门控 | 改 |
| `web/classic/src/components/layout/SiderBar.jsx` | `viewerItems` 分区 | 改 |
| `web/classic/src/App.jsx` | 两条 viewer 路由 | 改 |
| `web/classic/src/components/table/users/{UsersColumnDefs,UsersTable}.jsx` | 删提升/降级 | 改 |
| `web/classic/src/components/table/users/modals/{PromoteUserModal,DemoteUserModal}.jsx` | 删除 | 删 |
| `web/classic/src/components/table/users/modals/EditUserModal.jsx` | 角色下拉 | 改 |
| `web/classic/src/i18n/locales/{zh-CN,en}.json` | 新文案 | 改 |
| `docs/superpowers/WORKLOG.md` | 工作日志 | 改 |

---

### Task 1: 角色常量

**Files:**
- Modify: `common/constants.go:197-208`
- Test: `common/role_test.go`

- [ ] **Step 1: 写失败测试**

```go
package common

import "testing"

// TestIsValidateRole_Viewer 观察员(3)必须是合法角色，否则 authHelper 会拒绝其会话。
func TestIsValidateRole_Viewer(t *testing.T) {
	if !IsValidateRole(RoleViewerUser) {
		t.Fatalf("RoleViewerUser must be valid")
	}
	if RoleViewerUser <= RoleCommonUser || RoleViewerUser >= RoleSupplierUser {
		t.Fatalf("RoleViewerUser must sit between common(1) and supplier(5), got %d", RoleViewerUser)
	}
	for _, bad := range []int{2, 4, 6, 9, 11, 99, 101, -1} {
		if IsValidateRole(bad) {
			t.Fatalf("role %d must be invalid", bad)
		}
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./common/ -run TestIsValidateRole_Viewer`
Expected: 编译错误 `undefined: RoleViewerUser`

- [ ] **Step 3: 实现**

```go
const (
	RoleGuestUser    = 0
	RoleCommonUser   = 1
	RoleViewerUser   = 3 // 观察员：只读看全站渠道/日志，其余同普通用户
	RoleSupplierUser = 5
	RoleAdminUser    = 10
	RoleRootUser     = 100
)

func IsValidateRole(role int) bool {
	return role == RoleGuestUser || role == RoleCommonUser || role == RoleViewerUser ||
		role == RoleSupplierUser || role == RoleAdminUser || role == RoleRootUser
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./common/ -run TestIsValidateRole_Viewer`
Expected: PASS

---

### Task 2: ViewerAuth 中间件

**Files:**
- Modify: `middleware/auth.go:36-40,153-160,192-214`
- Test: `middleware/viewer_auth_test.go`

- [ ] **Step 1: 写失败测试**

```go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// performViewerAuth 以会话方式登录 role 用户并请求受 ViewerAuth 保护的路由。
// model.DB 置 nil 走 authHelper 的"数据层未就绪→信任会话值"降级分支，避免建库。
func performViewerAuth(t *testing.T, role int) (*httptest.ResponseRecorder, *bool) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	prevDB := model.DB
	model.DB = nil
	t.Cleanup(func() { model.DB = prevDB })

	reached := false
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("viewer-auth-test"))))
	r.GET("/login", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("id", 7)
		s.Set("username", "u7")
		s.Set("role", role)
		s.Set("status", common.UserStatusEnabled)
		s.Set("group", "default")
		require.NoError(t, s.Save())
		c.Status(http.StatusOK)
	})
	r.GET("/v", ViewerAuth(), func(c *gin.Context) {
		reached = true
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	lw := httptest.NewRecorder()
	r.ServeHTTP(lw, httptest.NewRequest(http.MethodGet, "/login", nil))
	cookies := lw.Result().Cookies()

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v", nil)
	req.Header.Set("New-Api-User", "7")
	for _, ck := range cookies {
		req.AddCookie(ck)
	}
	r.ServeHTTP(w, req)
	return w, &reached
}

func TestViewerAuth_AllowsViewerAndAdmins(t *testing.T) {
	for _, role := range []int{common.RoleViewerUser, common.RoleAdminUser, common.RoleRootUser} {
		w, reached := performViewerAuth(t, role)
		require.Equal(t, http.StatusOK, w.Code, "role %d", role)
		require.True(t, *reached, "role %d must reach handler", role)
	}
}

func TestViewerAuth_RejectsCommonAndSupplier(t *testing.T) {
	for _, role := range []int{common.RoleCommonUser, common.RoleSupplierUser} {
		_, reached := performViewerAuth(t, role)
		require.False(t, *reached, "role %d must be rejected", role)
	}
}

// TestUserAuth_StillLinear 抽取谓词后原 minRole 语义不变：普通用户可过 UserAuth。
func TestUserAuth_StillLinear(t *testing.T) {
	gin.SetMode(gin.TestMode)
	prevDB := model.DB
	model.DB = nil
	t.Cleanup(func() { model.DB = prevDB })
	reached := false
	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("x"))))
	r.GET("/login", func(c *gin.Context) {
		s := sessions.Default(c)
		s.Set("id", 8)
		s.Set("username", "u8")
		s.Set("role", common.RoleCommonUser)
		s.Set("status", common.UserStatusEnabled)
		s.Set("group", "default")
		require.NoError(t, s.Save())
	})
	r.GET("/u", UserAuth(), func(c *gin.Context) { reached = true })
	lw := httptest.NewRecorder()
	r.ServeHTTP(lw, httptest.NewRequest(http.MethodGet, "/login", nil))
	req := httptest.NewRequest(http.MethodGet, "/u", nil)
	req.Header.Set("New-Api-User", "8")
	for _, ck := range lw.Result().Cookies() {
		req.AddCookie(ck)
	}
	r.ServeHTTP(httptest.NewRecorder(), req)
	require.True(t, reached)
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./middleware/ -run 'TestViewerAuth|TestUserAuth_StillLinear'`
Expected: `undefined: ViewerAuth`

- [ ] **Step 3: 实现**

在 `middleware/auth.go`：把 `func authHelper(c *gin.Context, minRole int) {` 改为

```go
// authHelper 线性角色门槛：role >= minRole 放行（UserAuth/SupplierAuth/AdminAuth/RootAuth 沿用）。
func authHelper(c *gin.Context, minRole int) {
	authHelperWithRoleCheck(c, func(role int) bool { return role >= minRole })
}

// authHelperWithRoleCheck 通用鉴权：会话/access_token 解析、New-Api-User 校验、
// 缓存回查、封禁检查后，用 allow 谓词决定角色是否放行。
func authHelperWithRoleCheck(c *gin.Context, allow func(role int) bool) {
```

函数体内原 `if role.(int) < minRole {` 改为 `if !allow(role.(int)) {`。

在 `SupplierAuth` 之后加：

```go
// ViewerAuth 观察员专用：只放行 role==RoleViewerUser 或管理员以上。
// 不能用线性 minRole=3，否则供应商(5)也会通过。
func ViewerAuth() func(c *gin.Context) {
	return func(c *gin.Context) {
		authHelperWithRoleCheck(c, func(role int) bool {
			return role == common.RoleViewerUser || role >= common.RoleAdminUser
		})
	}
}
```

- [ ] **Step 4: 运行确认通过**

Run: `go test ./middleware/`
Expected: PASS（含既有 header_nav / require_twofa 测试）

---

### Task 3: 观察员渠道接口（白名单 DTO）

**Files:**
- Modify: `controller/channel.go:129-135,331-336,226-232,468-474`（core 签名与输出）
- Modify: `controller/supplier_channel.go:34-41`
- Create: `controller/viewer_channel.go`
- Test: `controller/viewer_channel_test.go`
- Modify: `router/api-router.go`（Task 4 一并加路由组）

- [ ] **Step 1: 写失败测试**

```go
package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func strp(s string) *string { return &s }
func f64p(f float64) *float64 { return &f }
func int64p(i int64) *int64 { return &i }
func uintp(u uint) *uint { return &u }

func fullySensitiveChannel() *model.Channel {
	return &model.Channel{
		Id: 42, Type: 14, Key: "sk-secret", Status: 1, Name: "claude-main",
		Weight: uintp(7), Priority: int64p(9), CreatedTime: 1700000000, TestTime: 1700000100,
		ResponseTime: 350, BaseURL: strp("https://res.services.ai.azure.com/anthropic"),
		Other: "azure-other", Balance: 123.45, BalanceUpdatedTime: 1700000200,
		SupplierId: 5, SupplierName: "sup-a", CreatedBy: 5, CreatedByName: "sup-a",
		CostPrice: f64p(6.8), Models: "claude-opus-5,claude-sonnet-5", Group: "default,azure-claude",
		UsedQuota: 500000, ModelMapping: strp(`{"a":"b"}`), StatusCodeMapping: strp(`{"429":"503"}`),
		OtherInfo: `{"status_reason":"key leaked sk-xxx"}`, Tag: strp("t1"),
		Setting: strp(`{"proxy":"socks5://x"}`), ParamOverride: strp(`{"temperature":0}`),
		HeaderOverride: strp(`{"Authorization":"Bearer leak"}`), Remark: strp("内部备注"),
		ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: 3, MultiKeyStatusList: map[int]int{0: 1, 1: 2}, MultiKeyPollingIndex: 1},
		OtherSettings: `{"azure_version":"2024-02-01"}`, OfficialUsd: 12.3, Receivable: 83.6,
	}
}

// TestViewerChannelView_Whitelist 白名单之外的任何键都不得出现在观察员响应里。
func TestViewerChannelView_Whitelist(t *testing.T) {
	view := viewerChannelView(fullySensitiveChannel())
	data, err := common.Marshal(view)
	require.NoError(t, err)
	var got map[string]any
	require.NoError(t, common.Unmarshal(data, &got))

	allowed := map[string]bool{
		"id": true, "name": true, "type": true, "status": true, "models": true, "group": true,
		"response_time": true, "test_time": true, "created_time": true,
		"used_quota": true, "balance": true, "balance_updated_time": true, "channel_info": true,
	}
	for k := range got {
		require.True(t, allowed[k], "unexpected key %q leaked to viewer", k)
	}
	for k := range allowed {
		_, ok := got[k]
		require.True(t, ok, "whitelisted key %q missing", k)
	}
	require.EqualValues(t, 42, got["id"])
	require.Equal(t, "claude-main", got["name"])
	require.EqualValues(t, 123.45, got["balance"])
	require.EqualValues(t, 500000, got["used_quota"])

	ci, ok := got["channel_info"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, true, ci["is_multi_key"])
	require.EqualValues(t, 3, ci["multi_key_size"])
	require.Len(t, ci, 2, "channel_info must only expose is_multi_key and multi_key_size")

	s := string(data)
	for _, leak := range []string{"sk-secret", "azure.com", "6.8", "Bearer leak", "内部备注", "socks5", "sup-a", "status_reason", "83.6", "12.3"} {
		require.NotContains(t, s, leak)
	}
}

// TestViewerChannelViews_TagModeParentKeepsTagAndChildren 标签模式父行保留 tag，children 同样脱敏。
func TestViewerChannelViews_TagModeParentKeepsTagAndChildren(t *testing.T) {
	child := fullySensitiveChannel()
	parent := &model.Channel{Id: 0, Name: "t1", Tag: strp("t1"), Children: []*model.Channel{child}}
	views := viewerChannelViews([]*model.Channel{parent})
	require.Len(t, views, 1)
	require.Equal(t, "t1", views[0].Tag)
	require.Len(t, views[0].Children, 1)
	data, err := common.Marshal(views[0].Children[0])
	require.NoError(t, err)
	require.NotContains(t, string(data), "sk-secret")
	require.NotContains(t, string(data), "6.8")
}
```

> 若 `model.Channel` 没有 `Children` 字段（当前 tag 模式在前端聚合），则删除第二个测试中 `Children` 相关断言，改为只断言 `views[0].Tag == "t1"`，并在 `viewerChannelView` 中不处理 children。写测试前先 `grep -n "Children" model/channel.go` 确认。

- [ ] **Step 2: 运行确认失败**

Run: `go test ./controller/ -run TestViewerChannel`
Expected: `undefined: viewerChannelView`

- [ ] **Step 3: 实现 `controller/viewer_channel.go`**

```go
package controller

import (
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// viewerChannelInfo 观察员可见的多 Key 摘要：只暴露是否多 Key 与数量。
type viewerChannelInfo struct {
	IsMultiKey   bool `json:"is_multi_key"`
	MultiKeySize int  `json:"multi_key_size"`
}

// viewerChannel 观察员渠道白名单 DTO。字段以外的任何信息（key/base_url/成本价/覆写/备注/权重/供应商）
// 一律不进入此结构体，因此不可能被序列化出去。新增字段前先对照设计文档 §1.1。
type viewerChannel struct {
	Id                 int               `json:"id"`
	Name               string            `json:"name"`
	Type               int               `json:"type"`
	Status             int               `json:"status"`
	Models             string            `json:"models"`
	Group              string            `json:"group"`
	ResponseTime       int               `json:"response_time"`
	TestTime           int64             `json:"test_time"`
	CreatedTime        int64             `json:"created_time"`
	UsedQuota          int64             `json:"used_quota"`
	Balance            float64           `json:"balance"`
	BalanceUpdatedTime int64             `json:"balance_updated_time"`
	ChannelInfo        viewerChannelInfo `json:"channel_info"`
	// Tag 仅标签聚合模式的父行需要（前端按 tag 分组渲染），普通行序列化时为空字符串会被 omitempty 省略。
	Tag string `json:"tag,omitempty"`
}

func viewerChannelView(ch *model.Channel) *viewerChannel {
	if ch == nil {
		return nil
	}
	v := &viewerChannel{
		Id:                 ch.Id,
		Name:               ch.Name,
		Type:               ch.Type,
		Status:             ch.Status,
		Models:             ch.Models,
		Group:              ch.Group,
		ResponseTime:       ch.ResponseTime,
		TestTime:           ch.TestTime,
		CreatedTime:        ch.CreatedTime,
		UsedQuota:          ch.UsedQuota,
		Balance:            ch.Balance,
		BalanceUpdatedTime: ch.BalanceUpdatedTime,
		ChannelInfo: viewerChannelInfo{
			IsMultiKey:   ch.ChannelInfo.IsMultiKey,
			MultiKeySize: ch.ChannelInfo.MultiKeySize,
		},
	}
	if ch.Tag != nil {
		v.Tag = *ch.Tag
	}
	return v
}

func viewerChannelViews(channels []*model.Channel) []*viewerChannel {
	out := make([]*viewerChannel, 0, len(channels))
	for _, ch := range channels {
		if ch == nil {
			continue
		}
		out = append(out, viewerChannelView(ch))
	}
	return out
}

// ViewerListChannels 观察员渠道列表：复用管理员核心，输出白名单 DTO。
func ViewerListChannels(c *gin.Context) {
	listChannelsCore(c, channelListOptions{viewer: true})
}

// ViewerSearchChannels 观察员渠道搜索：复用管理员核心，输出白名单 DTO。
func ViewerSearchChannels(c *gin.Context) {
	searchChannelsCore(c, channelListOptions{viewer: true})
}
```

> 如果 Task 3 Step 1 确认 `model.Channel` 有 `Children` 字段，给 `viewerChannel` 加 `Children []*viewerChannel \`json:"children,omitempty"\`` 并在 `viewerChannelView` 里递归映射 `ch.Children`。

- [ ] **Step 4: 改 `controller/channel.go` core 签名**

```go
// channelListOptions 渠道列表/搜索核心的作用域选项。
//   - forceSupplierId>0：供应商端，强制只看本人渠道，回填成本/应收款。
//   - viewer：观察员端，忽略 supplier_name 过滤，不回填供应商名/应收款，输出白名单 DTO。
type channelListOptions struct {
	forceSupplierId int
	viewer          bool
}
```

- `func GetAllChannels(c)` → `listChannelsCore(c, channelListOptions{})`
- `func SearchChannels(c)` → `searchChannelsCore(c, channelListOptions{})`
- `func listChannelsCore(c *gin.Context, forceSupplierId int)` → `func listChannelsCore(c *gin.Context, opts channelListOptions)`，函数体内 `forceSupplierId` 全部替换为 `opts.forceSupplierId`。
- `supplier_name` 分支：`} else if supplierName := ...; supplierName != "" {` 前加条件 `!opts.viewer &&`：

```go
	} else if supplierName := strings.TrimSpace(c.Query("supplier_name")); !opts.viewer && supplierName != "" {
```

- 输出段：

```go
	for _, datum := range channelData {
		clearChannelInfo(datum)
	}
	var items any = channelData
	if opts.viewer {
		items = viewerChannelViews(channelData)
	} else {
		backfillChannelSupplierNames(channelData)
		if opts.forceSupplierId > 0 {
			backfillSupplierUnsettled(channelData)
		}
	}
	// ...typeCounts 不变...
	common.ApiSuccess(c, gin.H{
		"items":       items,
		"total":       total,
		"page":        pageInfo.GetPage(),
		"page_size":   pageInfo.GetPageSize(),
		"type_counts": typeCounts,
	})
```

- `searchChannelsCore` 同样改签名、同样的 `!opts.viewer &&`、输出段：

```go
	for _, datum := range pagedData {
		clearChannelInfo(datum)
	}
	var items any = pagedData
	if opts.viewer {
		items = viewerChannelViews(pagedData)
	} else {
		backfillChannelSupplierNames(pagedData)
		if opts.forceSupplierId > 0 {
			backfillSupplierUnsettled(pagedData)
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"items":       items,
			"total":       total,
			"type_counts": typeCounts,
		},
	})
```

- `controller/supplier_channel.go`：

```go
func SupplierListChannels(c *gin.Context) {
	listChannelsCore(c, channelListOptions{forceSupplierId: c.GetInt("id")})
}
func SupplierSearchChannels(c *gin.Context) {
	searchChannelsCore(c, channelListOptions{forceSupplierId: c.GetInt("id")})
}
```

- [ ] **Step 5: 运行确认通过**

Run: `go build ./... && go test ./controller/ -run 'TestViewerChannel|TestListChannels|TestSearchChannels|Channel'`
Expected: 编译通过，新测试 PASS，既有渠道测试不回归。

---

### Task 4: 观察员日志接口 + 路由组

**Files:**
- Create: `controller/viewer_log.go`
- Test: `controller/viewer_log_test.go`
- Modify: `router/api-router.go`（在 `supplierSelfRoute` 之后新增）

- [ ] **Step 1: 写失败测试**

```go
package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

// TestBlankViewerLog 观察员看全站日志：隐藏消费者身份与平台成本，保留渠道/模型/花费。
func TestBlankViewerLog(t *testing.T) {
	other := common.MapToJsonStr(map[string]any{
		"model_ratio": 2.5, "group_ratio": 3.3, "admin_info": map[string]any{"use_channel": []string{"1"}},
		"stream_status": "ok", "frt": 120,
	})
	logs := []*model.Log{
		{Id: 1, UserId: 9, Username: "alice", TokenName: "tok", TokenId: 3, Ip: "1.2.3.4",
			ModelName: "claude-opus-5", ChannelId: 42, ChannelName: "claude-main", Quota: 1234,
			OfficialUsd: 0.5, CostPriceSnapshot: 6.8, Other: other},
		nil,
	}
	blankViewerLog(logs)
	l := logs[0]
	require.Equal(t, "", l.Username)
	require.Equal(t, "", l.TokenName)
	require.Equal(t, "", l.Ip)
	require.Equal(t, 0, l.UserId)
	require.Equal(t, 0, l.TokenId)
	require.Equal(t, float64(0), l.OfficialUsd)
	require.Equal(t, float64(0), l.CostPriceSnapshot)
	require.Equal(t, "claude-main", l.ChannelName)
	require.Equal(t, 42, l.ChannelId)
	require.Equal(t, 1234, l.Quota)
	m, _ := common.StrToMap(l.Other)
	_, hasAdmin := m["admin_info"]
	_, hasStream := m["stream_status"]
	require.False(t, hasAdmin)
	require.False(t, hasStream)
	require.EqualValues(t, 3.3, m["group_ratio"], "group ratio is public pricing for a customer, keep it")
	require.EqualValues(t, 120, m["frt"])
	require.NotPanics(t, func() { blankViewerLog(nil) })
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./controller/ -run TestBlankViewerLog`
Expected: `undefined: blankViewerLog`

- [ ] **Step 3: 实现 `controller/viewer_log.go`**

```go
package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// blankViewerLog 观察员全站日志脱敏：抹掉消费者身份（用户名/令牌名/IP/ID）与平台成本
// （官方价/冻结成本价/admin_info），保留渠道名、模型、tokens、花费（售价对客户不敏感）。
func blankViewerLog(logs []*model.Log) {
	for _, l := range logs {
		if l == nil {
			continue
		}
		l.Username = ""
		l.TokenName = ""
		l.Ip = ""
		l.UserId = 0
		l.TokenId = 0
		l.OfficialUsd = 0
		l.CostPriceSnapshot = 0
		if l.Other == "" {
			continue
		}
		otherMap, _ := common.StrToMap(l.Other)
		if otherMap == nil {
			continue
		}
		delete(otherMap, "admin_info")
		delete(otherMap, "stream_status")
		l.Other = common.MapToJsonStr(otherMap)
	}
}

// ViewerListLogs 观察员全站日志：复用管理员查询，禁止按用户名/令牌名筛选，输出前脱敏。
func ViewerListLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	logs, total, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, "", "", pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, group, requestId, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	blankViewerLog(logs)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

// ViewerLogsStat 观察员全站消耗统计（quota 为售价口径，对客户不敏感）。
func ViewerLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	stat, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, "", "", channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
}
```

- [ ] **Step 4: 路由组**（`router/api-router.go`，紧跟 `supplierSelfRoute` 块之后）

```go
		// 观察员（role=3）只读作用域：全站渠道白名单视图 + 全站脱敏日志。ViewerAuth 精确匹配角色，
		// 供应商(5)不可进入；管理员以上放行便于排查。不开放任何写/测试/刷余额接口。
		viewerRoute := apiRouter.Group("/viewer")
		viewerRoute.Use(middleware.ViewerAuth())
		{
			viewerRoute.GET("/channel/", controller.ViewerListChannels)
			viewerRoute.GET("/channel/search", controller.ViewerSearchChannels)
			viewerRoute.GET("/channel/models", controller.EnabledListModels)
			viewerRoute.GET("/groups", controller.GetGroups)
			viewerRoute.GET("/log/", controller.ViewerListLogs)
			viewerRoute.GET("/log/stat", controller.ViewerLogsStat)
		}
```

- [ ] **Step 5: 运行确认通过**

Run: `go build ./... && go test ./controller/ -run 'TestBlankViewerLog' && go vet ./router/ ./controller/ ./middleware/`
Expected: PASS，无 vet 告警。

---

### Task 5: 供应商端隐藏售价

**Files:**
- Modify: `controller/supplier_logs.go:15-56,58-84`
- Modify: `controller/supplier_logs_test.go`
- Modify: `model/log.go:615`（新增 `SumSupplierOfficialUsd`）
- Test: `model/supplier_official_usd_test.go`

- [ ] **Step 1: 写失败测试（controller）**，追加到 `controller/supplier_logs_test.go`：

```go
// TestBlankSellingPrice 供应商不得看到平台售价：quota 归零、other 中的分组倍率等加价字段剔除，官方价参数保留。
func TestBlankSellingPrice(t *testing.T) {
	other := common.MapToJsonStr(map[string]any{
		"model_ratio": 2.5, "completion_ratio": 5, "model_price": 0, "cache_ratio": 0.1,
		"group_ratio": 3.3, "user_group_ratio": 2.0, "billing_mode": "tiered_expr", "matched_tier": "t2",
		"billing_preference": "wallet", "billing_source": "subscription", "wallet_quota_deducted": 100,
		"subscription_id": 1, "subscription_plan_title": "Pro", "admin_info": map[string]any{"x": 1},
		"stream_status": "ok", "frt": 88, "upstream_model_name": "claude-opus-5", "is_model_mapped": true,
	})
	logs := []*model.Log{{Quota: 3300, OfficialUsd: 0.1, Other: other}, nil}
	blankSellingPrice(logs)
	require.Equal(t, 0, logs[0].Quota)
	require.Equal(t, 0.1, logs[0].OfficialUsd, "official price is the supplier's settlement basis, keep it")
	m, _ := common.StrToMap(logs[0].Other)
	for _, gone := range []string{"group_ratio", "user_group_ratio", "billing_mode", "matched_tier", "billing_preference",
		"billing_source", "wallet_quota_deducted", "subscription_id", "subscription_plan_title", "admin_info", "stream_status"} {
		_, ok := m[gone]
		require.False(t, ok, "%s must be removed", gone)
	}
	for _, kept := range []string{"model_ratio", "completion_ratio", "model_price", "cache_ratio", "frt", "upstream_model_name", "is_model_mapped"} {
		_, ok := m[kept]
		require.True(t, ok, "%s must be kept", kept)
	}
	require.NotPanics(t, func() { blankSellingPrice(nil) })
}
```

补 import：`"github.com/QuantumNous/new-api/common"`。

- [ ] **Step 2: 写失败测试（model）** `model/supplier_official_usd_test.go`：

```go
package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupOfficialUsdDB(t *testing.T) {
	t.Helper()
	common.UsingSQLite = true
	common.RedisEnabled = false
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	DB = db
	LOG_DB = db
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

// TestSumSupplierOfficialUsd 只汇总本人渠道、消费类型、时间窗内的 official_usd，与 quota 无关。
func TestSumSupplierOfficialUsd(t *testing.T) {
	setupOfficialUsdDB(t)
	rows := []Log{
		{Type: LogTypeConsume, ChannelId: 1, CreatedAt: 100, Quota: 99999, OfficialUsd: 1.5},
		{Type: LogTypeConsume, ChannelId: 1, CreatedAt: 200, Quota: 99999, OfficialUsd: 2.25},
		{Type: LogTypeConsume, ChannelId: 2, CreatedAt: 150, Quota: 99999, OfficialUsd: 100}, // 他人渠道
		{Type: LogTypeError, ChannelId: 1, CreatedAt: 150, Quota: 0, OfficialUsd: 50},         // 非消费
		{Type: LogTypeConsume, ChannelId: 1, CreatedAt: 999, Quota: 1, OfficialUsd: 7},        // 窗外
	}
	require.NoError(t, LOG_DB.Create(&rows).Error)

	got, err := SumSupplierOfficialUsd([]int{1}, 100, 300)
	require.NoError(t, err)
	require.InDelta(t, 3.75, got, 1e-9)

	all, err := SumSupplierOfficialUsd([]int{1}, 0, 0)
	require.NoError(t, err)
	require.InDelta(t, 10.75, all, 1e-9)

	none, err := SumSupplierOfficialUsd(nil, 0, 0)
	require.NoError(t, err)
	require.Equal(t, float64(0), none)
}
```

- [ ] **Step 3: 运行确认失败**

Run: `go test ./controller/ -run TestBlankSellingPrice; go test ./model/ -run TestSumSupplierOfficialUsd`
Expected: 两处 `undefined`

- [ ] **Step 4: 实现 model**（`model/log.go`，紧跟 `SumSupplierStat` 之后）

```go
// SumSupplierOfficialUsd 供应商日志页顶部统计：时间窗内本人渠道消费日志的官方价合计（USD）。
// 供应商结算按 official_usd 口径，因此只暴露该值；含分组倍率的 quota（平台售价）不再返回给供应商。
func SumSupplierOfficialUsd(channelIds []int, startTimestamp int64, endTimestamp int64) (float64, error) {
	if len(channelIds) == 0 {
		return 0, nil
	}
	q := LOG_DB.Table("logs").
		Select("COALESCE(SUM(official_usd), 0)").
		Where("type = ? AND channel_id IN ?", LogTypeConsume, channelIds)
	if startTimestamp != 0 {
		q = q.Where("created_at >= ?", startTimestamp)
	}
	if endTimestamp != 0 {
		q = q.Where("created_at <= ?", endTimestamp)
	}
	var total float64
	if err := q.Row().Scan(&total); err != nil {
		return 0, err
	}
	return total, nil
}
```

- [ ] **Step 5: 实现 controller**（`controller/supplier_logs.go`）

在 `blankConsumerIdentity(logs)` 之后调用 `blankSellingPrice(logs)`；`SupplierLogsStat` 改为：

```go
	stat, err := model.SumSupplierStat(channelIds, startTs, endTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	officialUsd, err := model.SumSupplierOfficialUsd(channelIds, startTs, endTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 只返回官方价口径；quota（含分组倍率的平台售价）不再暴露给供应商。
	common.ApiSuccess(c, gin.H{
		"official_usd": officialUsd,
		"rpm":          stat.Rpm,
		"tpm":          stat.Tpm,
	})
```

新增：

```go
// sellingPriceOtherKeys 日志 other 里会暴露平台加价/计费策略的键，供应商端一律剔除。
// 保留 model_ratio / completion_ratio / model_price / cache_* 等官方价参数与请求元数据。
var sellingPriceOtherKeys = []string{
	"group_ratio", "user_group_ratio",
	"billing_mode", "matched_tier", "billing_preference", "billing_source", "wallet_quota_deducted",
	"admin_info", "stream_status",
}

// blankSellingPrice 抹掉供应商日志中的平台售价：quota 归零，other 剔除加价/订阅/管理字段。
// official_usd 与 cost_price_snapshot 是供应商结算依据，保留。
func blankSellingPrice(logs []*model.Log) {
	for _, l := range logs {
		if l == nil {
			continue
		}
		l.Quota = 0
		if l.Other == "" {
			continue
		}
		otherMap, _ := common.StrToMap(l.Other)
		if otherMap == nil {
			continue
		}
		for _, k := range sellingPriceOtherKeys {
			delete(otherMap, k)
		}
		for k := range otherMap {
			if strings.HasPrefix(k, "subscription_") {
				delete(otherMap, k)
			}
		}
		l.Other = common.MapToJsonStr(otherMap)
	}
}
```

补 import `"strings"`。

- [ ] **Step 6: 运行确认通过**

Run: `go test ./controller/ -run 'TestBlankSellingPrice|TestBlankConsumerIdentity' && go test ./model/ -run 'TestSumSupplierOfficialUsd|TestSumSupplierStat'`
Expected: PASS

---

### Task 6: 角色变更规则 + 删除 promote/demote

**Files:**
- Modify: `model/user.go:570-602`（`Edit` 落 role）
- Modify: `controller/user.go:630-672`（`UpdateUser`）、`980-999`（删两个 case）
- Modify: `i18n/keys.go:95-98`、`i18n/locales/en.yaml`、`zh-CN.yaml`、`zh-TW.yaml`
- Test: `controller/user_role_test.go`

- [ ] **Step 1: 写失败测试**

```go
package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/stretchr/testify/require"
)

// TestValidateRoleChange 角色变更规则表（设计文档 §3.1）。
func TestValidateRoleChange(t *testing.T) {
	root, admin := common.RoleRootUser, common.RoleAdminUser
	cases := []struct {
		name                    string
		myRole, myId            int
		targetId, originRole    int
		newRole                 int
		wantRole                int
		wantErr                 string
	}{
		{"缺省不改(0)保留原角色", root, 1, 2, common.RoleSupplierUser, 0, common.RoleSupplierUser, ""},
		{"角色不变直接放行", admin, 1, 2, common.RoleCommonUser, common.RoleCommonUser, common.RoleCommonUser, ""},
		{"root 设观察员", root, 1, 2, common.RoleSupplierUser, common.RoleViewerUser, common.RoleViewerUser, ""},
		{"root 设管理员", root, 1, 2, common.RoleCommonUser, admin, admin, ""},
		{"admin 设供应商", admin, 1, 2, common.RoleCommonUser, common.RoleSupplierUser, common.RoleSupplierUser, ""},
		{"admin 不能设管理员", admin, 1, 2, common.RoleCommonUser, admin, 0, i18n.MsgUserCannotCreateHigherLevel},
		{"任何人不能设 root", root, 1, 2, common.RoleCommonUser, root, 0, i18n.MsgUserRoleInvalid},
		{"非法值 4", root, 1, 2, common.RoleCommonUser, 4, 0, i18n.MsgUserRoleInvalid},
		{"游客 0 之外的非法负数", root, 1, 2, common.RoleCommonUser, -1, 0, i18n.MsgUserRoleInvalid},
		{"不能改 root 的角色", root, 1, 2, root, admin, 0, i18n.MsgUserCannotChangeRootRole},
		{"不能改自己的角色", root, 1, 1, root, admin, 0, i18n.MsgUserCannotChangeOwnRole},
		{"admin 改自己也拒", admin, 5, 5, admin, common.RoleCommonUser, 0, i18n.MsgUserCannotChangeOwnRole},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := validateRoleChange(tc.myRole, tc.myId, tc.targetId, tc.originRole, tc.newRole)
			if tc.wantErr == "" {
				require.Nil(t, err)
				require.Equal(t, tc.wantRole, got)
				return
			}
			require.NotNil(t, err)
			require.Equal(t, tc.wantErr, err.key)
		})
	}
}
```

- [ ] **Step 2: 运行确认失败**

Run: `go test ./controller/ -run TestValidateRoleChange`
Expected: `undefined: validateRoleChange` / `i18n.MsgUserRoleInvalid`

- [ ] **Step 3: i18n 键**

`i18n/keys.go`（`MsgUserAdminCannotPromote` 后）：

```go
	MsgUserRoleInvalid               = "user.role_invalid"
	MsgUserCannotChangeRootRole      = "user.cannot_change_root_role"
	MsgUserCannotChangeOwnRole       = "user.cannot_change_own_role"
```

`i18n/locales/en.yaml`（`user.admin_cannot_promote` 后）：

```yaml
user.role_invalid: "Invalid role: only Regular User, Viewer, Supplier or Admin can be assigned"
user.cannot_change_root_role: "Cannot change the role of a super administrator"
user.cannot_change_own_role: "Cannot change your own role"
```

`zh-CN.yaml`：

```yaml
user.role_invalid: "角色不合法：只能设置为普通用户、观察员、供应商或管理员"
user.cannot_change_root_role: "无法修改超级管理员的角色"
user.cannot_change_own_role: "无法修改自己的角色"
```

`zh-TW.yaml`：

```yaml
user.role_invalid: "角色不合法：只能設定為普通使用者、觀察員、供應商或管理員"
user.cannot_change_root_role: "無法修改超級管理員的角色"
user.cannot_change_own_role: "無法修改自己的角色"
```

- [ ] **Step 4: 实现 `validateRoleChange`**（`controller/user.go`，放在 `canManageTargetRole` 之后）

```go
// roleChangeError 角色变更校验错误，key 为 i18n 消息键。
type roleChangeError struct{ key string }

// assignableRoles 编辑弹窗可分配的角色集合：不含游客(0)与超管(100)。
var assignableRoles = map[int]bool{
	common.RoleCommonUser:   true,
	common.RoleViewerUser:   true,
	common.RoleSupplierUser: true,
	common.RoleAdminUser:    true,
}

// validateRoleChange 校验管理员/超管通过编辑接口修改目标用户角色：
//   - newRole==0 视为"未传/不改"，返回 originRole；
//   - 角色不变直接放行；
//   - newRole 必须在可分配集合内（禁止设为超管）；
//   - 不能改超管的角色；不能改自己的角色；
//   - 操作者必须能管理新角色（admin 不能设 admin，root 可以）。
// 返回最终应落库的角色。
func validateRoleChange(myRole, myId, targetId, originRole, newRole int) (int, *roleChangeError) {
	if newRole == 0 || newRole == originRole {
		return originRole, nil
	}
	if !assignableRoles[newRole] {
		return 0, &roleChangeError{key: i18n.MsgUserRoleInvalid}
	}
	if originRole == common.RoleRootUser {
		return 0, &roleChangeError{key: i18n.MsgUserCannotChangeRootRole}
	}
	if targetId == myId {
		return 0, &roleChangeError{key: i18n.MsgUserCannotChangeOwnRole}
	}
	if !canManageTargetRole(myRole, newRole) {
		return 0, &roleChangeError{key: i18n.MsgUserCannotCreateHigherLevel}
	}
	return newRole, nil
}
```

`UpdateUser` 中，把

```go
	if !canManageTargetRole(myRole, updatedUser.Role) {
		common.ApiErrorI18n(c, i18n.MsgUserCannotCreateHigherLevel)
		return
	}
```

替换为

```go
	finalRole, roleErr := validateRoleChange(myRole, c.GetInt("id"), originUser.Id, originUser.Role, updatedUser.Role)
	if roleErr != nil {
		common.ApiErrorI18n(c, roleErr.key)
		return
	}
	updatedUser.Role = finalRole
```

`ManageUser`：删除 `case "promote":` 与 `case "demote":` 两个分支（含其内部所有行）。运行 `go vet ./i18n/...` 确认 `MsgUserAlreadyAdmin`/`MsgUserAlreadyCommon`/`MsgUserAdminCannotPromote`/`MsgUserCannotDemoteRootUser` 未被其他地方引用后保留常量不动（避免无关 diff）。

- [ ] **Step 5: 角色单列更新 `UpdateUserRole`**（`model/user.go`，紧邻 `UpdateUserSetting`）

> 执行时发现既有安全测试 `TestEdit_DoesNotEscalateRoleStatusQuota` 锁定 `Edit` 不得写 role（GHSA-j6gc 防注入提权）。因此**不改 `Edit`**，改为专用单列更新 + 缓存失效，`UpdateUser` 在 `Edit` 成功后调用。

```go
// UpdateUserRole 管理员编辑接口专用：单列更新角色并失效用户缓存（会话下一请求回源 DB，即时生效）。
func UpdateUserRole(userId int, role int) error {
	if userId == 0 {
		return errors.New("id 为空！")
	}
	if !common.IsValidateRole(role) || role == common.RoleGuestUser || role == common.RoleRootUser {
		return errors.New("invalid role")
	}
	if err := DB.Model(&User{}).Where("id = ?", userId).Update("role", role).Error; err != nil {
		return err
	}
	return invalidateUserCache(userId)
}
```

`UpdateUser` 中 `Edit` 成功后：

```go
	if finalRole != originUser.Role {
		if err := model.UpdateUserRole(originUser.Id, finalRole); err != nil {
			common.ApiError(c, err)
			return
		}
	}
```

测试 `model/user_role_test.go::TestUpdateUserRole`：改后 role 变、status/quota 不变；0/100/4/-1 与 userId=0 均报错且不改库。

- [ ] **Step 6: 运行确认通过**

Run: `go build ./... && go test ./controller/ -run 'TestValidateRoleChange|Privesc' && go test ./model/ -run 'User'`
Expected: PASS；`user_privesc_test.go` 既有用例不回归。

---

### Task 7: 前端基础（角色判断、守卫、标签、文案）

**Files:**
- Modify: `web/classic/src/helpers/utils.jsx:50-58`
- Modify: `web/classic/src/helpers/auth.jsx:68-84`
- Modify: `web/classic/src/components/table/users/UsersColumnDefs.jsx:44-70`
- Modify: `web/classic/src/i18n/locales/zh-CN.json`、`en.json`

- [ ] **Step 1: `isViewer()`**（`utils.jsx`，`isSupplier` 之后）

```jsx
export function isViewer() {
  let user = localStorage.getItem('user');
  if (!user) return false;
  try {
    user = JSON.parse(user);
  } catch {
    return false;
  }
  return user && user.role === 3; // STRICT: exactly viewer
}
```

- [ ] **Step 2: `ViewerRoute`**（`auth.jsx`，`SupplierRoute` 之后，同样结构，判定 `user.role === 3`）

```jsx
export function ViewerRoute({ children }) {
  const raw = localStorage.getItem('user');
  if (!raw) {
    return <Navigate to='/login' state={{ from: history.location }} />;
  }
  try {
    const user = JSON.parse(raw);
    if (user && typeof user.role === 'number' && user.role === 3) {
      return children;
    }
  } catch (e) {
    // ignore
  }
  return <Navigate to='/forbidden' />;
}
```

> 先看 `SupplierRoute` 拒绝时跳去哪（`sed -n 80,84p helpers/auth.jsx`），与之一致。

- [ ] **Step 3: `renderRole` 加 `case 3`**

```jsx
    case 3:
      return (
        <Tag color='purple' shape='circle'>
          {t('观察员')}
        </Tag>
      );
```

- [ ] **Step 4: 文案**（两个 JSON 的 `translation` 对象里各加，zh-CN 值=键）

`en.json`：
```json
    "观察员": "Viewer",
    "资源总览": "Resource Overview",
    "全平台日志": "Platform Logs",
    "请选择角色": "Select role",
    "超级管理员的角色不可修改": "The super administrator's role cannot be changed",
    "不能修改自己的角色": "You cannot change your own role",
```
`zh-CN.json` 同键同值。`"角色"`、`"官方计费"`、`"普通用户"`、`"供应商"`、`"管理员"` 已存在，不重复加。

- [ ] **Step 5: 检查**

Run: `cd web/classic && bun run build 2>&1 | tail -5`
Expected: 构建成功（此步只加代码未引用，预期无报错）。

---

### Task 8: 渠道页 viewer 模式 + 页面/路由/侧栏

**Files:**
- Modify: `web/classic/src/hooks/channels/useChannelsData.jsx:44-56,688-700,789-800,1311-1316`
- Modify: `web/classic/src/components/table/channels/index.jsx:39-70`
- Modify: `web/classic/src/components/table/channels/ChannelsTable.jsx:40-110,169-178`
- Modify: `web/classic/src/components/table/channels/ChannelsColumnDefs.jsx:321-348,494-524,596-606,617-636,636-745,746+`
- Modify: `web/classic/src/components/table/channels/ChannelsActions.jsx:50-110,285-300`
- Modify: `web/classic/src/components/table/channels/ChannelsFilters.jsx:37,109`
- Create: `web/classic/src/pages/ViewerChannels/index.jsx`
- Modify: `web/classic/src/App.jsx:59,397-405`
- Modify: `web/classic/src/components/layout/SiderBar.jsx:28,34-59,175-208,571-583`

- [ ] **Step 1: 钩子**

```jsx
  const isSupplierMode = mode === 'supplier';
  // 观察员模式：只读白名单接口，无任何写操作；分组接口走 /api/viewer/groups。
  const isViewerMode = mode === 'viewer';
  const apiBase = isViewerMode
    ? '/api/viewer/channel'
    : isSupplierMode
      ? '/api/supplier/channel'
      : '/api/channel';
```

`fetchGroups`：

```jsx
      let res = await API.get(
        isViewerMode
          ? `/api/viewer/groups`
          : isSupplierMode
            ? `/api/supplier/self/groups`
            : `/api/group/`,
      );
```

`urlSupplier`：`const urlSupplier = isSupplierMode || isViewerMode ? '' : searchParams.get('supplier') || '';`

`handleRow` 不变（只改背景色）。return 对象加 `isViewerMode,`。

- [ ] **Step 2: `index.jsx`**：`EditChannelModal`、`BatchTagModal`、`ModelTestModal`、`MultiKeyManageModal`、`ChannelUpstreamUpdateModal`、`EditTagModal` 全部包在 `{!channelsData.isViewerMode && (...)}` 内；`ColumnSelectorModal` 保留。

- [ ] **Step 3: `ChannelsTable.jsx`**：解构加 `isViewerMode`，传给 `getChannelsColumns({... isViewerMode ...})` 并加入 `useMemo` 依赖；`rowSelection={enableBatchDelete && !isViewerMode ? {...} : null}`。

- [ ] **Step 4: `ChannelsColumnDefs.jsx`**：签名加 `isViewerMode = false,`。
  - 创建者/应收款那段三元改为：

```jsx
    ...(isViewerMode
      ? []
      : isSupplierMode
        ? [ /* 应收款列，原样 */ ]
        : [ /* 创建者列，原样 */ ]),
```

  - 余额标签：`onClick={() => { if (!isViewerMode) updateChannelBalance(record); }}`，`className={record.type === 57 && !isViewerMode ? 'cursor-pointer' : ''}`。
  - 成本价列、优先级列、权重列、操作列：各自包成 `...(isViewerMode ? [] : [{ ...原列... }])`。为避免大段缩进改动，可在 `return [` 之前构建 `const columns = [...]`，末尾 `return isViewerMode ? columns.filter((c) => !VIEWER_HIDDEN.has(c.key)) : columns;`，其中

```jsx
  const VIEWER_HIDDEN = new Set([
    'cost_price',
    'receivable',
    COLUMN_KEYS.SUPPLIER,
    COLUMN_KEYS.PRIORITY,
    COLUMN_KEYS.WEIGHT,
    COLUMN_KEYS.OPERATE,
  ]);
```

  推荐用 filter 方案（改动最小，且列 key 已存在）。

- [ ] **Step 5: `ChannelsActions.jsx`**：props 加 `isViewerMode`。观察员只保留"刷新"与筛选相关控件：把第一行左侧整个批量按钮容器、Dropdown、以及右侧的"批量删除开关"、"标签聚合模式"开关都包在 `{!isViewerMode && (...)}` 中；保留搜索/刷新按钮。做法：找到 `return (` 后的最外层 `<div className='flex flex-col gap-2'>`，其第一个子块（批量操作行）整体 `{!isViewerMode && (<div ...>...</div>)}`。若刷新按钮位于该行内，则单独抽出来放在条件之外。

- [ ] **Step 6: `ChannelsFilters.jsx`**：props 加 `isViewerMode`，供应商筛选框条件改为 `{!isSupplierMode && !isViewerMode && (...)}`。

- [ ] **Step 7: 页面**（`pages/ViewerChannels/index.jsx`，版权头与 `SupplierChannels` 一致）

```jsx
import React from 'react';
import ChannelsPage from '../../components/table/channels';

const ViewerChannels = () => {
  return (
    <div className='classic-page-fill'>
      <ChannelsPage mode='viewer' />
    </div>
  );
};

export default ViewerChannels;
```

- [ ] **Step 8: 路由**（`App.jsx`）：`import { AuthRedirect, PrivateRoute, AdminRoute, SupplierRoute, ViewerRoute } from './helpers';`（确认 `helpers/index.js` 已 re-export auth.jsx 全部导出）；`const ViewerChannels = lazy(() => import('./pages/ViewerChannels'));`；在 `/console/supplier/channels` 路由后加：

```jsx
        <Route
          path='/console/viewer/channels'
          element={
            <ViewerRoute>
              <Suspense fallback={<Loading></Loading>} key={location.pathname}>
                <ViewerChannels />
              </Suspense>
            </ViewerRoute>
          }
        />
```

- [ ] **Step 9: 侧栏**（`SiderBar.jsx`）：import 加 `isViewer`；`routerMap` 加 `viewer_channels: '/console/viewer/channels', viewer_logs: '/console/viewer/logs'`；`supplierItems` 之后加：

```jsx
  const viewerItems = useMemo(() => {
    return [
      {
        text: t('资源总览'),
        itemKey: 'viewer_channels',
        to: '/console/viewer/channels',
      },
      {
        text: t('全平台日志'),
        itemKey: 'viewer_logs',
        to: '/console/viewer/logs',
      },
    ];
  }, [t]);
```

在供应商区域 JSX 之后加：

```jsx
          {/* 观察员区域 - 仅观察员（role === 3）可见；控制台/个人中心照常显示 */}
          {isViewer() && (
            <>
              <Divider className='sidebar-divider' />
              <div>
                {!collapsed && (
                  <div className='sidebar-group-label'>{t('观察员')}</div>
                )}
                {viewerItems.map((item) => renderNavItem(item))}
              </div>
            </>
          )}
```

- [ ] **Step 10: 构建**

Run: `cd web/classic && bun run build 2>&1 | tail -5`
Expected: 成功。

---

### Task 9: 日志页 viewer 模式 + 供应商隐藏售价 UI

**Files:**
- Modify: `web/classic/src/hooks/usage-logs/useUsageLogsData.jsx:47,82-90,118-145,353-370,372-385,800-812,890-905`
- Modify: `web/classic/src/components/table/usage-logs/index.jsx:33-35`
- Modify: `web/classic/src/components/table/usage-logs/UsageLogsTable.jsx:45-72`
- Modify: `web/classic/src/components/table/usage-logs/UsageLogsColumnDefs.jsx:483-486,522,592,805-832,836`
- Modify: `web/classic/src/components/table/usage-logs/UsageLogsFilters.jsx:34,105`
- Modify: `web/classic/src/components/table/usage-logs/UsageLogsActions.jsx:26-33,57-58`
- Create: `web/classic/src/pages/ViewerLogs/index.jsx`
- Modify: `web/classic/src/App.jsx`

- [ ] **Step 1: 钩子签名与角色标志**

```jsx
export const useLogsData = ({ mode = 'self' } = {}) => {
  ...
  const isAdminUser = isAdmin();
  const isSupplierUser = isSupplier();
  // 观察员全站日志页：显式 mode='viewer'（观察员的 /log 仍是本人日志，走普通用户分支）。
  const isViewerUser = mode === 'viewer';
  const STORAGE_KEY = isViewerUser
    ? 'logs-table-columns-viewer'
    : isAdminUser
      ? 'logs-table-columns-admin'
      : isSupplierUser
        ? 'logs-table-columns-supplier'
        : 'logs-table-columns-user';
```

默认列可见性：

```jsx
      [COLUMN_KEYS.CHANNEL]: isAdminUser || isSupplierUser || isViewerUser,
      [COLUMN_KEYS.USERNAME]: isAdminUser && !isViewerUser,
      [COLUMN_KEYS.TOKEN]: !isSupplierUser && !isViewerUser,
      ...
      // 供应商看不到平台售价（花费列），观察员/管理员/终端用户可见
      [COLUMN_KEYS.COST]: !isSupplierUser,
      [COLUMN_KEYS.PAYABLE]: (isAdminUser || isSupplierUser) && !isViewerUser,
      [COLUMN_KEYS.RETRY]: isAdminUser && !isViewerUser,
      [COLUMN_KEYS.IP]: !isSupplierUser && !isViewerUser,
```

列表 URL（`isSupplierUser` 分支之前插入）：

```jsx
    if (isViewerUser) {
      url = `/api/viewer/log/?p=${startIdx}&page_size=${pageSize}&type=${currentLogType}&model_name=${model_name}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}&channel=${channel}&group=${group}&request_id=${request_id}`;
    } else if (isSupplierUser) {
```

统计：新增

```jsx
  const getLogViewerStat = async () => {
    const { model_name, start_timestamp, end_timestamp, channel, group, logType: formLogType } = getFormValues();
    const currentLogType = formLogType !== undefined ? formLogType : logType;
    let localStartTimestamp = Date.parse(start_timestamp) / 1000;
    let localEndTimestamp = Date.parse(end_timestamp) / 1000;
    let url = `/api/viewer/log/stat?type=${currentLogType}&model_name=${model_name}&start_timestamp=${localStartTimestamp}&end_timestamp=${localEndTimestamp}&channel=${channel}&group=${group}`;
    url = encodeURI(url);
    let res = await API.get(url);
    const { success, message, data } = res.data;
    if (success) {
      setStat(data);
    } else {
      showError(message);
    }
  };
```

`handleEyeClick`：`if (isViewerUser) { await getLogViewerStat(); } else if (isSupplierUser) {...`。`showUserInfoFunc` 加 `if (!isAdminUser || isViewerUser) return;`。return 对象加 `isViewerUser,`。

- [ ] **Step 2: `index.jsx`**：`const LogsPage = ({ mode } = {}) => { const logsData = useLogsData({ mode }); ...`。`UserInfoModal`、`ChannelAffinityUsageCacheModal` 包在 `{!logsData.isViewerUser && (...)}`。

- [ ] **Step 3: `UsageLogsTable.jsx`**：解构与 `getLogsColumns` 调用、`useMemo` 依赖都加 `isViewerUser`。

- [ ] **Step 4: `UsageLogsColumnDefs.jsx`**：签名加 `isViewerUser = false,`。
  - 渠道列条件：`(isAdminUser || isSupplierUser || isViewerUser) && (...)`。
  - 用户列：`isAdminUser && !isViewerUser ? (...) : (<></>)`。
  - `admin_info` 相关渲染（第 922 行 `return isAdminUser ? <div>{content}</div> : <></>;`）改为 `isAdminUser && !isViewerUser`。
  - 花费列：在 render 开头加 `if (isSupplierUser) return <></>;`（后端已归零，双保险）。

- [ ] **Step 5: `UsageLogsFilters.jsx`**：props 加 `isViewerUser`；渠道 ID 输入对观察员也开放，用户名输入只给管理员：

```jsx
          {(isAdminUser || isViewerUser) && (
            <Form.Input field='channel' ... />
          )}
          {isAdminUser && !isViewerUser && (
            <Form.Input field='username' ... />
          )}
```

令牌名筛选框（若存在 `field='token_name'`）对观察员隐藏：包 `{!isViewerUser && (...)}`。

- [ ] **Step 6: `UsageLogsActions.jsx`**：props 加 `isSupplierUser`；第一个 Tag 内容改为

```jsx
            {isSupplierUser
              ? `${t('官方计费')}: $${Number(stat.official_usd || 0).toFixed(4)}`
              : `${t('消耗额度')}: ${renderQuota(stat.quota)}`}
```

- [ ] **Step 7: 页面**（`pages/ViewerLogs/index.jsx`）

```jsx
import React from 'react';
import UsageLogsTable from '../../components/table/usage-logs';

const ViewerLogs = () => (
  <div className='mt-[60px] px-2'>
    <UsageLogsTable mode='viewer' />
  </div>
);

export default ViewerLogs;
```

- [ ] **Step 8: 路由**：`const ViewerLogs = lazy(() => import('./pages/ViewerLogs'));`，`/console/viewer/logs` 用 `ViewerRoute` 包裹，写法同 Task 8 Step 8。

- [ ] **Step 9: 构建**

Run: `cd web/classic && bun run build 2>&1 | tail -5`
Expected: 成功。

---

### Task 10: 用户角色下拉框 + 删除提升/降级

**Files:**
- Modify: `web/classic/src/components/table/users/modals/EditUserModal.jsx:82-96,355-370`
- Modify: `web/classic/src/components/table/users/UsersColumnDefs.jsx:212-226,301-314,325-337,395-404`
- Modify: `web/classic/src/components/table/users/UsersTable.jsx:28-29,57-58,69-76,106-114,144-145,157-158,213-227`
- Delete: `web/classic/src/components/table/users/modals/PromoteUserModal.jsx`、`DemoteUserModal.jsx`

- [ ] **Step 1: `EditUserModal`**
  - import 加 `import { isRoot } from '../../../../helpers';`（确认 helpers 导出路径与同目录其他 import 一致）。
  - `getInitValues` 加 `role: 1,`。
  - 角色选项：

```jsx
  const currentUser = (() => {
    try {
      return JSON.parse(localStorage.getItem('user') || 'null');
    } catch {
      return null;
    }
  })();
  const roleOptions = [
    { label: t('普通用户'), value: 1 },
    { label: t('观察员'), value: 3 },
    { label: t('供应商'), value: 5 },
    ...(isRoot() ? [{ label: t('管理员'), value: 10 }] : []),
  ];
  const targetIsRoot = inputs?.role === 100;
  const targetIsSelf = Boolean(currentUser && userId && Number(userId) === currentUser.id);
  const roleDisabled = targetIsRoot || targetIsSelf;
```

  - 在"分组" `<Col span={24}>` 之后加：

```jsx
                      {isEdit && (
                        <Col span={24}>
                          <Form.Select
                            field='role'
                            label={t('角色')}
                            placeholder={t('请选择角色')}
                            optionList={
                              targetIsRoot
                                ? [{ label: t('超级管理员'), value: 100 }]
                                : roleOptions
                            }
                            disabled={roleDisabled}
                            extraText={
                              targetIsRoot
                                ? t('超级管理员的角色不可修改')
                                : targetIsSelf
                                  ? t('不能修改自己的角色')
                                  : undefined
                            }
                          />
                        </Col>
                      )}
```

  - `submit`：`if (!isEdit || roleDisabled) delete payload.role;`（自更新与禁用态不传 role；后端 0/缺省=不改）。

- [ ] **Step 2: `UsersColumnDefs.jsx`**：删除 `renderOperations` 参数中的 `showPromoteModal, showDemoteModal`、JSX 里"提升""降级"两个 `<Button>`、`getUsersColumns` 参数与 `renderOperations(...)` 调用中的两项。

- [ ] **Step 3: `UsersTable.jsx`**：删除两个 import、两个 state、`showPromoteUserModal/showDemoteUserModal`、两个 confirm handler、`getUsersColumns` 参数与依赖中的两项、末尾两个 Modal JSX。

- [ ] **Step 4: 删文件**

Run: `git rm web/classic/src/components/table/users/modals/PromoteUserModal.jsx web/classic/src/components/table/users/modals/DemoteUserModal.jsx`

- [ ] **Step 5: 全局引用检查**

Run: `grep -rn "PromoteUserModal\|DemoteUserModal\|showPromoteModal\|showDemoteModal\|'promote'\|'demote'" web/classic/src`
Expected: 无输出（`useUsersData.jsx:123` 的注释改为 `// Manage user operations (enable, disable, delete, unlock, add_quota)`）。

- [ ] **Step 6: 构建**

Run: `cd web/classic && bun run build 2>&1 | tail -5`
Expected: 成功。

---

### Task 11: 全量验证 + 端到端自测 + WORKLOG + 提交

- [ ] **Step 1: 后端全量**

Run: `go build ./... && go vet ./common/ ./middleware/ ./controller/ ./model/ ./router/ && go test ./common/ ./middleware/ ./controller/ ./model/ 2>&1 | tail -30`
Expected: 新增测试全 PASS；失败集合与基线一致（基线白名单见 `docs/known-stale-tests.txt`；`controller` 已知 `TestListModelsTokenLimitIncludesTieredBillingModel`）。

- [ ] **Step 2: 前端构建**

Run: `cd web/classic && bun run build 2>&1 | tail -5`
Expected: 成功。

- [ ] **Step 3: 本地端到端**（现有 compose PG，见 memory `local-deploy-db-connection`）

按 `run` 技能或既有方式启动后端 + `bun run dev`，用三种账号验证设计文档"测试计划 · 前端"中的每一条；至少用 curl 断言接口面：

```bash
# 观察员会话 cookie 记为 $CK，用户 id 记为 $UID
curl -s -b "$CK" -H "New-Api-User: $UID" 'http://localhost:3000/api/channel/?p=1&page_size=1' | head -c 200      # 期望 success:false 权限不足
curl -s -b "$CK" -H "New-Api-User: $UID" 'http://localhost:3000/api/viewer/channel/?p=1&page_size=1' | grep -o '"cost_price"\|"base_url"\|"key"' # 期望无输出
curl -s -b "$CK" -H "New-Api-User: $UID" 'http://localhost:3000/api/viewer/log/?p=1&page_size=1' | grep -o '"username":"[^"]*"'   # 期望 "username":""
# 供应商会话
curl -s -b "$SCK" -H "New-Api-User: $SUID" 'http://localhost:3000/api/supplier/self/logs?p=1&page_size=1' | grep -o '"quota":[0-9]*\|group_ratio' # 期望 "quota":0 且无 group_ratio
curl -s -b "$SCK" -H "New-Api-User: $SUID" 'http://localhost:3000/api/supplier/self/logs/stat' # 期望含 official_usd，无 quota
```

- [ ] **Step 4: WORKLOG**（`docs/superpowers/WORKLOG.md` 末尾追加一条 `### [2026-09-17] 观察员角色 + 供应商端隐藏售价 + 角色下拉框`，写清需求、落地文件、验证结果、提交状态）。

- [ ] **Step 5: 提交**（用户已授权 commit；不 push）

```bash
git add common middleware controller model i18n router web/classic/src docs/superpowers/specs/2026-09-17-tokenki-viewer-role-supplier-price-hiding-design.md docs/superpowers/plans/2026-09-17-tokenki-viewer-role-supplier-price-hiding.md docs/superpowers/WORKLOG.md
git commit -m "feat(role): viewer role with read-only channel/log scope, hide selling price from suppliers, role select in user editor"
```

> `docs/deploy/report/2026-09-17-tokenki-prod-v2026.09.17.2.md` 与 WORKLOG 中上一条"生产重发 v2026.09.17.2"是前一会话遗留的待提交文档；WORKLOG 同文件无法拆分，会随本次提交进入仓库，发版报告是否一并加入在提交前向用户说明。

---

## Self-Review

- **Spec 覆盖**：§1.1 常量/鉴权/路由/白名单/日志脱敏 → T1–T4；§1.2 前端 → T7–T9；§2 → T5 + T9 Step 1/4/6；§3 → T6 + T10；边界/测试 → T11。模型广场配置项无代码任务（设计已定不改代码）。
- **占位符**：Task 3 Step 1 关于 `Children` 的条件说明是"先确认再选分支"，两条路径都给了完整代码。
- **类型一致**：`channelListOptions{forceSupplierId, viewer}` 在 T3 定义、T3/T4 使用；`viewerChannelView/viewerChannelViews` 命名一致；`blankViewerLog`、`blankSellingPrice`、`SumSupplierOfficialUsd`、`validateRoleChange` 在测试与实现中同名同签名；前端 `isViewerMode`（渠道）与 `isViewerUser`（日志）刻意区分以匹配各自既有命名（`isSupplierMode` / `isSupplierUser`）。
