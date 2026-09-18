package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type repriceResp struct {
	Success bool `json:"success"`
	Data    struct {
		DryRun        bool    `json:"dry_run"`
		TotalLogs     int64   `json:"total_logs"`
		TotalDeltaCny float64 `json:"total_delta_cny"`
		Channels      []struct {
			ChannelId  int     `json:"channel_id"`
			SupplierId int     `json:"supplier_id"`
			NewPrice   float64 `json:"new_price"`
			Logs       int64   `json:"logs"`
			DeltaCny   float64 `json:"delta_cny"`
		} `json:"channels"`
	} `json:"data"`
}

// 渠道 22（供应商 777）当前成本价 ¥2.0，但未结算日志还带着旧价 ¥2.2；另有一条已打包日志。
func seedRepriceForController(t *testing.T, currentPrice float64) {
	t.Helper()
	setupInitiateSettlementDB(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Ability{})) // Channel.Update 会同步 abilities
	common.MemoryCacheEnabled = false
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: 22, Name: "op5-1", Key: "k", Type: 14, SupplierId: 777, CreatedBy: 777, CostPrice: &currentPrice, Status: 1, Models: "m", Group: "g",
	}).Error)
	require.NoError(t, model.LOG_DB.Create([]*model.Log{
		{Id: 1, UserId: 1, Type: model.LogTypeConsume, ChannelId: 22, SettlementId: 0, OfficialUsd: 1.0, CostPriceSnapshot: 2.2, ModelName: "m"},
		{Id: 2, UserId: 1, Type: model.LogTypeConsume, ChannelId: 22, SettlementId: 5, OfficialUsd: 1.0, CostPriceSnapshot: 2.2, ModelName: "m"},
	}).Error)
}

func newJSONContext(userId int, method, path, body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("id", userId)
	return c, w
}

func snapshotOf(t *testing.T, id int) float64 {
	t.Helper()
	var l model.Log
	require.NoError(t, model.LOG_DB.First(&l, id).Error)
	return l.CostPriceSnapshot
}

// 缺省 dry_run：只汇报每个渠道的差额，不写库不记账。
func TestAdminRepriceUnsettled_DefaultsToDryRun(t *testing.T) {
	seedRepriceForController(t, 2.0)

	c, w := newJSONContext(1, http.MethodPost, "/api/admin/settlement/reprice-unsettled", `{}`)
	AdminRepriceUnsettled(c)

	require.Equal(t, http.StatusOK, w.Code, "resp=%s", w.Body.String())
	var resp repriceResp
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp), "resp=%s", w.Body.String())
	require.True(t, resp.Success, "resp=%s", w.Body.String())
	require.True(t, resp.Data.DryRun)
	require.Len(t, resp.Data.Channels, 1)
	require.Equal(t, 22, resp.Data.Channels[0].ChannelId)
	require.EqualValues(t, 1, resp.Data.Channels[0].Logs)
	require.InDelta(t, 2.0-2.2, resp.Data.Channels[0].DeltaCny, 1e-9)
	require.EqualValues(t, 1, resp.Data.TotalLogs)
	require.InDelta(t, 2.0-2.2, resp.Data.TotalDeltaCny, 1e-9)

	require.InDelta(t, 2.2, snapshotOf(t, 1), 1e-9)
	var ledgers int64
	require.NoError(t, model.DB.Model(&model.SettlementLedger{}).Count(&ledgers).Error)
	require.EqualValues(t, 0, ledgers)
}

// 显式 dry_run=false：未结算日志改为当前价，已打包不动，按渠道记一条 action=reprice 账本（金额为差额）。
func TestAdminRepriceUnsettled_ApplyWritesAndRecordsLedger(t *testing.T) {
	seedRepriceForController(t, 2.0)

	c, w := newJSONContext(1, http.MethodPost, "/api/admin/settlement/reprice-unsettled", `{"dry_run":false}`)
	AdminRepriceUnsettled(c)

	require.Equal(t, http.StatusOK, w.Code, "resp=%s", w.Body.String())
	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())
	require.InDelta(t, 2.0, snapshotOf(t, 1), 1e-9)
	require.InDelta(t, 2.2, snapshotOf(t, 2), 1e-9)

	var led model.SettlementLedger
	require.NoError(t, model.DB.Where("action = ?", "reprice").First(&led).Error)
	require.Equal(t, 777, led.SupplierId)
	require.Equal(t, 0, led.SettlementId)
	require.Equal(t, 1, led.OperatorId)
	require.True(t, led.OperatorIsAdmin)
	require.InDelta(t, 1.0, led.OfficialUsd, 1e-9)
	require.InDelta(t, 2.0-2.2, led.ComputedCNY, 1e-9)
	require.Contains(t, led.Remark, "22")
}

// 供应商自己改成本价：保存后未结算日志立刻按新价重定价，并记一条供应商操作的 reprice 账本。
func TestSupplierUpdateChannel_CostPriceChangeRepricesUnsettledLogs(t *testing.T) {
	seedRepriceForController(t, 2.2)

	c, w := newJSONContext(777, http.MethodPut, "/api/supplier/channel/", `{"id":22,"name":"op5-1","type":14,"models":"m","group":"g","cost_price":2.0}`)
	SupplierUpdateChannel(c)

	require.Equal(t, http.StatusOK, w.Code, "resp=%s", w.Body.String())
	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())

	ch, err := model.GetChannelById(22, false)
	require.NoError(t, err)
	require.NotNil(t, ch.CostPrice)
	require.InDelta(t, 2.0, *ch.CostPrice, 1e-9)
	require.InDelta(t, 2.0, snapshotOf(t, 1), 1e-9)
	require.InDelta(t, 2.2, snapshotOf(t, 2), 1e-9)

	var led model.SettlementLedger
	require.NoError(t, model.DB.Where("action = ?", "reprice").First(&led).Error)
	require.Equal(t, 777, led.SupplierId)
	require.Equal(t, 777, led.OperatorId)
	require.False(t, led.OperatorIsAdmin)
	require.InDelta(t, 2.0-2.2, led.ComputedCNY, 1e-9)
}

// 成本价没变（或请求未携带 cost_price）：不重定价、不记账。
func TestSupplierUpdateChannel_UnchangedCostPriceDoesNotReprice(t *testing.T) {
	seedRepriceForController(t, 2.2)

	c, w := newJSONContext(777, http.MethodPut, "/api/supplier/channel/", `{"id":22,"name":"renamed","type":14,"models":"m","group":"g"}`)
	SupplierUpdateChannel(c)

	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())
	require.InDelta(t, 2.2, snapshotOf(t, 1), 1e-9)
	var ledgers int64
	require.NoError(t, model.DB.Model(&model.SettlementLedger{}).Count(&ledgers).Error)
	require.EqualValues(t, 0, ledgers)
}
