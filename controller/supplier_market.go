package controller

import (
	"time"

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

// SupplierOverview 当前供应商概览聚合：待结算、已结算、渠道状态、今日用量、竞价梯队。
func SupplierOverview(c *gin.Context) {
	supplierId := c.GetInt("id")
	now := time.Now().Unix()

	pending, err := model.GetSupplierPendingStat(supplierId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	settled, err := model.GetSupplierSettledStats(supplierId, now)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	available, unavailable, err := model.GetSupplierChannelStatusCounts(supplierId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	channelIds, err := model.GetSupplierChannelIds(supplierId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	requests, tokens, err := model.GetSupplierTodayUsage(channelIds, now)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	bids, err := model.GetSupplierMarketBids(supplierId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"pending": pending,
		"settled": settled,
		"channels": gin.H{
			"available":   available,
			"unavailable": unavailable,
		},
		"today_usage": gin.H{
			"requests": requests,
			"tokens":   tokens,
		},
		"bids": bids,
	})
}
