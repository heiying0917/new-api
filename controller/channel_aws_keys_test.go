package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/require"
)

// 两组 AWS 凭证：AK1 可用 2 个区域，AK2 可用 1 个区域（「批量生成可用区域密钥」写回的形态）
const (
	awsTestAK1 = "AKIATESTAAAAAAAA1111"
	awsTestAK2 = "AKIATESTBBBBBBBB2222"
)

var (
	awsTestAK1Lines = []string{awsTestAK1 + "|sk1|us-east-1", awsTestAK1 + "|sk1|us-west-2"}
	awsTestAK2Lines = []string{awsTestAK2 + "|sk2|us-east-1"}
)

func joinKeyLines(groups ...[]string) string {
	all := make([]string, 0)
	for _, g := range groups {
		all = append(all, g...)
	}
	return strings.Join(all, "\n")
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := common.Marshal(v)
	require.NoError(t, err)
	return string(b)
}

func channelsNamedPrefix(t *testing.T, prefix string) []model.Channel {
	t.Helper()
	var chs []model.Channel
	require.NoError(t, model.DB.Where("name LIKE ?", prefix+"%").Order("id").Find(&chs).Error)
	return chs
}

func adminAddChannel(t *testing.T, body map[string]any) string {
	t.Helper()
	c, w := newJSONContext(1, http.MethodPost, "/api/channel/", mustJSON(t, body))
	AddChannel(c)
	return w.Body.String()
}

func awsChannelPayload(name, key string) map[string]any {
	return map[string]any{
		"name": name, "type": constant.ChannelTypeAws, "key": key,
		"models": "claude-haiku-4-5", "group": "default",
	}
}

// 聚合模式下多组 AWS 凭证：每组凭证一个多 key 渠道，渠道内是该凭证的各区域 key，名称追加 AK 后四位。
func TestAddChannel_AwsAggregateSplitsByCredential(t *testing.T) {
	seedConcurrencyFixture(t)

	resp := adminAddChannel(t, map[string]any{
		"mode": "multi_to_single", "multi_key_mode": "random",
		"channel": awsChannelPayload("awsb", joinKeyLines(awsTestAK1Lines, awsTestAK2Lines)),
	})
	require.Contains(t, resp, `"success":true`, "resp=%s", resp)

	chs := channelsNamedPrefix(t, "awsb")
	require.Len(t, chs, 2, "两组凭证应建两个渠道")

	require.Equal(t, "awsb-1111", chs[0].Name)
	require.Equal(t, strings.Join(awsTestAK1Lines, "\n"), chs[0].Key)
	require.True(t, chs[0].ChannelInfo.IsMultiKey)
	require.Equal(t, 2, chs[0].ChannelInfo.MultiKeySize)
	require.Equal(t, constant.MultiKeyModeRandom, chs[0].ChannelInfo.MultiKeyMode)

	require.Equal(t, "awsb-2222", chs[1].Name)
	require.Equal(t, strings.Join(awsTestAK2Lines, "\n"), chs[1].Key)
	require.True(t, chs[1].ChannelInfo.IsMultiKey)
	require.Equal(t, 1, chs[1].ChannelInfo.MultiKeySize)
}

// 单组凭证聚合：仍是一个渠道，名称不变。
func TestAddChannel_AwsAggregateSingleCredentialKeepsOneChannel(t *testing.T) {
	seedConcurrencyFixture(t)

	resp := adminAddChannel(t, map[string]any{
		"mode": "multi_to_single", "multi_key_mode": "random",
		"channel": awsChannelPayload("awsb", joinKeyLines(awsTestAK1Lines)),
	})
	require.Contains(t, resp, `"success":true`, "resp=%s", resp)

	chs := channelsNamedPrefix(t, "awsb")
	require.Len(t, chs, 1)
	require.Equal(t, "awsb", chs[0].Name)
	require.Equal(t, 2, chs[0].ChannelInfo.MultiKeySize)
}

