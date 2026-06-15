package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupPrivescTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db
	model.LOG_DB = db
	require.NoError(t, db.AutoMigrate(&model.User{}, &model.Supplier{}, &model.Token{}, &model.Log{}))
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// TestRegister_IgnoresInjectedRoleStatusQuota 端到端验证公开注册对“注入提权”免疫：
// 攻击者在注册请求体里塞 role=100(超管)/status/quota/group，注册成功后落库的角色
// 必须恒为「供应商(5)」、额度为新用户默认值、分组非攻击者指定值。
// 这是对“无法通过注入提权”最直接的回归证明。
func TestRegister_IgnoresInjectedRoleStatusQuota(t *testing.T) {
	setupPrivescTestDB(t)

	prevReg, prevPwd, prevEmail := common.RegisterEnabled, common.PasswordRegisterEnabled, common.EmailVerificationEnabled
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.EmailVerificationEnabled = false
	t.Cleanup(func() {
		common.RegisterEnabled = prevReg
		common.PasswordRegisterEnabled = prevPwd
		common.EmailVerificationEnabled = prevEmail
	})

	r := gin.New()
	r.Use(sessions.Sessions("session", cookie.NewStore([]byte("privesc-test"))))
	r.POST("/api/user/register", Register)

	// 恶意 payload：注入超管角色 + 巨额额度 + 特权分组 + 启用状态
	body := `{"username":"evilreg","password":"password123","role":100,"status":1,"quota":999999,"group":"root"}`
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "resp=%s", w.Body.String())
	require.Contains(t, w.Body.String(), `"success":true`, "注册应成功: %s", w.Body.String())

	var got model.User
	require.NoError(t, model.DB.Where("username = ?", "evilreg").First(&got).Error)
	assert.Equal(t, common.RoleSupplierUser, got.Role, "注入的 role=100 必须被忽略，落库应为供应商(5)")
	assert.Equal(t, common.UserStatusEnabled, got.Status, "status 应为启用")
	assert.Equal(t, common.QuotaForNewUser, got.Quota, "注入的 quota 必须被忽略")
	assert.NotEqual(t, "root", got.Group, "注入的 group 必须被忽略")
}

// TestCanManageTargetRole 锁定角色层级不变式（CreateUser/UpdateUser/ManageUser 共用）：
// 普通管理员(10) 不能管理/创建 >= 自己等级的账号；只有超管(100) 能管理任意人。
// 注意：供应商(5) 在中间件层（AdminAuth，要求 >=10）就被挡死，永远到不了此函数。
func TestCanManageTargetRole(t *testing.T) {
	admin := common.RoleAdminUser
	root := common.RoleRootUser

	// 管理员无法染指管理员/超管
	assert.False(t, canManageTargetRole(admin, common.RoleAdminUser), "管理员不能管理同级管理员")
	assert.False(t, canManageTargetRole(admin, common.RoleRootUser), "管理员不能管理超管")
	// 管理员可管理供应商/普通用户
	assert.True(t, canManageTargetRole(admin, common.RoleSupplierUser))
	assert.True(t, canManageTargetRole(admin, common.RoleCommonUser))
	// 超管可管理任意人（含另一超管）
	assert.True(t, canManageTargetRole(root, common.RoleRootUser))
	assert.True(t, canManageTargetRole(root, common.RoleAdminUser))
}
