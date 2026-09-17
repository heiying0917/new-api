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

type backfillResp struct {
	Success bool `json:"success"`
	Data    struct {
		DryRun      bool  `json:"dry_run"`
		Scanned     int64 `json:"scanned"`
		Updated     int64 `json:"updated"`
		Done        bool  `json:"done"`
		NextAfterId int   `json:"next_after_id"`
	} `json:"data"`
}

func seedBackfillOfficialUsdForController(t *testing.T) {
	t.Helper()
	setupInitiateSettlementDB(t)
	common.MemoryCacheEnabled = false
	cp := 2.2
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: 22, Name: "op5-1", Key: "k", SupplierId: 777, CostPrice: &cp, Status: 1, Models: "m", Group: "g",
	}).Error)
	require.NoError(t, model.LOG_DB.Create(&model.Log{
		Id: 1, UserId: 1, Type: model.LogTypeConsume, ChannelId: 22, Quota: 65389,
		Other: `{"group_ratio":1}`, OfficialUsd: 0.004785, CostPriceSnapshot: 2.2, ModelName: "claude-opus-5",
	}).Error)
}

func newAdminBackfillContext(adminId int, body string) (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/settlement/backfill-official-usd", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.Request = req
	c.Set("id", adminId)
	return c, w
}

// 缺省即 dry_run：只汇报差额，不写日志表，也不落账本。
func TestAdminBackfillOfficialUsd_DefaultsToDryRun(t *testing.T) {
	seedBackfillOfficialUsdForController(t)

	c, w := newAdminBackfillContext(1, `{}`)
	AdminBackfillOfficialUsd(c)

	require.Equal(t, http.StatusOK, w.Code, "resp=%s", w.Body.String())
	var resp backfillResp
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp), "resp=%s", w.Body.String())
	require.True(t, resp.Success, "resp=%s", w.Body.String())
	require.True(t, resp.Data.DryRun)
	require.EqualValues(t, 1, resp.Data.Scanned)
	require.EqualValues(t, 1, resp.Data.Updated)
	require.True(t, resp.Data.Done)

	var l model.Log
	require.NoError(t, model.LOG_DB.First(&l, 1).Error)
	require.InDelta(t, 0.004785, l.OfficialUsd, 1e-9)
	var ledgers int64
	require.NoError(t, model.DB.Model(&model.SettlementLedger{}).Count(&ledgers).Error)
	require.EqualValues(t, 0, ledgers)
}

// 显式 dry_run=false：落库，并按供应商各记一条 append-only 账本（action=backfill，金额为差额）。
func TestAdminBackfillOfficialUsd_ApplyWritesAndRecordsLedger(t *testing.T) {
	seedBackfillOfficialUsdForController(t)

	c, w := newAdminBackfillContext(1, `{"dry_run":false,"limit":100}`)
	AdminBackfillOfficialUsd(c)

	require.Equal(t, http.StatusOK, w.Code, "resp=%s", w.Body.String())
	var resp backfillResp
	require.NoError(t, common.Unmarshal(w.Body.Bytes(), &resp), "resp=%s", w.Body.String())
	require.True(t, resp.Success, "resp=%s", w.Body.String())
	require.False(t, resp.Data.DryRun)
	require.EqualValues(t, 1, resp.Data.Updated)

	var l model.Log
	require.NoError(t, model.LOG_DB.First(&l, 1).Error)
	require.InDelta(t, 0.130778, l.OfficialUsd, 1e-9)

	var led model.SettlementLedger
	require.NoError(t, model.DB.Where("action = ?", "backfill").First(&led).Error)
	require.Equal(t, 777, led.SupplierId)
	require.Equal(t, 0, led.SettlementId)
	require.Equal(t, 1, led.OperatorId)
	require.True(t, led.OperatorIsAdmin)
	require.InDelta(t, 0.130778-0.004785, led.OfficialUsd, 1e-9)
	require.InDelta(t, (0.130778-0.004785)*2.2, led.ComputedCNY, 1e-9)
}

// 非法 JSON 直接报错，不触碰数据。
func TestAdminBackfillOfficialUsd_RejectsBadBody(t *testing.T) {
	seedBackfillOfficialUsdForController(t)

	c, w := newAdminBackfillContext(1, `{"dry_run":`)
	AdminBackfillOfficialUsd(c)

	require.Contains(t, w.Body.String(), `"success":false`, "resp=%s", w.Body.String())
	var l model.Log
	require.NoError(t, model.LOG_DB.First(&l, 1).Error)
	require.InDelta(t, 0.004785, l.OfficialUsd, 1e-9)
}
