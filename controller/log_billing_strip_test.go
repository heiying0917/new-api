package controller

import (
	"testing"

	"github.com/QuantumNous/new-api/model"

	"github.com/stretchr/testify/assert"
)

// TestClearSupplierBillingFields 验证：普通终端用户的日志响应里，供应商成本相关字段
// (official_usd / cost_price_snapshot) 必须被清零，避免泄露平台对供应商的成本；
// 其余字段保持不变。
func TestClearSupplierBillingFields(t *testing.T) {
	logs := []*model.Log{
		{OfficialUsd: 1.5, CostPriceSnapshot: 7.0, Quota: 100, ModelName: "gpt-4"},
		{OfficialUsd: 0, CostPriceSnapshot: 0, Quota: 50, ModelName: "gpt-3.5"},
	}

	clearSupplierBillingFields(logs)

	for _, l := range logs {
		assert.Equal(t, 0.0, l.OfficialUsd)
		assert.Equal(t, 0.0, l.CostPriceSnapshot)
	}
	// 其他字段不受影响
	assert.Equal(t, 100, logs[0].Quota)
	assert.Equal(t, "gpt-4", logs[0].ModelName)
	assert.Equal(t, 50, logs[1].Quota)

	// 空切片不应 panic
	clearSupplierBillingFields(nil)
}
