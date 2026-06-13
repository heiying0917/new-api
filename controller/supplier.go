package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

// GetAllSuppliers 供应商列表（分页）。仅超管。
func GetAllSuppliers(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.GetAllSuppliers(pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

// SearchSuppliers 关键词搜索供应商。仅超管。
func SearchSuppliers(c *gin.Context) {
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)
	items, total, err := model.SearchSuppliers(keyword, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(items)
	common.ApiSuccess(c, pageInfo)
}

type updateSupplierRequest struct {
	UserId          int     `json:"user_id"`
	Priority        *int    `json:"priority"`
	Enabled         *bool   `json:"enabled"`
	SettlementMode  string  `json:"settlement_mode"`
	SettlementCycle string  `json:"settlement_cycle"`
	Remark          *string `json:"remark"`
}

// UpdateSupplier 更新供应商资料（优先级/启用/结算方式/周期/备注）。仅超管。
func UpdateSupplier(c *gin.Context) {
	var req updateSupplierRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		common.ApiError(c, err)
		return
	}
	if req.UserId == 0 {
		common.ApiErrorMsg(c, "user_id is required")
		return
	}
	// 校验枚举值（Fix B）
	if req.SettlementMode != "" && req.SettlementMode != "manual" && req.SettlementMode != "auto" {
		common.ApiErrorMsg(c, "invalid settlement_mode")
		return
	}
	if req.SettlementCycle != "" && req.SettlementCycle != "day" && req.SettlementCycle != "week" && req.SettlementCycle != "month" {
		common.ApiErrorMsg(c, "invalid settlement_cycle")
		return
	}
	fields := map[string]interface{}{}
	if req.Priority != nil {
		fields["priority"] = *req.Priority
	}
	if req.Enabled != nil {
		fields["enabled"] = *req.Enabled
	}
	if req.SettlementMode != "" {
		fields["settlement_mode"] = req.SettlementMode
	}
	if req.SettlementCycle != "" {
		fields["settlement_cycle"] = req.SettlementCycle
	}
	if req.Remark != nil {
		fields["remark"] = *req.Remark
	}
	// 无任何可更新字段时显式报错，避免静默成功（Fix A）
	if len(fields) == 0 {
		common.ApiErrorMsg(c, "no fields to update")
		return
	}
	// 确保资料存在（幂等），避免对老供应商更新时报 not found
	if _, err := model.CreateSupplierProfile(req.UserId); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.UpdateSupplier(req.UserId, fields); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}
