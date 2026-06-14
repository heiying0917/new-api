package model

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/QuantumNous/new-api/common"
)

// SettlementLedger 结算资金账本：append-only，记录每次 create/confirm/cancel 的金额快照与操作者。
// 仅插入、永不更新/删除，作为资金动作的不可否认审计与对账依据（杜绝"内鬼改实付额无痕"）。
type SettlementLedger struct {
	Id              int     `json:"id"`
	SettlementId    int     `json:"settlement_id" gorm:"index"`
	SupplierId      int     `json:"supplier_id" gorm:"index"`
	Action          string  `json:"action" gorm:"type:varchar(16);index"` // create / confirm / cancel
	OfficialUsd     float64 `json:"official_usd"`
	ComputedCNY     float64 `json:"computed_cny"`
	ActualAmount    float64 `json:"actual_amount"`
	ActualCurrency  string  `json:"actual_currency" gorm:"type:varchar(8)"`
	OperatorId      int     `json:"operator_id" gorm:"index"`
	OperatorIsAdmin bool    `json:"operator_is_admin"`
	SnapshotHash    string  `json:"snapshot_hash" gorm:"type:varchar(64)"`
	Remark          string  `json:"remark" gorm:"type:varchar(255)"`
	CreatedAt       int64   `json:"created_at" gorm:"autoCreateTime"`
}

// SettlementSnapshotHash 关键字段哈希，用于事后检测账单是否被篡改。
func SettlementSnapshotHash(s *Settlement) string {
	if s == nil {
		return ""
	}
	raw := fmt.Sprintf("%d|%d|%d|%.6f|%.6f|%.6f|%s|%d",
		s.Id, s.SupplierId, s.Status, s.OfficialUsd, s.ComputedCNY, s.ActualAmount, s.ActualCurrency, s.LogCount)
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// RecordSettlementLedger append-only 写入一条账本记录。失败仅记日志、不阻断主流程
// （结算状态本身是权威，账本为审计补充）。
func RecordSettlementLedger(e *SettlementLedger) {
	if e == nil {
		return
	}
	if err := DB.Create(e).Error; err != nil {
		common.SysLog("failed to record settlement ledger: " + err.Error())
	}
}

// GetSettlementLedger 取某结算单的全部账本记录（按时间升序，审计用）。
func GetSettlementLedger(settlementId int) ([]*SettlementLedger, error) {
	var rows []*SettlementLedger
	err := DB.Where("settlement_id = ?", settlementId).Order("id asc").Find(&rows).Error
	return rows, err
}
