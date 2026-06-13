package controller

import (
	"encoding/csv"
	"fmt"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
)

// ─── Supplier endpoints ───────────────────────────────────────────────────────

// SupplierCreateSettlement creates a new manual settlement for the calling supplier.
func SupplierCreateSettlement(c *gin.Context) {
	s, err := model.CreateSettlement(c.GetInt("id"), "manual", common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, s)
}

// SupplierListSettlements returns paginated settlements for the calling supplier.
func SupplierListSettlements(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	list, total, err := model.GetSettlementsBySupplier(c.GetInt("id"), pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(list)
	common.ApiSuccess(c, pageInfo)
}

// SupplierGetSettlement returns a single settlement owned by the calling supplier.
func SupplierGetSettlement(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	s, err := model.GetSettlementById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if s.SupplierId != c.GetInt("id") {
		common.ApiErrorMsg(c, "forbidden: not your settlement")
		return
	}
	common.ApiSuccess(c, s)
}

// SupplierGetSettlementLogs returns paginated logs for a settlement owned by the calling supplier.
func SupplierGetSettlementLogs(c *gin.Context) {
	supplierId := c.GetInt("id")
	id, _ := strconv.Atoi(c.Param("id"))
	s, err := model.GetSettlementById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if s.SupplierId != supplierId {
		common.ApiErrorMsg(c, "forbidden: not your settlement")
		return
	}
	pageInfo := common.GetPageQuery(c)
	logs, total, err := model.GetSettlementLogs(id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

// SupplierCancelSettlement cancels a pending settlement owned by the calling supplier.
func SupplierCancelSettlement(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.CancelSettlement(id, c.GetInt("id"), false); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// SupplierExportSettlement streams a settlement CSV after verifying ownership.
func SupplierExportSettlement(c *gin.Context) {
	supplierId := c.GetInt("id")
	id, _ := strconv.Atoi(c.Param("id"))
	s, err := model.GetSettlementById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if s.SupplierId != supplierId {
		common.ApiErrorMsg(c, "forbidden: not your settlement")
		return
	}
	exportSettlementCSV(c, id)
}

// ─── Admin endpoints ──────────────────────────────────────────────────────────

// AdminListSettlements returns paginated settlements; optional ?status=N filter.
func AdminListSettlements(c *gin.Context) {
	status, _ := strconv.Atoi(c.Query("status"))
	pageInfo := common.GetPageQuery(c)
	list, total, err := model.ListSettlements(status, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(list)
	common.ApiSuccess(c, pageInfo)
}

// AdminGetSettlement returns a single settlement by ID.
func AdminGetSettlement(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	s, err := model.GetSettlementById(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, s)
}

// AdminGetSettlementLogs returns paginated logs for a settlement.
func AdminGetSettlementLogs(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	pageInfo := common.GetPageQuery(c)
	logs, total, err := model.GetSettlementLogs(id, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(logs)
	common.ApiSuccess(c, pageInfo)
}

type confirmSettlementBody struct {
	ActualAmount   float64 `json:"actual_amount"`
	ActualCurrency string  `json:"actual_currency"`
	SettleMethod   string  `json:"settle_method"`
	Remark         string  `json:"remark"`
}

// AdminConfirmSettlement marks an applied settlement as settled with payment details.
func AdminConfirmSettlement(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	var body confirmSettlementBody
	if err := c.ShouldBindJSON(&body); err != nil {
		common.ApiError(c, err)
		return
	}
	if err := model.ConfirmSettlement(id, body.ActualAmount, body.ActualCurrency, body.SettleMethod, body.Remark, common.GetTimestamp()); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminCancelSettlement cancels any applied settlement regardless of owner.
func AdminCancelSettlement(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.CancelSettlement(id, 0, true); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, nil)
}

// AdminExportSettlement streams a settlement CSV without ownership checks.
func AdminExportSettlement(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	exportSettlementCSV(c, id)
}

// ─── Shared CSV helper ────────────────────────────────────────────────────────

func exportSettlementCSV(c *gin.Context, settlementId int) {
	s, err := model.GetSettlementById(settlementId)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	logs, _, err := model.GetSettlementLogs(settlementId, 0, 1000000)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=settlement-%d.csv", settlementId))
	_, _ = c.Writer.WriteString("\xEF\xBB\xBF") // UTF-8 BOM for Excel
	w := csv.NewWriter(c.Writer)
	// 账单头
	_ = w.Write([]string{"账单ID", "供应商ID", "状态", "周期开始", "周期结束", "官方价(USD)", "应付(CNY)", "实付", "币种", "结算方式", "备注", "条数"})
	_ = w.Write([]string{
		strconv.Itoa(s.Id), strconv.Itoa(s.SupplierId), strconv.Itoa(s.Status),
		strconv.FormatInt(s.PeriodStart, 10), strconv.FormatInt(s.PeriodEnd, 10),
		strconv.FormatFloat(s.OfficialUsd, 'f', 6, 64), strconv.FormatFloat(s.ComputedCNY, 'f', 4, 64),
		strconv.FormatFloat(s.ActualAmount, 'f', 4, 64), s.ActualCurrency, s.SettleMethod, s.Remark,
		strconv.FormatInt(s.LogCount, 10),
	})
	_ = w.Write([]string{})
	// 明细表头
	_ = w.Write([]string{"日志ID", "时间", "模型", "渠道ID", "Prompt", "Completion", "官方价(USD)"})
	for _, lg := range logs {
		_ = w.Write([]string{
			strconv.Itoa(lg.Id), strconv.FormatInt(lg.CreatedAt, 10), lg.ModelName, strconv.Itoa(lg.ChannelId),
			strconv.Itoa(lg.PromptTokens), strconv.Itoa(lg.CompletionTokens), strconv.FormatFloat(lg.OfficialUsd, 'f', 6, 64),
		})
	}
	w.Flush()
}
