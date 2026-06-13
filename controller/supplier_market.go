package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// SupplierPending 当前供应商待结算统计
func SupplierPending(c *gin.Context) {
	stat, err := model.GetSupplierPendingStat(c.GetInt("id"))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, stat)
}

// SupplierMarketPrice 各分组市场最低价（供应商+管理员可见）
func SupplierMarketPrice(c *gin.Context) {
	m, err := model.GetGroupMarketPrices()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, m)
}
