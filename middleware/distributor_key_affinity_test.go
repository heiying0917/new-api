package middleware

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/stretchr/testify/require"
)

// 生产形态：一个渠道 = 一个 AWS 凭证 × 多个 region key。Bedrock 提示词缓存按 region 隔离，
// 渠道亲和只钉到渠道时，渠道内每次换 key 会让多轮会话大部分轮次读不到上一轮写的缓存。
func newRegionKeysChannel(id int, mode constant.MultiKeyMode) model.Channel {
	keys := make([]string, 9)
	for i := range keys {
		keys[i] = fmt.Sprintf("AK%d|SK%d|region-%d", id, id, i)
	}
	return model.Channel{
		Id: id, Name: fmt.Sprintf("c%d", id), Key: strings.Join(keys, "\n"),
		ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: len(keys), MultiKeyMode: mode},
	}
}

// 带 metadata.user_id 的同一会话：每次请求都落在同一渠道的同一把 key 上（随机、轮询模式都一样）。
func TestDistribute_AffinitySessionSticksToOneKey(t *testing.T) {
	for i, mode := range []constant.MultiKeyMode{constant.MultiKeyModeRandom, constant.MultiKeyModePolling} {
		t.Run(string(mode), func(t *testing.T) {
			setupDistributorCooldownDB(t, newRegionKeysChannel(61+i, mode))
			picked := make(chan [2]string, 64)
			router := newCooldownTestRouter(picked)

			for _, session := range []string{"session-a", "session-b", "session-c"} {
				require.Equal(t, http.StatusOK, sendClaudeCodeRequest(router, session))
				first := <-picked
				for n := 0; n < 20; n++ {
					require.Equal(t, http.StatusOK, sendClaudeCodeRequest(router, session))
					require.Equal(t, first, <-picked, "会话 %s 第 %d 次请求换了 key", session, n+2)
				}
			}
		})
	}
}

// 不同会话分散到各把 key，不能把负载压到同一个 region。
func TestDistribute_AffinitySessionsSpreadAcrossKeys(t *testing.T) {
	setupDistributorCooldownDB(t, newRegionKeysChannel(63, constant.MultiKeyModeRandom))
	picked := make(chan [2]string, 64)
	router := newCooldownTestRouter(picked)

	used := map[string]int{}
	for i := 0; i < 180; i++ {
		require.Equal(t, http.StatusOK, sendClaudeCodeRequest(router, fmt.Sprintf("spread-session-%d", i)))
		used[(<-picked)[1]]++
	}
	require.Len(t, used, 9, "180 个会话应覆盖全部 9 把 key，实际 %v", used)
	for k, n := range used {
		require.Less(t, n, 40, "key %s 分到 %d 个会话，过于集中", k, n)
	}
}

// 会话所在的 key 429 冷却中：顺延到另一把 key，冷却期间稳定落在这把顺延的 key 上（不是每次随机）。
func TestDistribute_AffinityKeyCoolingMovesToStableFallback(t *testing.T) {
	setupDistributorCooldownDB(t, newRegionKeysChannel(64, constant.MultiKeyModeRandom))
	picked := make(chan [2]string, 64)
	router := newCooldownTestRouter(picked)

	require.Equal(t, http.StatusOK, sendClaudeCodeRequest(router, "session-cool"))
	first := (<-picked)[1]
	service.MarkRateLimitCooldown(nil, 64, first, cooldownTestModel)

	require.Equal(t, http.StatusOK, sendClaudeCodeRequest(router, "session-cool"))
	fallback := <-picked
	require.Equal(t, "64", fallback[0], "只有一把 key 冷却，渠道仍可用")
	require.NotEqual(t, first, fallback[1], "冷却中的 key 必须避开")
	for n := 0; n < 10; n++ {
		require.Equal(t, http.StatusOK, sendClaudeCodeRequest(router, "session-cool"))
		require.Equal(t, fallback, <-picked, "冷却期间应稳定落在同一把顺延 key 上")
	}
}

// 不带 metadata.user_id（没有亲和键）的请求保持原有选 key 行为：轮询模式依次轮转。
func TestDistribute_NoAffinityKeepsKeyRotation(t *testing.T) {
	setupDistributorCooldownDB(t, model.Channel{
		Id: 65, Name: "c65", Key: "ka\nkb\nkc",
		ChannelInfo: model.ChannelInfo{IsMultiKey: true, MultiKeySize: 3, MultiKeyMode: constant.MultiKeyModePolling},
	})
	picked := make(chan [2]string, 64)
	router := newCooldownTestRouter(picked)

	got := make([]string, 0, 6)
	for n := 0; n < 6; n++ {
		w := httptest.NewRecorder()
		body := fmt.Sprintf(`{"model":%q,"max_tokens":1,"messages":[{"role":"user","content":"hi"}]}`, cooldownTestModel)
		req := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		require.Equal(t, http.StatusOK, w.Code)
		got = append(got, (<-picked)[1])
	}
	require.Equal(t, []string{"ka", "kb", "kc", "ka", "kb", "kc"}, got)
}
