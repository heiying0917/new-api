package controller

import (
	"fmt"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"

	"github.com/gin-gonic/gin"
)

type adminBackfillOfficialUsdReq struct {
	DryRun  *bool `json:"dry_run"`
	AfterId int   `json:"after_id"`
	Limit   int   `json:"limit"`
}

const (
	backfillOfficialUsdDefaultLimit = 5000
	backfillOfficialUsdMaxLimit     = 20000
)

// AdminBackfillOfficialUsd 超管一次性工具：重算供应商「未结算」消费日志的 official_usd
// （修复 2026-09-18 发现的文本路径漏算 Claude 缓存 token 导致的历史数据偏低）。
// 缺省 dry_run=true 只汇报差额不落库；显式 dry_run=false 才写入，并按供应商各记一条
// action=backfill 的 append-only 资金账本（official_usd/computed_cny 为差额）。
// 游标分页：响应 done=false 时用 next_after_id 作为下一次的 after_id 续调。
func AdminBackfillOfficialUsd(c *gin.Context) {
	var req adminBackfillOfficialUsdReq
	if c.Request != nil && c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.ShouldBindJSON(&req); err != nil {
			common.ApiErrorMsg(c, "invalid request body")
			return
		}
	}
	dryRun := req.DryRun == nil || *req.DryRun
	limit := req.Limit
	if limit <= 0 {
		limit = backfillOfficialUsdDefaultLimit
	}
	if limit > backfillOfficialUsdMaxLimit {
		limit = backfillOfficialUsdMaxLimit
	}
	if req.AfterId < 0 {
		req.AfterId = 0
	}

	res, err := model.BackfillUnsettledOfficialUsd(req.AfterId, limit, dryRun)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if !dryRun {
		operatorId := c.GetInt("id")
		for _, d := range res.PerSupplier {
			if d.Updated == 0 {
				continue
			}
			model.RecordSettlementLedger(&model.SettlementLedger{
				SupplierId: d.SupplierId, Action: "backfill",
				OfficialUsd: d.DeltaUsd, ComputedCNY: d.DeltaCny,
				OperatorId: operatorId, OperatorIsAdmin: true,
				Remark: fmt.Sprintf("official_usd backfill: logs=%d after_id=%d", d.Updated, req.AfterId),
			})
		}
		common.SysLog(fmt.Sprintf("official_usd backfill applied by admin %d: scanned=%d updated=%d unchanged=%d skipped=%d after_id=%d next_after_id=%d done=%v",
			operatorId, res.Scanned, res.Updated, res.Unchanged, res.Skipped, req.AfterId, res.NextAfterId, res.Done))
	}
	common.ApiSuccess(c, res)
}
