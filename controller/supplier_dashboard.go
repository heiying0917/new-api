package controller

import (
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// SupplierDashboard 当前供应商数据看板：按天用量序列 + 渠道维度排名。
// 可选 query 参数 start_timestamp / end_timestamp（unix 秒）；默认范围为最近 7 天。
func SupplierDashboard(c *gin.Context) {
	supplierId := c.GetInt("id")
	now := time.Now().Unix()

	endTs := parseQueryInt64(c, "end_timestamp", now)
	startTs := parseQueryInt64(c, "start_timestamp", now-7*86400)

	channelIds, err := model.GetSupplierChannelIds(supplierId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	series, err := model.GetSupplierUsageSeries(channelIds, startTs, endTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	ranking, err := model.GetSupplierChannelRanking(channelIds, startTs, endTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"series":  series,
		"ranking": ranking,
	})
}

// SupplierRealtime 当前供应商实时 RPM/TPM。
func SupplierRealtime(c *gin.Context) {
	supplierId := c.GetInt("id")

	channelIds, err := model.GetSupplierChannelIds(supplierId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	stat, err := model.GetSupplierRealtimeStat(channelIds)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	common.ApiSuccess(c, gin.H{
		"rpm": stat.Rpm,
		"tpm": stat.Tpm,
	})
}

// parseQueryInt64 读取 query 参数并解析为 int64，解析失败或缺省时返回 def。
func parseQueryInt64(c *gin.Context, key string, def int64) int64 {
	raw := c.Query(key)
	if raw == "" {
		return def
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return def
	}
	return v
}
