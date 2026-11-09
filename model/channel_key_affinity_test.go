package model

import (
	"fmt"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"

	"github.com/stretchr/testify/require"
)

// awsRegionKeysForAffinityTest 生产形态的多 key 渠道：同一个 AWS 凭证的各 region key，只有末尾的 region 不同。
func awsRegionKeysForAffinityTest() []string {
	regions := []string{"us-east-1", "us-east-2", "us-west-2", "eu-west-1", "eu-central-1", "ap-northeast-1", "ap-southeast-1", "ap-southeast-2", "ap-south-1"}
	keys := make([]string, len(regions))
	for i, r := range regions {
		keys[i] = "AKIAEXAMPLEEXAMPLE00|wJalrXUtnFEMIexampleSecretKeyExample00|" + r
	}
	return keys
}

func newKeyAffinityTestChannel(id int, keys []string, mode constant.MultiKeyMode) *Channel {
	return &Channel{
		Id:          id,
		Key:         strings.Join(keys, "\n"),
		ChannelInfo: ChannelInfo{IsMultiKey: true, MultiKeySize: len(keys), MultiKeyMode: mode},
	}
}

func pickKeyByAffinityForTest(t *testing.T, ch *Channel, affinity string, avoid func(string) bool) string {
	t.Helper()
	key, idx, apiErr := ch.GetEnabledKeyByAffinity(affinity, avoid)
	require.Nil(t, apiErr)
	require.Equal(t, ch.GetKeys()[idx], key, "返回的下标必须对应返回的 key（日志 multi_key_index 依赖它）")
	return key
}

// 同一亲和标识在多 key 渠道内始终选同一把 key，与渠道的随机 / 轮询模式无关（Bedrock 缓存按 region 隔离，换 key 就丢缓存）。
func TestGetEnabledKeyByAffinity_SameAffinitySameKey(t *testing.T) {
	for _, mode := range []constant.MultiKeyMode{constant.MultiKeyModeRandom, constant.MultiKeyModePolling} {
		t.Run(string(mode), func(t *testing.T) {
			ch := newKeyAffinityTestChannel(9101, awsRegionKeysForAffinityTest(), mode)
			first := pickKeyByAffinityForTest(t, ch, "session-a", nil)
			for i := 0; i < 50; i++ {
				require.Equal(t, first, pickKeyByAffinityForTest(t, ch, "session-a", nil))
			}
		})
	}
}

// 不同会话均匀分散到各把 key（各 region），不能把负载压到同一个 region。
func TestGetEnabledKeyByAffinity_SpreadsSessionsEvenly(t *testing.T) {
	keys := awsRegionKeysForAffinityTest()
	ch := newKeyAffinityTestChannel(9102, keys, constant.MultiKeyModeRandom)
	const sessions = 9000
	counts := map[string]int{}
	for i := 0; i < sessions; i++ {
		counts[pickKeyByAffinityForTest(t, ch, fmt.Sprintf("session-%d", i), nil)]++
	}
	expect := sessions / len(keys)
	for _, k := range keys {
		require.InDelta(t, expect, counts[k], float64(expect)*0.15, "key %s 分到 %d 个会话，期望约 %d", k, counts[k], expect)
	}
}

// 选中的 key 被避开（429 冷却）时只顺延这一部分会话，且顺延到各自的下一把（分散到其余 key，而不是全压到同一把）；
// 其它会话不受影响。
func TestGetEnabledKeyByAffinity_AvoidedKeyOnlyMovesItsSessions(t *testing.T) {
	keys := awsRegionKeysForAffinityTest()
	ch := newKeyAffinityTestChannel(9103, keys, constant.MultiKeyModeRandom)
	cooling := keys[3]
	avoid := func(k string) bool { return k == cooling }

	movedTo := map[string]int{}
	for i := 0; i < 2000; i++ {
		s := fmt.Sprintf("session-%d", i)
		before := pickKeyByAffinityForTest(t, ch, s, nil)
		after := pickKeyByAffinityForTest(t, ch, s, avoid)
		require.NotEqual(t, cooling, after, "冷却中的 key 必须避开")
		if before != cooling {
			require.Equal(t, before, after, "原本不在冷却 key 上的会话不能被挪动")
			continue
		}
		movedTo[after]++
		require.Equal(t, after, pickKeyByAffinityForTest(t, ch, s, avoid), "冷却期间顺延到的 key 也要稳定")
	}
	require.GreaterOrEqual(t, len(movedTo), len(keys)-3, "冷却 key 上的会话应分散到其余多把 key，实际 %v", movedTo)
}

