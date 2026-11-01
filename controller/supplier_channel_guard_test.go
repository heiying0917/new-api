package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/ratio_setting"
	"github.com/QuantumNous/new-api/types"

	"github.com/stretchr/testify/require"
)

// 夹具：渠道 22 属供应商 777（seedConsumptionFixture），补上供应商资料与 abilities 表。
func seedSupplierChannelGuardFixture(t *testing.T, supplierEnabled, supplierAutoDisabled bool) {
	t.Helper()
	seedConcurrencyFixture(t)
	require.NoError(t, model.DB.Create(&model.Supplier{UserId: 777}).Error)
	require.NoError(t, model.DB.Model(&model.Supplier{}).Where("user_id = ?", 777).
		Updates(map[string]interface{}{"enabled": supplierEnabled, "auto_disabled": supplierAutoDisabled}).Error)
}

func setChannelFields(t *testing.T, id int, fields map[string]interface{}) {
	t.Helper()
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", id).Updates(fields).Error)
}

func channelOf(t *testing.T, id int) *model.Channel {
	t.Helper()
	ch, err := model.GetChannelById(id, true)
	require.NoError(t, err)
	return ch
}

func supplierOf(t *testing.T, id int) *model.Supplier {
	t.Helper()
	s, err := model.GetSupplierByUserId(id)
	require.NoError(t, err)
	return s
}

// 供应商不能关闭自动禁用：编辑与新增时传 auto_ban=0 都会被强制为开启。
func TestSupplierChannel_AutoBanAlwaysOn(t *testing.T) {
	seedSupplierChannelGuardFixture(t, true, false)
	setChannelFields(t, 22, map[string]interface{}{"auto_ban": 0})

	c, w := newJSONContext(777, http.MethodPut, "/api/supplier/channel/", `{"id":22,"auto_ban":0,"name":"renamed"}`)
	SupplierUpdateChannel(c)
	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())
	require.True(t, channelOf(t, 22).GetAutoBan())
	require.Equal(t, "renamed", channelOf(t, 22).Name)

	c, w = newJSONContext(777, http.MethodPost, "/api/supplier/channel/", `{"name":"n","key":"k","models":"m","cost_price":1.5,"auto_ban":0}`)
	SupplierAddChannel(c)
	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())
	var created model.Channel
	require.NoError(t, model.DB.Where("supplier_id = ? AND name = ?", 777, "n").First(&created).Error)
	require.True(t, created.GetAutoBan())
}

// 被自动禁用的渠道：测试不通过时拒绝启用、保持禁用，同次提交的其它修改照常保存。
func TestSupplierChannel_EnableAutoDisabledRejectedWhenTestFails(t *testing.T) {
	seedSupplierChannelGuardFixture(t, false, true)
	// Midjourney 类型不支持渠道测试，测试必然失败（不依赖网络）
	setChannelFields(t, 22, map[string]interface{}{"status": common.ChannelStatusAutoDisabled, "type": constant.ChannelTypeMidjourney})

	c, w := newJSONContext(777, http.MethodPut, "/api/supplier/channel/", `{"id":22,"status":1,"name":"fixed-key"}`)
	SupplierUpdateChannel(c)
	require.Contains(t, w.Body.String(), `"success":false`, "resp=%s", w.Body.String())
	require.Contains(t, w.Body.String(), "测试未通过", "resp=%s", w.Body.String())

	ch := channelOf(t, 22)
	require.Equal(t, common.ChannelStatusAutoDisabled, ch.Status, "测试不通过必须保持禁用")
	require.Equal(t, "fixed-key", ch.Name, "其它修改照常保存")
	require.False(t, supplierOf(t, 777).Enabled, "渠道没启用，供应商不能被级联启用")
}

