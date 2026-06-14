package controller

import (
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// SupplierListChannels 列出当前供应商自己的渠道（支持 keyword 搜索）
func SupplierListChannels(c *gin.Context) {
	supplierId := c.GetInt("id")
	pageInfo := common.GetPageQuery(c)
	keyword := c.Query("keyword")
	var (
		list  []*model.Channel
		total int64
		err   error
	)
	if keyword != "" {
		list, total, err = model.SearchChannelsBySupplier(supplierId, keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	} else {
		list, total, err = model.GetChannelsBySupplier(supplierId, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	}
	if err != nil {
		common.ApiError(c, err)
		return
	}
	// 为本页渠道补充未结算计费信息：official_usd（未结算官方计费）与 receivable（应收款）。
	ids := make([]int, 0, len(list))
	for _, ch := range list {
		ids = append(ids, ch.Id)
	}
	usdByChannel, _ := model.GetUnsettledOfficialUsdByChannels(ids)
	for _, ch := range list {
		usd := usdByChannel[ch.Id]
		ch.OfficialUsd = usd
		if ch.CostPrice != nil {
			ch.Receivable = usd * (*ch.CostPrice)
		} else {
			ch.Receivable = 0
		}
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(list)
	common.ApiSuccess(c, pageInfo)
}

// SupplierGetChannel 取自己的单个渠道（含 key）
func SupplierGetChannel(c *gin.Context) {
	supplierId := c.GetInt("id")
	id, _ := strconv.Atoi(c.Param("id"))
	ch, err := model.GetChannelById(id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if ch.SupplierId != supplierId {
		common.ApiErrorMsg(c, "forbidden: not your channel")
		return
	}
	common.ApiSuccess(c, ch)
}

// SupplierAddChannel 创建渠道（强制归属本人 + 成本价必填 > 0）
func SupplierAddChannel(c *gin.Context) {
	supplierId := c.GetInt("id")
	var ch model.Channel
	if err := c.ShouldBindJSON(&ch); err != nil {
		common.ApiError(c, err)
		return
	}
	if ch.CostPrice == nil || *ch.CostPrice <= 0 {
		common.ApiErrorMsg(c, "cost_price is required and must be > 0")
		return
	}
	if ch.Name == "" || ch.Key == "" || ch.Models == "" {
		common.ApiErrorMsg(c, "name, key and models are required")
		return
	}
	ch.Id = 0
	ch.SupplierId = supplierId
	ch.Status = common.ChannelStatusEnabled
	if ch.Group == "" {
		ch.Group = "default"
	}
	if err := ch.Insert(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, gin.H{"id": ch.Id})
}

// SupplierUpdateChannel 更新自己的渠道（保持归属不变）
func SupplierUpdateChannel(c *gin.Context) {
	supplierId := c.GetInt("id")
	var patch model.Channel
	if err := c.ShouldBindJSON(&patch); err != nil {
		common.ApiError(c, err)
		return
	}
	existing, err := model.GetChannelById(patch.Id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if existing.SupplierId != supplierId {
		common.ApiErrorMsg(c, "forbidden: not your channel")
		return
	}
	if patch.CostPrice != nil && *patch.CostPrice <= 0 {
		common.ApiErrorMsg(c, "cost_price must be > 0")
		return
	}
	patch.SupplierId = supplierId
	if err := patch.Update(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, nil)
}

// SupplierDeleteChannel 删除自己的渠道
func SupplierDeleteChannel(c *gin.Context) {
	supplierId := c.GetInt("id")
	id, _ := strconv.Atoi(c.Param("id"))
	existing, err := model.GetChannelById(id, true)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if existing.SupplierId != supplierId {
		common.ApiErrorMsg(c, "forbidden: not your channel")
		return
	}
	if err := existing.Delete(); err != nil {
		common.ApiError(c, err)
		return
	}
	model.InitChannelCache()
	common.ApiSuccess(c, nil)
}
