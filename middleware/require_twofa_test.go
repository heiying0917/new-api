package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func setupTwoFADB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.RedisEnabled = false
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.TwoFA{}, &model.PasskeyCredential{}))
	model.DB = db
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
}

// performRequireTwoFA 构造一条带 RequireTwoFAEnabled 中间件的路由，预置 c.id=userId，
// 返回响应记录器与「业务处理器是否被执行」。
func performRequireTwoFA(t *testing.T, userId int) (*httptest.ResponseRecorder, *bool) {
	t.Helper()
	reached := false
	router := gin.New()
	router.GET("/k", func(c *gin.Context) {
		c.Set("id", userId)
		c.Next()
	}, RequireTwoFAEnabled(), func(c *gin.Context) {
		reached = true
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/k", nil)
	router.ServeHTTP(w, req)
	return w, &reached
}

// TestRequireTwoFAEnabled_BlocksWhenNeither 未开启 2FA 也无 Passkey → 403 + TWO_FA_NOT_ENABLED，业务不执行。
func TestRequireTwoFAEnabled_BlocksWhenNeither(t *testing.T) {
	setupTwoFADB(t)
	w, reached := performRequireTwoFA(t, 1001)
	require.Equal(t, http.StatusForbidden, w.Code)
	require.Contains(t, w.Body.String(), "TWO_FA_NOT_ENABLED")
	require.False(t, *reached)
}

// TestRequireTwoFAEnabled_AllowsWhen2FA 已开启 2FA → 放行。
func TestRequireTwoFAEnabled_AllowsWhen2FA(t *testing.T) {
	setupTwoFADB(t)
	require.NoError(t, model.DB.Create(&model.TwoFA{UserId: 1002, IsEnabled: true, Secret: "s"}).Error)
	w, reached := performRequireTwoFA(t, 1002)
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, *reached)
}

// TestRequireTwoFAEnabled_AllowsWhenPasskey 无 2FA 但有 Passkey → 放行。
func TestRequireTwoFAEnabled_AllowsWhenPasskey(t *testing.T) {
	setupTwoFADB(t)
	require.NoError(t, model.DB.Create(&model.PasskeyCredential{
		UserID: 1003, CredentialID: "cid-1003", PublicKey: "pk",
	}).Error)
	w, reached := performRequireTwoFA(t, 1003)
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, *reached)
}

// TestRequireTwoFAEnabled_RejectsAnonymous 未登录(id=0) → 401。
func TestRequireTwoFAEnabled_RejectsAnonymous(t *testing.T) {
	setupTwoFADB(t)
	w, reached := performRequireTwoFA(t, 0)
	require.Equal(t, http.StatusUnauthorized, w.Code)
	require.False(t, *reached)
}
