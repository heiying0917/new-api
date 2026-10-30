package controller

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// seedConcurrencyFixture 复用消耗列夹具（渠道 22 属供应商 777，渠道 23 为管理员渠道），补上渠道更新需要的 abilities 表。
func seedConcurrencyFixture(t *testing.T) {
	t.Helper()
	seedConsumptionFixture(t)
	require.NoError(t, model.DB.AutoMigrate(&model.Ability{}))
}

func maxConcurrencyOf(t *testing.T, channelId int) int {
	t.Helper()
	ch, err := model.GetChannelById(channelId, false)
	require.NoError(t, err)
	return ch.GetMaxConcurrency()
}

// 管理员编辑渠道：越界并发上限被拒；合法值落库，且可改回 0（不限）。
func TestUpdateChannel_MaxConcurrencyValidatedAndPersisted(t *testing.T) {
	seedConcurrencyFixture(t)

	for _, bad := range []string{"-1", "10001"} {
		c, w := newJSONContext(1, http.MethodPut, "/api/channel/", `{"id":23,"max_concurrency":`+bad+`}`)
		UpdateChannel(c)
		require.Contains(t, w.Body.String(), `"success":false`, "max_concurrency=%s resp=%s", bad, w.Body.String())
		require.Equal(t, 0, maxConcurrencyOf(t, 23))
	}

	c, w := newJSONContext(1, http.MethodPut, "/api/channel/", `{"id":23,"max_concurrency":3}`)
	UpdateChannel(c)
	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())
	require.Equal(t, 3, maxConcurrencyOf(t, 23))

	c, w = newJSONContext(1, http.MethodPut, "/api/channel/", `{"id":23,"max_concurrency":0}`)
	UpdateChannel(c)
	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())
	require.Equal(t, 0, maxConcurrencyOf(t, 23), "改回 0（不限）必须落库")
}

// 供应商新增 / 编辑自己的渠道同样可设并发上限，并做同样的范围校验。
func TestSupplierChannel_MaxConcurrencyValidatedAndPersisted(t *testing.T) {
	seedConcurrencyFixture(t)

	c, w := newJSONContext(777, http.MethodPut, "/api/supplier/channel/", `{"id":22,"max_concurrency":-1}`)
	SupplierUpdateChannel(c)
	require.Contains(t, w.Body.String(), `"success":false`, "resp=%s", w.Body.String())
	require.Equal(t, 0, maxConcurrencyOf(t, 22))

	c, w = newJSONContext(777, http.MethodPut, "/api/supplier/channel/", `{"id":22,"max_concurrency":8}`)
	SupplierUpdateChannel(c)
	require.Contains(t, w.Body.String(), `"success":true`, "resp=%s", w.Body.String())
	require.Equal(t, 8, maxConcurrencyOf(t, 22))

	c, w = newJSONContext(777, http.MethodPost, "/api/supplier/channel/", `{"name":"n","key":"k","models":"m","cost_price":1.5,"max_concurrency":10001}`)
	SupplierAddChannel(c)
	require.Contains(t, w.Body.String(), `"success":false`, "resp=%s", w.Body.String())
}

// 渠道列表回填当前并发：管理员与供应商可见「当前 / 上限」；观察员白名单 DTO 不暴露。
func TestChannelList_BackfillsCurrentConcurrency(t *testing.T) {
	seedConcurrencyFixture(t)
	two := 2
	require.NoError(t, (&model.Channel{Id: 22, MaxConcurrency: &two}).Update())

	held := make([]*gin.Context, 0, 3)
	for _, id := range []int{22, 22, 23} {
		hc, _ := gin.CreateTestContext(httptest.NewRecorder())
		require.True(t, service.TryAcquireChannelSlot(hc, &model.Channel{Id: id, MaxConcurrency: &two}))
		held = append(held, hc)
	}
	t.Cleanup(func() {
		for _, hc := range held {
			service.ReleaseChannelSlot(hc)
		}
	})

	c, w := newGetContext(1, "/api/channel/?p=1&page_size=20")
	GetAllChannels(c)
	items := listItemsById(t, w)
	require.EqualValues(t, 2, items[22]["current_concurrency"])
	require.EqualValues(t, 2, items[22]["max_concurrency"])
	require.EqualValues(t, 1, items[23]["current_concurrency"])
	require.EqualValues(t, 0, items[23]["max_concurrency"])

	c, w = newGetContext(777, "/api/supplier/channel/?p=1&page_size=20")
	SupplierListChannels(c)
	items = listItemsById(t, w)
	require.EqualValues(t, 2, items[22]["current_concurrency"])
	require.NotContains(t, items, 23)

	c, w = newGetContext(1, "/api/channel/search?keyword=op5")
	SearchChannels(c)
	items = listItemsById(t, w)
	require.EqualValues(t, 2, items[22]["current_concurrency"])

	c, w = newGetContext(3, "/api/viewer/channel/?p=1&page_size=20")
	ViewerListChannels(c)
	items = listItemsById(t, w)
	require.NotContains(t, items[22], "current_concurrency")
	require.NotContains(t, items[22], "max_concurrency")
}
