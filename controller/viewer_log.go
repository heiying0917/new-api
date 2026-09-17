package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// blankViewerLog 观察员全站日志脱敏：抹掉消费者身份（用户名/令牌名/IP/ID）与平台成本
// （官方价/冻结成本价/admin_info），保留渠道名、模型、tokens、花费（售价对客户不敏感）。
func blankViewerLog(logs []*model.Log) {
	for _, l := range logs {
		if l == nil {
			continue
		}
		l.Username = ""
		l.TokenName = ""
		l.Ip = ""
		l.UserId = 0
		l.TokenId = 0
		l.OfficialUsd = 0
		l.CostPriceSnapshot = 0
		if l.Other == "" {
			continue
		}
		otherMap, _ := common.StrToMap(l.Other)
		if otherMap == nil {
			continue
		}
		delete(otherMap, "admin_info")
		delete(otherMap, "stream_status")
		l.Other = common.MapToJsonStr(otherMap)
	}
}

// ViewerListLogs 观察员全站日志：复用管理员查询，禁止按用户名/令牌名筛选（身份已隐藏），输出前脱敏。
func ViewerListLogs(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	requestId := c.Query("request_id")
	logs, total, err := model.GetAllLogs(logType, startTimestamp, endTimestamp, modelName, "", "", pageInfo.GetStartIdx(), pageInfo.GetPageSize(), channel, group, requestId, "")
	if err != nil {
		common.ApiError(c, err)
		return
	}
	blankViewerLog(logs)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

// ViewerLogsStat 观察员全站消耗统计（quota 为售价口径，对客户不敏感）。
func ViewerLogsStat(c *gin.Context) {
	logType, _ := strconv.Atoi(c.Query("type"))
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	modelName := c.Query("model_name")
	channel, _ := strconv.Atoi(c.Query("channel"))
	group := c.Query("group")
	stat, err := model.SumUsedQuota(logType, startTimestamp, endTimestamp, modelName, "", "", channel, group)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"quota": stat.Quota,
			"rpm":   stat.Rpm,
			"tpm":   stat.Tpm,
		},
	})
}
