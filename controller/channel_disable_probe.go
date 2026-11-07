package controller

import (
	"fmt"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/bytedance/gopkg/util/gopool"
)

// 渠道测试（testChannel）的 gin context 标记：批量测试本身发的就是普通请求，报错后无需再探测。
const channelTestContextKey = "channel_test"

// goChannelDisable 异步执行渠道禁用（含禁用前探测与通知），不阻塞转发；测试中替换为同步执行。
var goChannelDisable = gopool.Go

// channelDisableProbeGate 同一渠道同一 key 冷却期内只探测一次（进程内）。
var channelDisableProbeGate = service.NewDisableProbeGate(time.Now)

// probeChannelForDisable 用一条普通请求（渠道测试请求，不带客户端的 anthropic-beta 等请求头）探测渠道，
// 返回探测报错；能发出请求且成功时返回 nil。测试中替换。
var probeChannelForDisable = func(channel *model.Channel, modelName string) *types.NewAPIError {
	testUserID, err := resolveChannelTestUserID(nil)
	if err != nil {
		return types.NewError(err, types.ErrorCodeGetChannelFailed)
	}
	result := testChannel(channel, testUserID, modelName, "", false)
	if result.newAPIError != nil {
		return result.newAPIError
	}
	if result.localErr != nil {
		return types.NewError(result.localErr, types.ErrorCodeGetChannelFailed)
	}
	return nil
}

// channelPinnedToKey 返回只含指定 key 的单 key 渠道副本，保证探测的是出错的那个 key；不修改原渠道。
func channelPinnedToKey(channel *model.Channel, usingKey string) *model.Channel {
	if usingKey == "" {
		return channel
	}
	pinned := *channel
	pinned.Key = usingKey
	pinned.Keys = nil
	pinned.ChannelInfo.IsMultiKey = false
	return &pinned
}

// probeThenDisableChannel 余额不足类报错（见 service.NeedsDisableProbe）的禁用流程：
// 用普通请求探测出错的 key，探测也命中禁用规则才禁用；探测成功或无结论都不禁用。
func probeThenDisableChannel(channelError types.ChannelError, reason string, modelName string) {
	if !channelDisableProbeGate.TryAcquire(channelError.ChannelId, channelError.UsingKey) {
		return
	}
	channel, err := model.GetChannelById(channelError.ChannelId, true)
	if err != nil {
		common.SysError(fmt.Sprintf("通道 #%d 禁用前探测：读取渠道失败，跳过禁用：%v", channelError.ChannelId, err))
		return
	}
	if channel.Status != common.ChannelStatusEnabled {
		return
	}

	probeErr := probeChannelForDisable(channelPinnedToKey(channel, channelError.UsingKey), modelName)
	if service.ShouldDisableChannel(probeErr) {
		service.DisableChannel(channelError, reason)
		return
	}
	probeResult := "成功"
	if probeErr != nil {
		probeResult = "未命中禁用规则：" + common.LocalLogPreview(probeErr.Error())
	}
	common.SysLog(fmt.Sprintf("通道「%s」（#%d）报错「%s」，但普通请求探测%s，判定为请求级限制（如 Claude Code 请求不在订阅附送额度覆盖范围），不禁用",
		channelError.ChannelName, channelError.ChannelId, common.LocalLogPreview(reason), probeResult))
}
