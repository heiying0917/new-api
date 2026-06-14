package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

// TestBlankConsumerIdentity verifies suppliers never see platform-consumer identity.
func TestBlankConsumerIdentity(t *testing.T) {
	logs := []*model.Log{
		{Username: "alice", TokenName: "tok-a", ModelName: "gpt-4", ChannelId: 1},
		{Username: "bob", TokenName: "tok-b", ModelName: "gpt-3.5", ChannelId: 2},
	}
	blankConsumerIdentity(logs)
	for _, l := range logs {
		require.Equal(t, "", l.Username, "username must be blanked")
		require.Equal(t, "", l.TokenName, "token name must be blanked")
	}
	// Non-identity fields preserved.
	require.Equal(t, "gpt-4", logs[0].ModelName)
	require.Equal(t, 1, logs[0].ChannelId)
	require.Equal(t, "gpt-3.5", logs[1].ModelName)
	require.Equal(t, 2, logs[1].ChannelId)

	// nil-safe.
	require.NotPanics(t, func() { blankConsumerIdentity(nil) })
}
