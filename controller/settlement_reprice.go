package controller

import (
	"fmt"
	"math"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type adminRepriceUnsettledReq struct {
	DryRun *bool `json:"dry_run"`
}

// AdminRepriceUnsettled 超管工具：把所有供应商渠道的未结算日志按各渠道当前成本价重定价。
// 缺省 dry_run=true 只汇报每个渠道的差额；dry_run=false 才写入，并按渠道各记一条 action=reprice 资金账本。
// 日常改价由 UpdateChannel / SupplierUpdateChannel 的 hook 自动同步，本接口用于对齐 hook 上线前的历史数据。
func AdminRepriceUnsettled(c *gin.Context) {
	var req adminRepriceUnsettledReq
	if c.Request != nil && c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiErrorMsg(c, "invalid request body")
			return
		}
	}
	dryRun := req.DryRun == nil || *req.DryRun
	results, err := model.RepriceAllUnsettledLogsToCurrent(dryRun)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	var totalLogs int64
	var totalDelta float64
	operatorId := c.GetInt("id")
	for _, r := range results {
		totalLogs += r.Logs
		totalDelta += r.DeltaCny
		if !dryRun {
			recordRepriceLedger(operatorId, true, r, "reprice-unsettled")
		}
	}
	if !dryRun {
		common.SysLog(fmt.Sprintf("reprice-unsettled applied by admin %d: channels=%d logs=%d delta_cny=%.4f",
			operatorId, len(results), totalLogs, totalDelta))
	}
	common.ApiSuccess(c, gin.H{
		"dry_run":         dryRun,
		"channels":        results,
		"total_logs":      totalLogs,
		"total_delta_cny": totalDelta,
	})
}

// repriceChannelIfCostChanged 渠道编辑保存后调用：成本价发生变化则把该渠道未结算日志的成交价
// 同步为新价，并记一条 action=reprice 账本（official_usd=受影响官方价合计，computed_cny=应付差额）。
// 仅供应商渠道有意义；成本价未变或新价非正时直接返回。失败只记日志，不影响渠道保存结果。
func repriceChannelIfCostChanged(c *gin.Context, before, after *model.Channel, operatorIsAdmin bool) {
	if before == nil || after == nil || after.SupplierId <= 0 {
		return
	}
	oldP, newP := 0.0, 0.0
	if before.CostPrice != nil {
		oldP = *before.CostPrice
	}
	if after.CostPrice != nil {
		newP = *after.CostPrice
	}
	if !(newP > 0) || math.Abs(newP-oldP) < 1e-12 {
		return
	}
	r, err := model.RepriceUnsettledLogs(after.Id, after.SupplierId, newP, false)
	if err != nil {
		common.SysLog(fmt.Sprintf("reprice unsettled logs failed for channel %d: %v", after.Id, err))
		return
	}
	recordRepriceLedger(c.GetInt("id"), operatorIsAdmin, r, fmt.Sprintf("cost_price ¥%g → ¥%g", oldP, newP))
}

// recordRepriceLedger 无受影响日志或差额为 0 时不记账，避免噪音。
func recordRepriceLedger(operatorId int, operatorIsAdmin bool, r *model.UnsettledRepriceResult, source string) {
	if r == nil || r.Logs == 0 || math.Abs(r.DeltaCny) < 1e-9 {
		return
	}
	model.RecordSettlementLedger(&model.SettlementLedger{
		SupplierId: r.SupplierId, Action: "reprice",
		OfficialUsd: r.OfficialUsd, ComputedCNY: r.DeltaCny,
		OperatorId: operatorId, OperatorIsAdmin: operatorIsAdmin,
		Remark: fmt.Sprintf("%s: channel %d, unsettled logs=%d, ¥%.4f → ¥%.4f", source, r.ChannelId, r.Logs, r.OldCny, r.NewCny),
	})
}
