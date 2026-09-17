package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// performRoleAuth 以会话方式登录 role 用户并请求受 mw 保护的路由，返回业务处理器是否被执行。
// model.DB 置 nil 走 authHelper 的"数据层未就绪→信任会话值"降级分支，避免建库。
func performRoleAuth(t *testing.T, role int, mw gin.HandlerFunc) (*httptest.ResponseRecorder, bool) {
	return performRoleAuthWithStatus(t, role, common.UserStatusEnabled, mw)
}

func performRoleAuthWithStatus(t *testing.T, role int, status int, mw gin.HandlerFunc) (*httptest.ResponseRecorder, bool) {
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
		s.Set("status", status)
		s.Set("group", "default")
		require.NoError(t, s.Save())
		c.Status(http.StatusOK)
	})
	r.GET("/v", mw, func(c *gin.Context) {
		reached = true
		c.JSON(http.StatusOK, gin.H{"success": true})
	})

	lw := httptest.NewRecorder()
	r.ServeHTTP(lw, httptest.NewRequest(http.MethodGet, "/login", nil))

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v", nil)
	req.Header.Set("New-Api-User", "7")
	for _, ck := range lw.Result().Cookies() {
		req.AddCookie(ck)
	}
	r.ServeHTTP(w, req)
	return w, reached
}

// TestViewerAuth_AllowsViewerAndAdmins 观察员本人与管理员以上可进入观察员只读作用域。
func TestViewerAuth_AllowsViewerAndAdmins(t *testing.T) {
	for _, role := range []int{common.RoleViewerUser, common.RoleAdminUser, common.RoleRootUser} {
		w, reached := performRoleAuth(t, role, ViewerAuth())
		require.Equal(t, http.StatusOK, w.Code, "role %d", role)
		require.True(t, reached, "role %d must reach handler", role)
	}
}

// TestViewerAuth_RejectsCommonAndSupplier 普通用户与供应商(5>3)都不能进入：精确匹配而非线性门槛。
func TestViewerAuth_RejectsCommonAndSupplier(t *testing.T) {
	for _, role := range []int{common.RoleCommonUser, common.RoleSupplierUser} {
		w, reached := performRoleAuth(t, role, ViewerAuth())
		require.False(t, reached, "role %d must be rejected", role)
		require.Contains(t, w.Body.String(), `"success":false`)
		// 前端靠 code + role 识别"角色已变更"并同步本地状态，缺一不可。
		require.Contains(t, w.Body.String(), `"code":"`+InsufficientPrivilegeCode+`"`)
		require.Contains(t, w.Body.String(), `"role":`+strconv.Itoa(role))
	}
}

// TestUserAuth_StillLinear 抽取谓词后线性语义不变：普通用户与观察员都可过 UserAuth，观察员不可过 SupplierAuth/AdminAuth。
func TestUserAuth_StillLinear(t *testing.T) {
	_, reached := performRoleAuth(t, common.RoleCommonUser, UserAuth())
	require.True(t, reached)
	_, reached = performRoleAuth(t, common.RoleViewerUser, UserAuth())
	require.True(t, reached)
	_, reached = performRoleAuth(t, common.RoleViewerUser, SupplierAuth())
	require.False(t, reached)
	_, reached = performRoleAuth(t, common.RoleViewerUser, AdminAuth())
	require.False(t, reached)
}

// TestUserAuth_BannedCarriesCode 禁用账号被拒时带稳定 code，前端据此清本地登录态。
func TestUserAuth_BannedCarriesCode(t *testing.T) {
	w, reached := performRoleAuthWithStatus(t, common.RoleCommonUser, common.UserStatusDisabled, UserAuth())
	require.False(t, reached)
	require.Contains(t, w.Body.String(), `"code":"`+UserBannedCode+`"`)
}
