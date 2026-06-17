package controller

import (
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"

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
	model.RecordSettlementLedger(&model.SettlementLedger{
		SettlementId: s.Id, SupplierId: s.SupplierId, Action: "create",
		OfficialUsd: s.OfficialUsd, ComputedCNY: s.ComputedCNY,
		OperatorId: c.GetInt("id"), OperatorIsAdmin: false,
		SnapshotHash: model.SettlementSnapshotHash(s),
	})
	common.ApiSuccess(c, s)
}

// SupplierListSettlements returns paginated settlements for the calling supplier.
// Optional ?status=N (0=all), ?start_timestamp= & ?end_timestamp= (unix seconds, filter on 申请时间).
func SupplierListSettlements(c *gin.Context) {
	status, _ := strconv.Atoi(c.Query("status"))
	startTs, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTs, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	pageInfo := common.GetPageQuery(c)
	list, total, err := model.GetSettlementsBySupplier(c.GetInt("id"), status, startTs, endTs, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
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

// SupplierGetSettlementBreakdown returns per-channel aggregation for a settlement owned by the calling supplier.
func SupplierGetSettlementBreakdown(c *gin.Context) {
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
	rows, err := model.GetSettlementChannelBreakdown(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
}

// SupplierCancelSettlement cancels a pending settlement owned by the calling supplier.
func SupplierCancelSettlement(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.CancelSettlement(id, c.GetInt("id"), false); err != nil {
		common.ApiError(c, err)
		return
	}
	if s, e := model.GetSettlementById(id); e == nil {
		model.RecordSettlementLedger(&model.SettlementLedger{
			SettlementId: s.Id, SupplierId: s.SupplierId, Action: "cancel",
			OfficialUsd: s.OfficialUsd, ComputedCNY: s.ComputedCNY,
			OperatorId: c.GetInt("id"), OperatorIsAdmin: false,
			SnapshotHash: model.SettlementSnapshotHash(s),
		})
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

// AdminListSettlements returns paginated settlements; optional ?status=N, ?supplier_id=N
// and ?keyword= filters. keyword fuzzy-matches the supplier's username/email; when no
// supplier matches the keyword, an empty page is returned without querying settlements.
func AdminListSettlements(c *gin.Context) {
	status, _ := strconv.Atoi(c.Query("status"))
	supplierId, _ := strconv.Atoi(c.Query("supplier_id"))
	keyword := c.Query("keyword")
	pageInfo := common.GetPageQuery(c)

	var supplierIds []int
	if strings.TrimSpace(keyword) != "" {
		ids, _ := model.GetSupplierIdsByKeyword(keyword)
		if len(ids) == 0 {
			// no supplier matches the keyword → empty page, skip the settlement query
			pageInfo.SetTotal(0)
			pageInfo.SetItems([]*model.Settlement{})
			common.ApiSuccess(c, pageInfo)
			return
		}
		supplierIds = ids
	} else if supplierId > 0 {
		supplierIds = []int{supplierId}
	}

	list, total, err := model.ListSettlements(status, supplierIds, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	backfillSettlementSupplierNames(list)
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(list)
	common.ApiSuccess(c, pageInfo)
}

// backfillSettlementSupplierNames 批量按 supplier_id 回填账单的 SupplierName（admin 列表用）。
func backfillSettlementSupplierNames(list []*model.Settlement) {
	idSet := make(map[int]struct{}, len(list))
	for _, s := range list {
		if s.SupplierId > 0 {
			idSet[s.SupplierId] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return
	}
	ids := make([]int, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	names, err := model.GetUsernamesByIds(ids)
	if err != nil {
		common.SysError("failed to backfill settlement supplier names: " + err.Error())
		return
	}
	for _, s := range list {
		if name, ok := names[s.SupplierId]; ok {
			s.SupplierName = name
		}
	}
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

// AdminGetSettlementBreakdown returns per-channel aggregation for a settlement (no ownership check).
func AdminGetSettlementBreakdown(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	rows, err := model.GetSettlementChannelBreakdown(id)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, rows)
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
	// 确认即打款：账本必须落一条，即使回查失败也用手上已知数据记录（杜绝"已确认但无账本"）。
	led := &model.SettlementLedger{
		SettlementId: id, Action: "confirm",
		ActualAmount: body.ActualAmount, ActualCurrency: body.ActualCurrency,
		OperatorId: c.GetInt("id"), OperatorIsAdmin: true, Remark: body.Remark,
	}
	if s, e := model.GetSettlementById(id); e == nil {
		led.SupplierId = s.SupplierId
		led.OfficialUsd = s.OfficialUsd
		led.ComputedCNY = s.ComputedCNY
		led.SnapshotHash = model.SettlementSnapshotHash(s)
	} else {
		common.SysLog("ledger: refetch settlement after confirm failed: " + e.Error())
	}
	model.RecordSettlementLedger(led)
	common.ApiSuccess(c, nil)
}

type adminInitiateReq struct {
	SupplierId int `json:"supplier_id"`
}

// AdminInitiateSettlement 管理员为指定供应商立即发起结算单(status=已申请)，返回新单供前端打开确认弹窗。
func AdminInitiateSettlement(c *gin.Context) {
	var req adminInitiateReq
	if err := c.ShouldBindJSON(&req); err != nil || req.SupplierId <= 0 {
		common.ApiErrorMsg(c, "invalid supplier_id")
		return
	}
	// 校验目标是供应商
	u, err := model.GetUserById(req.SupplierId, false)
	if err != nil || u == nil || u.Role != common.RoleSupplierUser {
		common.ApiErrorMsg(c, "目标不是供应商")
		return
	}
	s, err := model.CreateSettlement(req.SupplierId, "manual", common.GetTimestamp())
	if err != nil {
		common.ApiError(c, err) // 含「无待结算消费」错误透传
		return
	}
	model.RecordSettlementLedger(&model.SettlementLedger{
		SettlementId: s.Id, SupplierId: s.SupplierId, Action: "create",
		OfficialUsd: s.OfficialUsd, ComputedCNY: s.ComputedCNY,
		OperatorId: c.GetInt("id"), OperatorIsAdmin: true,
		SnapshotHash: model.SettlementSnapshotHash(s),
	})
	common.ApiSuccess(c, s)
}

// AdminCancelSettlement cancels any applied settlement regardless of owner.
func AdminCancelSettlement(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if err := model.CancelSettlement(id, 0, true); err != nil {
		common.ApiError(c, err)
		return
	}
	if s, e := model.GetSettlementById(id); e == nil {
		model.RecordSettlementLedger(&model.SettlementLedger{
			SettlementId: s.Id, SupplierId: s.SupplierId, Action: "cancel",
			OfficialUsd: s.OfficialUsd, ComputedCNY: s.ComputedCNY,
			OperatorId: c.GetInt("id"), OperatorIsAdmin: true,
			SnapshotHash: model.SettlementSnapshotHash(s),
		})
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
