package service

import (
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func keyAffinityTestContext(cacheKey string) *gin.Context {
	gin.SetMode(gin.TestMode)
	return buildChannelAffinityTemplateContextForTest(channelAffinityMeta{
		CacheKey:   cacheKey,
		TTLSeconds: 3600,
		RuleName:   "claude cli trace",
	})
}

func pickKeyForTest(t *testing.T, c *gin.Context, ch *model.Channel) string {
	t.Helper()
	key, idx, apiErr := PickChannelKey(c, ch, "m")
	require.Nil(t, apiErr)
	require.Equal(t, ch.GetKeys()[idx], key)
	return key
}

// 命中亲和规则的请求在多 key 渠道内按亲和键稳定选 key；所在 key 429 冷却时顺延到另一把，
// 冷却到期后回到原 key（原 region 上的缓存可能仍有效），而不是留在顺延的 key 上。
func TestPickChannelKey_AffinityStickyAcrossCooldown(t *testing.T) {
	seedSelectChannels(t, true, selectSeed{id: 1, priority: 10, key: strings.Join([]string{"r0", "r1", "r2", "r3", "r4", "r5", "r6", "r7", "r8"}, "\n")})
	now := time.Now()
	setCooldownNow(t, func() time.Time { return now })
	full, err := model.GetChannelById(1, true)
	require.NoError(t, err)

	c := keyAffinityTestContext(channelAffinityCacheNamespace + ":claude cli trace:g:session-a")
	first := pickKeyForTest(t, c, full)
	for i := 0; i < 20; i++ {
		require.Equal(t, first, pickKeyForTest(t, c, full))
	}

	markChannelCooldown(1, first, "m", 5*time.Second)
	fallback := pickKeyForTest(t, c, full)
	require.NotEqual(t, first, fallback)
	for i := 0; i < 20; i++ {
		require.Equal(t, fallback, pickKeyForTest(t, c, full), "冷却期间稳定落在顺延的 key 上")
	}

	now = now.Add(5*time.Second + time.Millisecond)
	require.Equal(t, first, pickKeyForTest(t, c, full), "冷却到期回到原 key")
}

// 不同亲和键在同一渠道内分散到不同 key；没有亲和上下文时（未命中规则 / 渠道测试等）按渠道原模式选 key。
func TestPickChannelKey_DifferentAffinitiesSpreadAndNoAffinityUnchanged(t *testing.T) {
	seedSelectChannels(t, true, selectSeed{id: 1, priority: 10, key: "r0\nr1\nr2\nr3"})
	full, err := model.GetChannelById(1, true)
	require.NoError(t, err)

	used := map[string]bool{}
	for _, s := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j", "k", "l"} {
		used[pickKeyForTest(t, keyAffinityTestContext(channelAffinityCacheNamespace+":rule:g:"+s), full)] = true
	}
	require.Greater(t, len(used), 1, "不同会话不能全部压到同一把 key")

	plain := gin.Context{}
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		seen[pickKeyForTest(t, &plain, full)] = true
		seen[pickKeyForTest(t, nil, full)] = true
	}
	require.Len(t, seen, 4, "没有亲和上下文时仍按随机模式在全部 key 间选")
}
