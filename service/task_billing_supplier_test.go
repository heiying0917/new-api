package service

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLogTaskConsumption_RecordsSupplierPayable 验证任务计费路径(suno/视频/MJ 等)
// 会把官方价美元写入日志：official_usd 由 quota 反推(不含分组折扣)，cost_price_snapshot 由
// RecordConsumeLog 按渠道当前成本价自动冻结，二者相乘即「应付供应商」。
func TestLogTaskConsumption_RecordsSupplierPayable(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)
	common.MemoryCacheEnabled = false

	const userID, channelID = 40, 40
	cp := 5.0
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: channelID, Name: "sup", Key: "k", SupplierId: 777, CostPrice: &cp,
		Status: common.ChannelStatusEnabled, Models: "m", Group: "default",
	}).Error)
	seedUser(t, userID, 1_000_000)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/task/submit", nil)

	groupRatio := 2.0
	quota := int(3 * groupRatio * common.QuotaPerUnit) // 期望反推 officialUsd = 3.0
	info := &relaycommon.RelayInfo{
		UserId:          userID,
		OriginModelName: "suno-v3",
		UsingGroup:      "default",
		PriceData: types.PriceData{
			Quota:          quota,
			ModelPrice:     0.5,
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: groupRatio},
		},
		ChannelMeta:   &relaycommon.ChannelMeta{ChannelId: channelID},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: "submit"},
	}

	LogTaskConsumption(c, info)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.Equal(t, model.LogTypeConsume, log.Type)
	assert.InDelta(t, 3.0, log.OfficialUsd, 1e-9)
	assert.InDelta(t, 5.0, log.CostPriceSnapshot, 1e-9)
}

// TestLogTaskConsumption_NonSupplierChannelNoPayable 回归守卫：非供应商渠道即使任务路径
// 无条件算出了 official_usd，也必须被 RecordConsumeLog 归一清零（不进入应付/结算口径）。
func TestLogTaskConsumption_NonSupplierChannelNoPayable(t *testing.T) {
	truncate(t)
	gin.SetMode(gin.TestMode)
	common.MemoryCacheEnabled = false

	const userID, channelID = 41, 41
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: channelID, Name: "plain", Key: "k", SupplierId: 0, // 非供应商渠道
		Status: common.ChannelStatusEnabled, Models: "m", Group: "default",
	}).Error)
	seedUser(t, userID, 1_000_000)

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/task/submit", nil)

	groupRatio := 2.0
	info := &relaycommon.RelayInfo{
		UserId:          userID,
		OriginModelName: "suno-v3",
		UsingGroup:      "default",
		PriceData: types.PriceData{
			Quota:          int(3 * groupRatio * common.QuotaPerUnit),
			GroupRatioInfo: types.GroupRatioInfo{GroupRatio: groupRatio},
		},
		ChannelMeta:   &relaycommon.ChannelMeta{ChannelId: channelID},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{Action: "submit"},
	}

	LogTaskConsumption(c, info)

	log := getLastLog(t)
	require.NotNil(t, log)
	assert.InDelta(t, 0.0, log.OfficialUsd, 1e-9)
	assert.InDelta(t, 0.0, log.CostPriceSnapshot, 1e-9)
}