// APIKey|Region 形态按 APIKey 分组。
func TestAddChannel_AwsAggregateGroupsApiKeyForm(t *testing.T) {
	seedConcurrencyFixture(t)

	resp := adminAddChannel(t, map[string]any{
		"mode": "multi_to_single", "multi_key_mode": "random",
		"channel": awsChannelPayload("awsk", "ABSKfirstkeyAAAA|us-east-1\nABSKfirstkeyAAAA|us-west-2\nABSKsecondkeyBBBB|us-east-1"),
	})
	require.Contains(t, resp, `"success":true`, "resp=%s", resp)

	chs := channelsNamedPrefix(t, "awsk")
	require.Len(t, chs, 2)
	require.Equal(t, "awsk-AAAA", chs[0].Name)
	require.Equal(t, 2, chs[0].ChannelInfo.MultiKeySize)
	require.Equal(t, "awsk-BBBB", chs[1].Name)
	require.Equal(t, 1, chs[1].ChannelInfo.MultiKeySize)
}

// 聚合模式下有格式错误的行：无法判断属于哪组凭证，整体拒绝，不建任何渠道。
func TestAddChannel_AwsAggregateRejectsMalformedLine(t *testing.T) {
	seedConcurrencyFixture(t)

	resp := adminAddChannel(t, map[string]any{
		"mode": "multi_to_single", "multi_key_mode": "random",
		"channel": awsChannelPayload("awsb", joinKeyLines(awsTestAK1Lines)+"\nnot-a-valid-line"),
	})
	require.Contains(t, resp, `"success":false`, "resp=%s", resp)
	require.Contains(t, resp, "第 3 行", "resp=%s", resp)
	require.Empty(t, channelsNamedPrefix(t, "awsb"))
}

// AWS 单密钥模式填了多行：转发时会整串当一个 key 用而必然失败，直接拒绝。
func TestAddChannel_AwsSingleModeRejectsMultipleLines(t *testing.T) {
	seedConcurrencyFixture(t)

	resp := adminAddChannel(t, map[string]any{
		"mode":    "single",
		"channel": awsChannelPayload("awsb", joinKeyLines(awsTestAK1Lines)),
	})
	require.Contains(t, resp, `"success":false`, "resp=%s", resp)
	require.Empty(t, channelsNamedPrefix(t, "awsb"))
}

// 批量创建并把 key 前缀加到名称：每个渠道只加自己的前缀，不会把前一个渠道的前缀累加进来。
func TestAddChannel_BatchKeyPrefixNamesDoNotAccumulate(t *testing.T) {
	seedConcurrencyFixture(t)

	resp := adminAddChannel(t, map[string]any{
		"mode": "batch", "batch_add_set_key_prefix_2_name": true,
		"channel": map[string]any{"name": "op", "type": constant.ChannelTypeOpenAI, "key": "sk-aaaaaaaa1\nsk-bbbbbbbb2", "models": "gpt-4o-mini", "group": "default"},
	})
	require.Contains(t, resp, `"success":true`, "resp=%s", resp)

	chs := channelsNamedPrefix(t, "op ")
	require.Len(t, chs, 2)
	require.Equal(t, "op sk-aaaaa", chs[0].Name)
	require.Equal(t, "op sk-bbbbb", chs[1].Name)
}

// ---------------- 编辑：多组凭证不合并 ----------------

func seedAwsChannel(t *testing.T, id int, supplierId int, key string, multiKey bool) {
	t.Helper()
	ch := model.Channel{Id: id, Name: "aws-edit", Key: key, Type: constant.ChannelTypeAws, SupplierId: supplierId, CreatedBy: supplierId, Status: 1, Models: "claude-haiku-4-5", Group: "default"}
	if supplierId != 0 {
		cp := 1.5
		ch.CostPrice = &cp
	}
	if multiKey {
		ch.ChannelInfo = model.ChannelInfo{IsMultiKey: true, MultiKeySize: len(strings.Split(key, "\n")), MultiKeyMode: constant.MultiKeyModeRandom}
	}
	require.NoError(t, model.DB.Create(&ch).Error)
}

