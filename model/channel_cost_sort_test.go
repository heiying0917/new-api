package model

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// resetChannelsForSort 清空 channels 表，隔离成本价排序测试（共享内存库）。
func resetChannelsForSort(t *testing.T) {
	t.Helper()
	require.NoError(t, DB.AutoMigrate(&Channel{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
}

func seedChannelWithCost(t *testing.T, id int, cost *float64) {
	t.Helper()
	require.NoError(t, DB.Create(&Channel{
		Id:        id,
		Name:      "ch_" + itoa(id),
		Key:       "key_" + itoa(id),
		Models:    "m",
		Group:     "g",
		CostPrice: cost,
	}).Error)
}

func channelIdPositions(channels []*Channel) map[int]int {
	pos := map[int]int{}
	for i, ch := range channels {
		pos[ch.Id] = i
	}
	return pos
}

// cost_price 必须被接受为合法排序字段（白名单变更的核心行为）。
func TestNewChannelSortOptions_AcceptsCostPrice(t *testing.T) {
	opts := NewChannelSortOptions("cost_price", "asc", false)
	require.Equal(t, "cost_price", opts.SortBy)
	require.Equal(t, "asc", opts.SortOrder)

	// 非法字段仍被清空（白名单防护不被破坏）。
	bad := NewChannelSortOptions("not_a_col", "asc", false)
	require.Equal(t, "", bad.SortBy)
}

// 升序：未设(NULL)与 0 都按 0 处理、排在最便宜端，正价在其后由小到大。
func TestChannelSortOptions_Apply_CostPriceAscending(t *testing.T) {
	resetChannelsForSort(t)
	zero, hi, lo := 0.0, 2.5, 1.0
	seedChannelWithCost(t, 1, nil)   // NULL
	seedChannelWithCost(t, 2, &zero) // 0
	seedChannelWithCost(t, 3, &hi)   // 2.5
	seedChannelWithCost(t, 4, &lo)   // 1.0

	var got []*Channel
	err := NewChannelSortOptions("cost_price", "asc", false).
		Apply(DB.Model(&Channel{})).Find(&got).Error
	require.NoError(t, err)
	require.Len(t, got, 4)

	pos := channelIdPositions(got)
	require.Equal(t, 3, pos[3], "cost 2.5 最贵，应排最后")
	require.Equal(t, 2, pos[4], "cost 1.0 应排倒数第二")
	require.Less(t, pos[1], pos[4], "NULL(视为0) 应排在 1.0 之前")
	require.Less(t, pos[2], pos[4], "0 应排在 1.0 之前")
}

// 降序：正价由大到小在前，未设/0 排在最便宜端（最后）。
func TestChannelSortOptions_Apply_CostPriceDescending(t *testing.T) {
	resetChannelsForSort(t)
	zero, hi, lo := 0.0, 2.5, 1.0
	seedChannelWithCost(t, 1, nil)
	seedChannelWithCost(t, 2, &zero)
	seedChannelWithCost(t, 3, &hi)
	seedChannelWithCost(t, 4, &lo)

	var got []*Channel
	err := NewChannelSortOptions("cost_price", "desc", false).
		Apply(DB.Model(&Channel{})).Find(&got).Error
	require.NoError(t, err)
	require.Len(t, got, 4)

	pos := channelIdPositions(got)
	require.Equal(t, 0, pos[3], "cost 2.5 最贵，应排最前")
	require.Equal(t, 1, pos[4], "cost 1.0 应排第二")
	require.Greater(t, pos[1], pos[4], "NULL(视为0) 应排在 1.0 之后")
	require.Greater(t, pos[2], pos[4], "0 应排在 1.0 之后")
}
