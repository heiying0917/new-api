package controller

import (
	"strconv"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// SupplierListLogs 返回当前供应商所属渠道的分页日志。
// query 参数：p（页码，默认 1）、page_size（默认 20，clamp 1..100）、type（默认 0=全部）、
// model、start_timestamp、end_timestamp（unix 秒）。
// 隐私：返回前清空每条日志的 Username / TokenName，绝不向供应商暴露平台消费者身份。
func SupplierListLogs(c *gin.Context) {
	supplierId := c.GetInt("id")

	page, _ := strconv.Atoi(c.Query("p"))
	if page < 1 {
		page = 1
	}
	pageSize, _ := strconv.Atoi(c.Query("page_size"))
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	logType, _ := strconv.Atoi(c.Query("type"))
	modelName := c.Query("model")
	startTs := parseQueryInt64(c, "start_timestamp", 0)
	endTs := parseQueryInt64(c, "end_timestamp", 0)

	channelIds, err := model.GetSupplierChannelIds(supplierId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	logs, total, err := model.GetSupplierLogs(channelIds, logType, startTs, endTs, modelName, (page-1)*pageSize, pageSize)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 隐私：清空消费者身份字段后再返回；售价：抹掉含分组倍率的 quota 与加价字段。
	blankConsumerIdentity(logs)
	blankSellingPrice(logs)

	common.ApiSuccess(c, gin.H{
		"items":     logs,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// SupplierLogsStat 返回当前供应商所属渠道的用量统计。
// query 参数：start_timestamp、end_timestamp（unix 秒，默认无界）。
// official_usd = 时间窗内 SUM(official_usd)（供应商结算口径）；rpm/tpm = 最近 60 秒。
func SupplierLogsStat(c *gin.Context) {
	supplierId := c.GetInt("id")

	startTs := parseQueryInt64(c, "start_timestamp", 0)
	endTs := parseQueryInt64(c, "end_timestamp", 0)

	channelIds, err := model.GetSupplierChannelIds(supplierId)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	stat, err := model.SumSupplierStat(channelIds, startTs, endTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	officialUsd, err := model.SumSupplierOfficialUsd(channelIds, startTs, endTs)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	// 只返回官方价口径；quota（含分组倍率的平台售价）不再暴露给供应商。
	common.ApiSuccess(c, gin.H{
		"official_usd": officialUsd,
		"rpm":          stat.Rpm,
		"tpm":          stat.Tpm,
	})
}

// blankConsumerIdentity 清空日志中的平台消费者身份字段（用户名、令牌名）。
// 供应商只应看到自己渠道的用量，不应看到是哪个平台用户/令牌在消费。
func blankConsumerIdentity(logs []*model.Log) {
	for _, l := range logs {
		if l == nil {
			continue
		}
		l.Username = ""
		l.TokenName = ""
	}
}

// sellingPriceOtherKeys 日志 other 里会暴露平台加价/计费策略的键，供应商端一律剔除。
// 保留 model_ratio / completion_ratio / model_price / cache_* 等官方价参数与请求元数据。
var sellingPriceOtherKeys = []string{
	"group_ratio", "user_group_ratio",
	"billing_mode", "matched_tier", "billing_preference", "billing_source", "wallet_quota_deducted",
	"admin_info", "stream_status",
}

// blankSellingPrice 抹掉供应商日志中的平台售价：quota 归零，other 剔除加价/订阅/管理字段。
// official_usd 与 cost_price_snapshot 是供应商结算依据，保留。
func blankSellingPrice(logs []*model.Log) {
	for _, l := range logs {
		if l == nil {
			continue
		}
		l.Quota = 0
		if l.Other == "" {
			continue
		}
		otherMap, _ := common.StrToMap(l.Other)
		if otherMap == nil {
			continue
		}
		for _, k := range sellingPriceOtherKeys {
			delete(otherMap, k)
		}
		for k := range otherMap {
			if strings.HasPrefix(k, "subscription_") {
				delete(otherMap, k)
			}
		}
		l.Other = common.MapToJsonStr(otherMap)
	}
}
