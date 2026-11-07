package controller

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 生产上 Anthropic 官 key 实际返回的报错原文（2026-11-06，渠道 161、163–168）
const probeCreditBalanceMsg = "Your credit balance is too low to access the Anthropic API. Please go to Plans & Billing to upgrade or purchase credits."

// 转发时上游 400 的形态（service.RelayErrorHandler 解析 Anthropic 错误体）
func relayCreditBalanceErr() *types.NewAPIError {
	return types.WithOpenAIError(types.OpenAIError{Message: probeCreditBalanceMsg, Type: "invalid_request_error"}, http.StatusBadRequest)
}

// 渠道测试（即探测）失败时的形态：testChannel 把上游非 200 包成 500 的 bad_response
func probeCreditBalanceErr() *types.NewAPIError {
	return types.NewOpenAIError(fmt.Errorf("bad response status code 400, message: %s", probeCreditBalanceMsg), types.ErrorCodeBadResponse, http.StatusInternalServerError)
}

type probeCall struct {
	channelId int
	key       string
	isMulti   bool
	model     string
}

type probeRecorder struct {
	mu     sync.Mutex
	calls  []probeCall
	result *types.NewAPIError
}

func (r *probeRecorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *probeRecorder) last() probeCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls[len(r.calls)-1]
}

// stubDisableProbe 替换探测函数（不真发请求），换一个独立的冷却闸门避免用例之间互相影响，
// 并让禁用流程同步执行，用例返回前禁用与通知都已完成。
func stubDisableProbe(t *testing.T, result *types.NewAPIError) *probeRecorder {
	t.Helper()
	rec := &probeRecorder{result: result}
	prevProbe, prevGate, prevGo := probeChannelForDisable, channelDisableProbeGate, goChannelDisable
	probeChannelForDisable = func(channel *model.Channel, modelName string) *types.NewAPIError {
		key, _, _ := channel.GetNextEnabledKey()
		rec.mu.Lock()
		rec.calls = append(rec.calls, probeCall{channelId: channel.Id, key: key, isMulti: channel.ChannelInfo.IsMultiKey, model: modelName})
		rec.mu.Unlock()
		return rec.result
	}
	channelDisableProbeGate = service.NewDisableProbeGate(time.Now)
	goChannelDisable = func(f func()) { f() }
	t.Cleanup(func() {
		probeChannelForDisable, channelDisableProbeGate, goChannelDisable = prevProbe, prevGate, prevGo
	})
	return rec
}

func setupDisableProbeTestDB(t *testing.T) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	prevSQLite, prevMySQL, prevPG, prevRedis := common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL, common.RedisEnabled
	prevMem, prevAutoDisable := common.MemoryCacheEnabled, common.AutomaticDisableChannelEnabled
	prevDB, prevLogDB := model.DB, model.LOG_DB
	common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL, common.RedisEnabled = true, false, false, false
	common.MemoryCacheEnabled, common.AutomaticDisableChannelEnabled = false, true

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	model.DB, model.LOG_DB = db, db
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}, &model.User{}))
	// DisableChannel 会通知 root 用户；无邮箱则只打日志，不发网络请求
	require.NoError(t, db.Create(&model.User{Id: 1, Username: "root", Role: common.RoleRootUser, Status: common.UserStatusEnabled}).Error)

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		model.DB, model.LOG_DB = prevDB, prevLogDB
		common.UsingSQLite, common.UsingMySQL, common.UsingPostgreSQL, common.RedisEnabled = prevSQLite, prevMySQL, prevPG, prevRedis
		common.MemoryCacheEnabled, common.AutomaticDisableChannelEnabled = prevMem, prevAutoDisable
	})
}

func seedProbeChannel(t *testing.T, id int, key string, status int, multiKey bool) {
	t.Helper()
	autoBan := 1
	ch := &model.Channel{
		Id:      id,
		Type:    constant.ChannelTypeAnthropic,
		Name:    fmt.Sprintf("Anthropic 官key #%d", id),
		Key:     key,
		Status:  status,
		AutoBan: &autoBan,
		Models:  "claude-sonnet-5",
		Group:   "default",
	}
	ch.ChannelInfo.IsMultiKey = multiKey
	if multiKey {
		ch.ChannelInfo.MultiKeySize = len(strings.Split(key, "\n"))
	}
	require.NoError(t, model.DB.Create(ch).Error)
}

func channelStatus(t *testing.T, id int) (int, string) {
	t.Helper()
	ch, err := model.GetChannelById(id, true)
	require.NoError(t, err)
	reason, _ := ch.GetOtherInfo()["status_reason"].(string)
	return ch.Status, reason
}