func adminUpdateChannel(t *testing.T, body map[string]any) string {
	t.Helper()
	c, w := newJSONContext(1, http.MethodPut, "/api/channel/", mustJSON(t, body))
	UpdateChannel(c)
	return w.Body.String()
}

// 编辑单 key 渠道转多 key 时只允许一组凭证。
func TestUpdateChannel_AwsConvertRejectsMultipleCredentials(t *testing.T) {
	seedConcurrencyFixture(t)
	seedAwsChannel(t, 30, 0, awsTestAK1Lines[0], false)

	resp := adminUpdateChannel(t, map[string]any{
		"id": 30, "type": constant.ChannelTypeAws, "mode": "multi_to_single", "multi_key_mode": "random",
		"key": joinKeyLines(awsTestAK1Lines, awsTestAK2Lines),
	})
	require.Contains(t, resp, `"success":false`, "resp=%s", resp)
	require.Contains(t, resp, "新建渠道", "resp=%s", resp)

	ch := channelOf(t, 30)
	require.Equal(t, awsTestAK1Lines[0], ch.Key, "被拒绝时密钥保持不变")
	require.False(t, ch.ChannelInfo.IsMultiKey)
}

// 编辑单 key 渠道转多 key：单组凭证照常转换。
func TestUpdateChannel_AwsConvertSingleCredential(t *testing.T) {
	seedConcurrencyFixture(t)
	seedAwsChannel(t, 30, 0, awsTestAK1Lines[0], false)

	resp := adminUpdateChannel(t, map[string]any{
		"id": 30, "type": constant.ChannelTypeAws, "mode": "multi_to_single", "multi_key_mode": "random",
		"key": joinKeyLines(awsTestAK1Lines),
	})
	require.Contains(t, resp, `"success":true`, "resp=%s", resp)

	ch := channelOf(t, 30)
	require.True(t, ch.ChannelInfo.IsMultiKey)
	require.Equal(t, 2, ch.ChannelInfo.MultiKeySize)
	require.Equal(t, joinKeyLines(awsTestAK1Lines), ch.Key)
}

// 已是多 key 的 AWS 渠道追加另一组凭证的 key：拒绝，不把两个账号合进一个渠道。
func TestUpdateChannel_AwsAppendRejectsOtherCredential(t *testing.T) {
	seedConcurrencyFixture(t)
	seedAwsChannel(t, 31, 0, joinKeyLines(awsTestAK1Lines), true)

	resp := adminUpdateChannel(t, map[string]any{
		"id": 31, "type": constant.ChannelTypeAws, "key_mode": "append", "key": awsTestAK2Lines[0],
	})
	require.Contains(t, resp, `"success":false`, "resp=%s", resp)
	require.Equal(t, joinKeyLines(awsTestAK1Lines), channelOf(t, 31).Key)
}

// AWS 单 key 渠道直接填多行（未转多 key）：拒绝。
func TestUpdateChannel_AwsSingleKeyRejectsMultipleLines(t *testing.T) {
	seedConcurrencyFixture(t)
	seedAwsChannel(t, 30, 0, awsTestAK1Lines[0], false)

	resp := adminUpdateChannel(t, map[string]any{
		"id": 30, "type": constant.ChannelTypeAws, "key": joinKeyLines(awsTestAK1Lines),
	})
	require.Contains(t, resp, `"success":false`, "resp=%s", resp)
	require.Equal(t, awsTestAK1Lines[0], channelOf(t, 30).Key)
}

