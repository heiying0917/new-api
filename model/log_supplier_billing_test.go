package model

import (
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// TestRecordConsumeLogSupplierBilling 验证 RecordConsumeLog 对供应商计费字段的统一归一：
//   - 供应商渠道：按渠道当前成本价自动补 cost_price_snapshot，保留传入的 official_usd；
//   - 非供应商渠道：official_usd 与 cost_price_snapshot 一律清零（避免脏数据进入结算口径）；
//   - 已显式传入 snapshot（文本路径语义）时不覆盖。
func TestRecordConsumeLogSupplierBilling(t *testing.T) {
	gin.SetMode(gin.TestMode)
	common.MemoryCacheEnabled = false // 让 CacheGetChannel 回退查 DB

	cp := 3.0
	require.NoError(t, DB.Create(&Channel{Id: 90001, Name: "sup", Key: "k", SupplierId: 555, CostPrice: &cp, Status: 1, Models: "m", Group: "g"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 90002, Name: "plain", Key: "k", SupplierId: 0, Status: 1, Models: "m", Group: "g"}).Error)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	// 1) 供应商渠道：传 official=2.0、snapshot=0 → 自动补 snapshot=3.0
	RecordConsumeLog(c, 1, RecordConsumeLogParams{ChannelId: 90001, ModelName: "m", OfficialUsd: 2.0})
	var l1 Log
	require.NoError(t, LOG_DB.Where("channel_id = ?", 90001).Order("id desc").First(&l1).Error)
	require.InDelta(t, 2.0, l1.OfficialUsd, 1e-9)
	require.InDelta(t, 3.0, l1.CostPriceSnapshot, 1e-9)

	// 2) 非供应商渠道：传 official=5.0 → 两字段清零
	RecordConsumeLog(c, 1, RecordConsumeLogParams{ChannelId: 90002, ModelName: "m", OfficialUsd: 5.0})
	var l2 Log
	require.NoError(t, LOG_DB.Where("channel_id = ?", 90002).Order("id desc").First(&l2).Error)
	require.InDelta(t, 0.0, l2.OfficialUsd, 1e-9)
	require.InDelta(t, 0.0, l2.CostPriceSnapshot, 1e-9)

	// 3) 供应商渠道 + 显式 snapshot=9.0（文本路径语义）→ 不覆盖
	RecordConsumeLog(c, 1, RecordConsumeLogParams{ChannelId: 90001, ModelName: "m2", OfficialUsd: 1.0, CostPriceSnapshot: 9.0})
	var l3 Log
	require.NoError(t, LOG_DB.Where("channel_id = ? AND model_name = ?", 90001, "m2").Order("id desc").First(&l3).Error)
	require.InDelta(t, 1.0, l3.OfficialUsd, 1e-9)
	require.InDelta(t, 9.0, l3.CostPriceSnapshot, 1e-9)
}