func newRelayChannelErrorContext(modelName string) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	c.Set("original_model", modelName)
	return c
}

func probeChannelError(id int, key string, multiKey bool) types.ChannelError {
	return *types.NewChannelError(id, constant.ChannelTypeAnthropic, fmt.Sprintf("Anthropic 官key #%d", id), multiKey, key, true)
}

// 2026-11-06 事故回归：某类请求报余额不足、但普通请求探测成功 → 判定为请求级计费限制，渠道保持启用。
func TestProbeThenDisableChannel_ProbeSucceeds_KeepsChannelEnabled(t *testing.T) {
	setupDisableProbeTestDB(t)
	seedProbeChannel(t, 161, "sk-ant-161", common.ChannelStatusEnabled, false)
	rec := stubDisableProbe(t, nil)

	probeThenDisableChannel(probeChannelError(161, "sk-ant-161", false), relayCreditBalanceErr().ErrorWithStatusCode(), "claude-sonnet-5")

	require.Equal(t, 1, rec.count())
	status, _ := channelStatus(t, 161)
	require.Equal(t, common.ChannelStatusEnabled, status)
}

// key 真没钱：普通请求探测也报余额不足 → 禁用，禁用原因保留触发它的原始报错。
func TestProbeThenDisableChannel_ProbeReproducesFatalError_DisablesChannel(t *testing.T) {
	setupDisableProbeTestDB(t)
	seedProbeChannel(t, 162, "sk-ant-162", common.ChannelStatusEnabled, false)
	stubDisableProbe(t, probeCreditBalanceErr())
	reason := relayCreditBalanceErr().ErrorWithStatusCode()

	probeThenDisableChannel(probeChannelError(162, "sk-ant-162", false), reason, "claude-sonnet-5")

	status, statusReason := channelStatus(t, 162)
	require.Equal(t, common.ChannelStatusAutoDisabled, status)
	require.Equal(t, reason, statusReason)
}

// 探测没能给出结论（上游临时故障、网络错误等，不命中禁用规则）→ 不禁用，交给下一次报错再探测。
func TestProbeThenDisableChannel_ProbeInconclusive_KeepsChannelEnabled(t *testing.T) {
	setupDisableProbeTestDB(t)
	seedProbeChannel(t, 163, "sk-ant-163", common.ChannelStatusEnabled, false)
	stubDisableProbe(t, types.NewOpenAIError(errors.New("upstream overloaded"), types.ErrorCodeBadResponse, http.StatusServiceUnavailable))

	probeThenDisableChannel(probeChannelError(163, "sk-ant-163", false), relayCreditBalanceErr().ErrorWithStatusCode(), "claude-sonnet-5")

	status, _ := channelStatus(t, 163)
	require.Equal(t, common.ChannelStatusEnabled, status)
}

// 渠道已被禁用（手动或自动）就不必再花一次请求探测。
func TestProbeThenDisableChannel_SkipsChannelNotEnabled(t *testing.T) {
	setupDisableProbeTestDB(t)
	seedProbeChannel(t, 164, "sk-ant-164", common.ChannelStatusManuallyDisabled, false)
	rec := stubDisableProbe(t, probeCreditBalanceErr())

	probeThenDisableChannel(probeChannelError(164, "sk-ant-164", false), relayCreditBalanceErr().ErrorWithStatusCode(), "claude-sonnet-5")

	require.Equal(t, 0, rec.count())
	status, _ := channelStatus(t, 164)
	require.Equal(t, common.ChannelStatusManuallyDisabled, status)
}

// 同一渠道同一 key 冷却期内重复报错只探测一次。
func TestProbeThenDisableChannel_CooldownProbesOnce(t *testing.T) {
	setupDisableProbeTestDB(t)
	seedProbeChannel(t, 165, "sk-ant-165", common.ChannelStatusEnabled, false)
	rec := stubDisableProbe(t, nil)
	chErr := probeChannelError(165, "sk-ant-165", false)
	reason := relayCreditBalanceErr().ErrorWithStatusCode()

	probeThenDisableChannel(chErr, reason, "claude-sonnet-5")
	probeThenDisableChannel(chErr, reason, "claude-sonnet-5")

	require.Equal(t, 1, rec.count())
}

