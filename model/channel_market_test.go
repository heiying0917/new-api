package model

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestGetGroupMarketPrices(t *testing.T) {
	require.NoError(t, DB.AutoMigrate(&Channel{}))
	require.NoError(t, DB.Exec("DELETE FROM channels").Error)
	c1, c2, c3 := 2.5, 2.0, 3.0
	require.NoError(t, DB.Create(&Channel{Id: 10, Name: "a", Key: "k1", CostPrice: &c1, Group: "claude,gpt", Status: common.ChannelStatusEnabled, Models: "m"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 11, Name: "b", Key: "k2", CostPrice: &c2, Group: "claude", Status: common.ChannelStatusEnabled, Models: "m"}).Error)
	require.NoError(t, DB.Create(&Channel{Id: 12, Name: "c", Key: "k3", CostPrice: &c3, Group: "gpt", Status: common.ChannelStatusManuallyDisabled, Models: "m"}).Error) // disabled excluded
	m, err := GetGroupMarketPrices()
	require.NoError(t, err)
	require.InDelta(t, 2.0, m["claude"], 1e-9) // min(2.5,2.0)
	require.InDelta(t, 2.5, m["gpt"], 1e-9)    // only ch1 (ch3 disabled)
}
