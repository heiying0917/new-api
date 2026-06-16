package controller

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupInitiateSettlementDB 复刻供应商结算所需表的内存库（users/suppliers/channels/settlements/logs/ledger）。
func setupInitiateSettlementDB(t *testing.T) *gorm.DB {
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
	require.NoError(t, db.AutoMigrate(
		&model.User{}, &model.Supplier{}, &model.Channel{},
		&model.Settlement{}, &model.Log{}, &model.SettlementLedger{},
	))
	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// seedSupplierWithPendingForController 建 1 个供应商用户 + 渠道 + 未结算消费日志，
// payable>0 时该供应商有待结算消费（snapshot=1.0 → computedCNY==officialUsd）。
func seedSupplierWithPendingForController(t *testing.T, supplierId int, payable float64) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.User{
		Id:       supplierId,
		Username: fmt.Sprintf("sup_%d", supplierId),
		Role:     common.RoleSupplierUser,
		Status:   common.UserStatusEnabled,
		AffCode:  fmt.Sprintf("aff_%d", supplierId),
	}).Error)

	channelId := supplierId*10 + 1
	cp := 1.0
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: channelId, Name: "ch", Key: fmt.Sprintf("k_%d", channelId), SupplierId: supplierId,
		CostPrice: &cp, Models: "m", Group: "g",
	}).Error)
	if payable > 0 {
		require.NoError(t, model.LOG_DB.Create(&model.Log{
			Type: model.LogTypeConsume, ChannelId: channelId,
			OfficialUsd: payable, CostPriceSnapshot: 1.0, SettlementId: 0,
		}).Error)
	}
}

// newAdminInitiateContext 构造带 admin id 与 JSON body 的 gin 测试上下文，模拟 RootAuth 已注入 id。
func newAdminInitiateContext(adminId int, body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/settlement/initiate", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("id", adminId)
	return c, w
}

// TestAdminInitiateSettlement_HappyPath 管理员为有待结算消费的供应商立即发起结算：
// 应建出一张「已申请」账单，且落一条 OperatorIsAdmin=true 的 create 账本。
func TestAdminInitiateSettlement_HappyPath(t *testing.T) {
	setupInitiateSettlementDB(t)
	const adminId, supplierId = 1, 901
	seedSupplierWithPendingForController(t, supplierId, 30)

	c, w := newAdminInitiateContext(adminId, fmt.Sprintf(`{"supplier_id":%d}`, supplierId))
	AdminInitiateSettlement(c)

	require.Equal(t, http.StatusOK, w.Code, "resp=%s", w.Body.String())
	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())

	// 账单：状态=已申请，金额来自种子日志
	var s model.Settlement
	require.NoError(t, model.DB.Where("supplier_id = ?", supplierId).First(&s).Error)
	assert.Equal(t, model.SettlementStatusApplied, s.Status, "新账单应为已申请")
	assert.InDelta(t, 30.0, s.OfficialUsd, 1e-9)
	assert.InDelta(t, 30.0, s.ComputedCNY, 1e-9) // snapshot=1.0
	assert.Equal(t, int64(1), s.LogCount)

	// 账本：create + 管理员操作
	var led model.SettlementLedger
	require.NoError(t, model.DB.Where("settlement_id = ? AND action = ?", s.Id, "create").First(&led).Error)
	assert.True(t, led.OperatorIsAdmin, "管理员发起应标记 OperatorIsAdmin=true")
	assert.Equal(t, adminId, led.OperatorId, "OperatorId 应为管理员 id")
	assert.Equal(t, supplierId, led.SupplierId)
}

// TestAdminInitiateSettlement_EmptyPending 供应商无待结算消费：应报错，且不留下任何账单（无孤儿单）。
func TestAdminInitiateSettlement_EmptyPending(t *testing.T) {
	setupInitiateSettlementDB(t)
	const adminId, supplierId = 1, 902
	seedSupplierWithPendingForController(t, supplierId, 0) // 有渠道但无未结算日志

	c, w := newAdminInitiateContext(adminId, fmt.Sprintf(`{"supplier_id":%d}`, supplierId))
	AdminInitiateSettlement(c)

	require.Equal(t, http.StatusOK, w.Code, "resp=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), `"success":false`, "无待结算消费应返回失败: %s", w.Body.String())

	// 关键回归：CreateSettlement 在空时会删除占位账单，故不得留下任何孤儿单
	var cnt int64
	require.NoError(t, model.DB.Model(&model.Settlement{}).Where("supplier_id = ?", supplierId).Count(&cnt).Error)
	assert.Equal(t, int64(0), cnt, "空待结算不得留下孤儿账单")
}

// TestAdminInitiateSettlement_NonSupplier 目标不是供应商：应被校验拦截，不创建账单。
func TestAdminInitiateSettlement_NonSupplier(t *testing.T) {
	setupInitiateSettlementDB(t)
	const adminId = 1
	// 一个普通用户（非供应商角色）
	const normalId = 555
	require.NoError(t, model.DB.Create(&model.User{
		Id: normalId, Username: "normal", Role: common.RoleCommonUser,
		Status: common.UserStatusEnabled, AffCode: "aff_normal",
	}).Error)

	c, w := newAdminInitiateContext(adminId, fmt.Sprintf(`{"supplier_id":%d}`, normalId))
	AdminInitiateSettlement(c)

	require.Equal(t, http.StatusOK, w.Code, "resp=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), `"success":false`, "非供应商应返回失败: %s", w.Body.String())
	assert.Contains(t, w.Body.String(), "目标不是供应商", "resp=%s", w.Body.String())

	var cnt int64
	require.NoError(t, model.DB.Model(&model.Settlement{}).Count(&cnt).Error)
	assert.Equal(t, int64(0), cnt, "非供应商不得创建账单")
}

// TestAdminInitiateSettlement_InvalidSupplierId supplier_id<=0 或缺失：直接报参数错误。
func TestAdminInitiateSettlement_InvalidSupplierId(t *testing.T) {
	setupInitiateSettlementDB(t)
	c, w := newAdminInitiateContext(1, `{"supplier_id":0}`)
	AdminInitiateSettlement(c)
	require.Equal(t, http.StatusOK, w.Code, "resp=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), `"success":false`, "resp=%s", w.Body.String())
	assert.Contains(t, w.Body.String(), "invalid supplier_id", "resp=%s", w.Body.String())
}