// 多 key 渠道：探测必须用出错的那个 key（只禁用该 key），并且用原请求的模型。
func TestProbeThenDisableChannel_ProbesFailingKeyWithRequestModel(t *testing.T) {
	setupDisableProbeTestDB(t)
	seedProbeChannel(t, 166, "sk-ant-a\nsk-ant-b\nsk-ant-c", common.ChannelStatusEnabled, true)
	rec := stubDisableProbe(t, nil)

	probeThenDisableChannel(probeChannelError(166, "sk-ant-b", true), relayCreditBalanceErr().ErrorWithStatusCode(), "claude-sonnet-5")

	require.Equal(t, 1, rec.count())
	call := rec.last()
	require.Equal(t, 166, call.channelId)
	require.Equal(t, "sk-ant-b", call.key)
	require.False(t, call.isMulti, "探测渠道固定为单 key，不能轮询到别的 key")
	require.Equal(t, "claude-sonnet-5", call.model)
}

// 转发路径：余额不足报错不再直接禁用，而是先探测；探测复现才禁用。
func TestProcessChannelError_CreditBalanceProbesBeforeDisable(t *testing.T) {
	setupDisableProbeTestDB(t)
	seedProbeChannel(t, 167, "sk-ant-167", common.ChannelStatusEnabled, false)
	rec := stubDisableProbe(t, probeCreditBalanceErr())

	processChannelError(newRelayChannelErrorContext("claude-sonnet-5"), probeChannelError(167, "sk-ant-167", false), relayCreditBalanceErr())

	status, _ := channelStatus(t, 167)
	require.Equal(t, common.ChannelStatusAutoDisabled, status)
	require.Equal(t, 1, rec.count(), "禁用前必须先探测")
	require.Equal(t, "claude-sonnet-5", rec.last().model)
}

// 其它渠道失效报错（key 无效、401 等）行为不变：直接禁用，不探测。
func TestProcessChannelError_OtherFatalErrorDisablesWithoutProbe(t *testing.T) {
	setupDisableProbeTestDB(t)
	seedProbeChannel(t, 168, "sk-ant-168", common.ChannelStatusEnabled, false)
	rec := stubDisableProbe(t, nil)

	processChannelError(newRelayChannelErrorContext("claude-sonnet-5"), probeChannelError(168, "sk-ant-168", false),
		types.WithOpenAIError(types.OpenAIError{Message: "invalid x-api-key", Type: "authentication_error"}, http.StatusUnauthorized))

	status, _ := channelStatus(t, 168)
	require.Equal(t, common.ChannelStatusAutoDisabled, status)
	require.Equal(t, 0, rec.count())
}

// 批量渠道测试发出的本来就是普通请求，余额不足即可直接禁用，不再重复探测。
func TestProcessChannelError_ChannelTestDisablesWithoutProbe(t *testing.T) {
	setupDisableProbeTestDB(t)
	seedProbeChannel(t, 169, "sk-ant-169", common.ChannelStatusEnabled, false)
	rec := stubDisableProbe(t, nil)
	c := newRelayChannelErrorContext("claude-sonnet-5")
	c.Set(channelTestContextKey, true)

	processChannelError(c, probeChannelError(169, "sk-ant-169", false), probeCreditBalanceErr())

	status, _ := channelStatus(t, 169)
	require.Equal(t, common.ChannelStatusAutoDisabled, status)
	require.Equal(t, 0, rec.count())
}

// 不支持渠道测试的类型（Kling、Midjourney 等任务渠道）无法探测：维持原行为直接禁用，不能因探测无结论而永不禁用。
func TestProcessChannelError_UntestableChannelTypeDisablesWithoutProbe(t *testing.T) {
	setupDisableProbeTestDB(t)
	seedProbeChannel(t, 170, "sk-kling-170", common.ChannelStatusEnabled, false)
	rec := stubDisableProbe(t, nil)
	chErr := *types.NewChannelError(170, constant.ChannelTypeKling, "Kling #170", false, "sk-kling-170", true)

	processChannelError(newRelayChannelErrorContext("kling-v2"), chErr, relayCreditBalanceErr())

	status, _ := channelStatus(t, 170)
	require.Equal(t, common.ChannelStatusAutoDisabled, status)
	require.Equal(t, 0, rec.count())
}

// 批量测试经 processChannelError 禁用时靠这个标记跳过探测：testChannel 返回的 context 必须带上它。
func TestTestChannel_MarksContextAsChannelTest(t *testing.T) {
	setupDisableProbeTestDB(t)
	// 多 key 渠道没有可用 key：在发出上游请求前就返回，但已建好测试 context
	ch := &model.Channel{Id: 171, Type: constant.ChannelTypeAnthropic, Name: "no-key", Status: common.ChannelStatusEnabled, Models: "claude-sonnet-5"}
	ch.ChannelInfo.IsMultiKey = true

	result := testChannel(ch, 1, "claude-sonnet-5", "", false)

	require.NotNil(t, result.newAPIError)
	require.NotNil(t, result.context)
	require.True(t, result.context.GetBool(channelTestContextKey))
}