// 不改密钥的普通编辑不受影响（历史上已混了多组凭证的渠道也能改名）。
func TestUpdateChannel_AwsMixedChannelEditWithoutKeyStillWorks(t *testing.T) {
	seedConcurrencyFixture(t)
	seedAwsChannel(t, 31, 0, joinKeyLines(awsTestAK1Lines, awsTestAK2Lines), true)

	resp := adminUpdateChannel(t, map[string]any{"id": 31, "type": constant.ChannelTypeAws, "name": "renamed"})
	require.Contains(t, resp, `"success":true`, "resp=%s", resp)
	require.Equal(t, "renamed", channelOf(t, 31).Name)
	require.Equal(t, 3, channelOf(t, 31).ChannelInfo.MultiKeySize)

	// 前端编辑多 key 渠道总会带 key_mode=append，密钥框为空时也一样：不算改密钥，不能被拦
	resp = adminUpdateChannel(t, map[string]any{"id": 31, "type": constant.ChannelTypeAws, "name": "renamed2", "key_mode": "append"})
	require.Contains(t, resp, `"success":true`, "resp=%s", resp)
	require.Equal(t, "renamed2", channelOf(t, 31).Name)
	require.Equal(t, joinKeyLines(awsTestAK1Lines, awsTestAK2Lines), channelOf(t, 31).Key)
}

// ---------------- 供应商端 ----------------

func supplierAddChannel(t *testing.T, body map[string]any) string {
	t.Helper()
	c, w := newJSONContext(777, http.MethodPost, "/api/supplier/channel/", mustJSON(t, body))
	SupplierAddChannel(c)
	return w.Body.String()
}

func supplierUpdateChannel(t *testing.T, body map[string]any) string {
	t.Helper()
	c, w := newJSONContext(777, http.MethodPut, "/api/supplier/channel/", mustJSON(t, body))
	SupplierUpdateChannel(c)
	return w.Body.String()
}

// 供应商用「批量生成可用区域密钥」建渠道：与管理员一致，按凭证拆成多 key 渠道，且都归属本人、强制自动禁用。
func TestSupplierAddChannel_AwsAggregateSplitsByCredential(t *testing.T) {
	seedSupplierChannelGuardFixture(t, true, false)

	resp := supplierAddChannel(t, map[string]any{
		"name": "sup-aws", "type": constant.ChannelTypeAws, "key": joinKeyLines(awsTestAK1Lines, awsTestAK2Lines),
		"models": "claude-haiku-4-5", "cost_price": 1.5, "auto_ban": 0,
		"mode": "multi_to_single", "multi_key_mode": "random",
	})
	require.Contains(t, resp, `"success":true`, "resp=%s", resp)

	chs := channelsNamedPrefix(t, "sup-aws")
	require.Len(t, chs, 2)
	for _, ch := range chs {
		require.Equal(t, 777, ch.SupplierId)
		require.Equal(t, 777, ch.CreatedBy)
		require.True(t, ch.GetAutoBan())
		require.True(t, ch.ChannelInfo.IsMultiKey)
	}
	require.Equal(t, "sup-aws-1111", chs[0].Name)
	require.Equal(t, 2, chs[0].ChannelInfo.MultiKeySize)
	require.Equal(t, "sup-aws-2222", chs[1].Name)

	var parsed struct {
		Data struct {
			Id  int   `json:"id"`
			Ids []int `json:"ids"`
		} `json:"data"`
	}
	require.NoError(t, common.UnmarshalJsonStr(resp, &parsed))
	require.Equal(t, chs[0].Id, parsed.Data.Id, "保留单渠道时的 id 字段")
	require.Equal(t, []int{chs[0].Id, chs[1].Id}, parsed.Data.Ids)
}