// 被自动禁用的渠道：测试通过才启用，并把因无可用渠道被自动停用的供应商一并恢复。
func TestSupplierChannel_EnableAutoDisabledAfterPassingTest(t *testing.T) {
	seedSupplierChannelGuardFixture(t, false, true)
	savedModelRatios := ratio_setting.ModelRatio2JSONString()
	t.Cleanup(func() { require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(savedModelRatios)) })
	require.NoError(t, ratio_setting.UpdateModelRatioByJSONString(`{"gpt-4o-mini":0.075}`))
	service.InitHttpClient() // 生产启动时初始化，测试里需手动初始化转发用的 HTTP 客户端
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"x","object":"chat.completion","created":1,"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(upstream.Close)
	setChannelFields(t, 22, map[string]interface{}{
		"status": common.ChannelStatusAutoDisabled, "type": constant.ChannelTypeOpenAI,
		"base_url": upstream.URL, "models": "gpt-4o-mini",
	})

	c, w := newJSONContext(777, http.MethodPut, "/api/supplier/channel/", `{"id":22,"status":1}`)
	SupplierUpdateChannel(c)
	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())

	ch := channelOf(t, 22)
	require.Equal(t, common.ChannelStatusEnabled, ch.Status)
	require.Empty(t, ch.GetOtherInfo()["status_reason"], "启用后清除禁用原因")
	s := supplierOf(t, 777)
	require.True(t, s.Enabled, "因无可用渠道被自动停用的供应商随之恢复")
	require.False(t, s.AutoDisabled)
}

// 供应商启停自己的渠道（非自动禁用）：供应商状态随之级联，但管理员手动停用的供应商不会被恢复。
func TestSupplierChannel_StatusChangeCascadesSupplier(t *testing.T) {
	seedSupplierChannelGuardFixture(t, false, true)
	setChannelFields(t, 22, map[string]interface{}{"status": common.ChannelStatusManuallyDisabled})

	c, w := newJSONContext(777, http.MethodPut, "/api/supplier/channel/", `{"id":22,"status":1}`)
	SupplierUpdateChannel(c)
	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())
	require.Equal(t, common.ChannelStatusEnabled, channelOf(t, 22).Status)
	require.True(t, supplierOf(t, 777).Enabled, "自动停用的供应商随渠道启用恢复")

	c, w = newJSONContext(777, http.MethodPut, "/api/supplier/channel/", `{"id":22,"status":2}`)
	SupplierUpdateChannel(c)
	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())
	require.False(t, supplierOf(t, 777).Enabled, "名下无可用渠道 → 供应商自动停用")

	// 管理员手动停用供应商后，供应商自己启用渠道不能把自己恢复
	require.NoError(t, model.UpdateSupplier(777, map[string]interface{}{"enabled": false}))
	c, w = newJSONContext(777, http.MethodPut, "/api/supplier/channel/", `{"id":22,"status":1}`)
	SupplierUpdateChannel(c)
	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())
	require.Equal(t, common.ChannelStatusEnabled, channelOf(t, 22).Status)
	require.False(t, supplierOf(t, 777).Enabled, "管理员停用的供应商不受渠道启用影响")
}

// 手动测试命中渠道失效规则：与转发失败一样自动禁用；限流、模型 ID 无效、总开关关闭、未开自动禁用都不禁用。
func TestDisableChannelOnTestFailure(t *testing.T) {
	seedSupplierChannelGuardFixture(t, true, false)
	prev := common.AutomaticDisableChannelEnabled
	t.Cleanup(func() { common.AutomaticDisableChannelEnabled = prev })
	common.AutomaticDisableChannelEnabled = true

	apiErr := func(msg string, status int) testResult {
		return testResult{newAPIError: types.NewOpenAIError(errors.New(msg), types.ErrorCodeAwsInvokeError, status)}
	}
	reset := func(autoBan int) *model.Channel {
		setChannelFields(t, 22, map[string]interface{}{"status": common.ChannelStatusEnabled, "auto_ban": autoBan})
		return channelOf(t, 22)
	}

	require.False(t, disableChannelOnTestFailure(reset(1), apiErr("ThrottlingException: Too many requests, please wait before trying again.", 429)))
	require.False(t, disableChannelOnTestFailure(reset(1), apiErr("ValidationException: The provided model identifier is invalid.", 400)), "手动测试可能选错模型，不禁用")
	require.Equal(t, common.ChannelStatusEnabled, channelOf(t, 22).Status)

	require.False(t, disableChannelOnTestFailure(reset(0), apiErr("ValidationException: Operation not allowed", 400)), "未开自动禁用")
	common.AutomaticDisableChannelEnabled = false
	require.False(t, disableChannelOnTestFailure(reset(1), apiErr("ValidationException: Operation not allowed", 400)), "总开关关闭")
	common.AutomaticDisableChannelEnabled = true

	require.True(t, disableChannelOnTestFailure(reset(1), apiErr("AccessDeniedException: Authentication failed: Please make sure your API Key is valid.", 403)))
	ch := channelOf(t, 22)
	require.Equal(t, common.ChannelStatusAutoDisabled, ch.Status)
	require.Contains(t, ch.GetOtherInfo()["status_reason"], "Please make sure your API Key is valid")
}
