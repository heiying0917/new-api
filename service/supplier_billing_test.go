package service

import (
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestComputeOfficialUsd_Ratio(t *testing.T) {
	// (1000 + 1000*3)*2 = 8000 quota; usd = 8000/QuotaPerUnit
	usd := ComputeOfficialUsd(1000, 1000, 2, 3, 0, false)
	require.InDelta(t, 8000.0/common.QuotaPerUnit, usd, 1e-9)
}

func TestComputeOfficialUsd_Price(t *testing.T) {
	require.InDelta(t, 0.04, ComputeOfficialUsd(0, 0, 0, 0, 0.04, true), 1e-9)
}

func TestComputeOfficialUsd_Positive(t *testing.T) {
	require.Greater(t, ComputeOfficialUsd(100, 100, 1, 1, 0, false), 0.0)
}