// 供应商勾选「批量创建」：每行一个渠道，而不是把多行塞进一个渠道。
func TestSupplierAddChannel_BatchCreatesOneChannelPerKey(t *testing.T) {
	seedSupplierChannelGuardFixture(t, true, false)

	resp := supplierAddChannel(t, map[string]any{
		"name": "sup-batch", "type": constant.ChannelTypeOpenAI, "key": "k1\nk2",
		"models": "gpt-4o-mini", "cost_price": 1.5, "mode": "batch",
	})
	require.Contains(t, resp, `"success":true`, "resp=%s", resp)

	chs := channelsNamedPrefix(t, "sup-batch")
	require.Len(t, chs, 2)
	require.Equal(t, "k1", chs[0].Key)
	require.Equal(t, "k2", chs[1].Key)
	require.False(t, chs[0].ChannelInfo.IsMultiKey)
}

// 供应商编辑：「批量生成可用区域密钥」（单组凭证）把单 key 渠道转为多 key。
func TestSupplierUpdateChannel_AwsConvertSingleCredential(t *testing.T) {
	seedSupplierChannelGuardFixture(t, true, false)
	seedAwsChannel(t, 32, 777, awsTestAK1Lines[0], false)

	resp := supplierUpdateChannel(t, map[string]any{
		"id": 32, "type": constant.ChannelTypeAws, "mode": "multi_to_single", "multi_key_mode": "random",
		"key": joinKeyLines(awsTestAK1Lines),
	})
	require.Contains(t, resp, `"success":true`, "resp=%s", resp)

	ch := channelOf(t, 32)
	require.True(t, ch.ChannelInfo.IsMultiKey)
	require.Equal(t, 2, ch.ChannelInfo.MultiKeySize)
	require.Equal(t, constant.MultiKeyModeRandom, ch.ChannelInfo.MultiKeyMode)
}

// 供应商编辑时多组凭证：拒绝，提示新建渠道。
func TestSupplierUpdateChannel_AwsConvertRejectsMultipleCredentials(t *testing.T) {
	seedSupplierChannelGuardFixture(t, true, false)
	seedAwsChannel(t, 32, 777, awsTestAK1Lines[0], false)

	resp := supplierUpdateChannel(t, map[string]any{
		"id": 32, "type": constant.ChannelTypeAws, "mode": "multi_to_single", "multi_key_mode": "random",
		"key": joinKeyLines(awsTestAK1Lines, awsTestAK2Lines),
	})
	require.Contains(t, resp, `"success":false`, "resp=%s", resp)
	require.Equal(t, awsTestAK1Lines[0], channelOf(t, 32).Key)
}

// 供应商多 key 渠道追加同一凭证的区域 key：追加而不是覆盖，key 数随之更新。
func TestSupplierUpdateChannel_MultiKeyAppend(t *testing.T) {
	seedSupplierChannelGuardFixture(t, true, false)
	seedAwsChannel(t, 33, 777, awsTestAK1Lines[0], true)

	resp := supplierUpdateChannel(t, map[string]any{
		"id": 33, "type": constant.ChannelTypeAws, "key_mode": "append", "key": awsTestAK1Lines[1],
	})
	require.Contains(t, resp, `"success":true`, "resp=%s", resp)

	ch := channelOf(t, 33)
	require.Equal(t, joinKeyLines(awsTestAK1Lines), ch.Key)
	require.Equal(t, 2, ch.ChannelInfo.MultiKeySize)
}

// 供应商不能通过请求体篡改多 key 状态（channel_info 以库里为准）。
func TestSupplierUpdateChannel_IgnoresClientChannelInfo(t *testing.T) {
	seedSupplierChannelGuardFixture(t, true, false)
	seedAwsChannel(t, 32, 777, awsTestAK1Lines[0], false)

	resp := supplierUpdateChannel(t, map[string]any{
		"id": 32, "type": constant.ChannelTypeAws, "name": "renamed",
		"channel_info": map[string]any{"is_multi_key": true, "multi_key_size": 99},
	})
	require.Contains(t, resp, `"success":true`, "resp=%s", resp)

	ch := channelOf(t, 32)
	require.Equal(t, "renamed", ch.Name)
	require.False(t, ch.ChannelInfo.IsMultiKey)
}