// 被禁用的 key 不会被选中，原本在它上面的会话顺延，其它会话不动；key 全部被避开时照常在启用 key 里按亲和选。
func TestGetEnabledKeyByAffinity_DisabledKeyAndAllAvoided(t *testing.T) {
	keys := awsRegionKeysForAffinityTest()
	ch := newKeyAffinityTestChannel(9104, keys, constant.MultiKeyModeRandom)
	before := map[string]string{}
	for i := 0; i < 500; i++ {
		s := fmt.Sprintf("session-%d", i)
		before[s] = pickKeyByAffinityForTest(t, ch, s, nil)
	}

	ch.ChannelInfo.MultiKeyStatusList = map[int]int{5: common.ChannelStatusAutoDisabled}
	for s, k := range before {
		after := pickKeyByAffinityForTest(t, ch, s, nil)
		require.NotEqual(t, keys[5], after, "禁用的 key 不能被选中")
		if k != keys[5] {
			require.Equal(t, k, after, "禁用一把 key 不影响其它会话")
		}
	}

	all := func(string) bool { return true }
	for s := range before {
		after := pickKeyByAffinityForTest(t, ch, s, all)
		require.NotEqual(t, keys[5], after, "全部被避开时也只能在启用 key 里选")
		require.Equal(t, after, pickKeyByAffinityForTest(t, ch, s, all), "全部被避开时仍按亲和稳定选")
	}
}

// 亲和按 key 内容而不是下标计算：key 顺序变化（编辑渠道、增删其它 key）不会让会话换 region。
func TestGetEnabledKeyByAffinity_IndependentOfKeyOrder(t *testing.T) {
	keys := awsRegionKeysForAffinityTest()
	reversed := make([]string, len(keys))
	for i, k := range keys {
		reversed[len(keys)-1-i] = k
	}
	a := newKeyAffinityTestChannel(9105, keys, constant.MultiKeyModeRandom)
	b := newKeyAffinityTestChannel(9106, reversed, constant.MultiKeyModeRandom)
	for i := 0; i < 300; i++ {
		s := fmt.Sprintf("session-%d", i)
		require.Equal(t, pickKeyByAffinityForTest(t, a, s, nil), pickKeyByAffinityForTest(t, b, s, nil))
	}
}

// 没有亲和标识时保持原有行为：轮询模式依次轮转。
func TestGetEnabledKeyByAffinity_EmptyAffinityKeepsModeBehavior(t *testing.T) {
	prev := common.MemoryCacheEnabled
	common.MemoryCacheEnabled = true
	t.Cleanup(func() { common.MemoryCacheEnabled = prev })

	ch := newKeyAffinityTestChannel(9107, []string{"ka", "kb", "kc"}, constant.MultiKeyModePolling)
	channelSyncLock.Lock()
	prevIDM := channelsIDM
	channelsIDM = map[int]*Channel{ch.Id: ch}
	channelSyncLock.Unlock()
	t.Cleanup(func() {
		channelSyncLock.Lock()
		channelsIDM = prevIDM
		channelSyncLock.Unlock()
	})

	got := make([]string, 0, 6)
	for i := 0; i < 6; i++ {
		got = append(got, pickKeyByAffinityForTest(t, ch, "", nil))
	}
	require.Equal(t, []string{"ka", "kb", "kc", "ka", "kb", "kc"}, got)
}

// 单 key 渠道直接返回 key 本身。
func TestGetEnabledKeyByAffinity_SingleKeyChannel(t *testing.T) {
	ch := &Channel{Id: 9108, Key: "only"}
	key, idx, apiErr := ch.GetEnabledKeyByAffinity("session-a", nil)
	require.Nil(t, apiErr)
	require.Equal(t, "only", key)
	require.Equal(t, 0, idx)
}
