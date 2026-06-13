package controller

import (
	"strconv"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
)

var supplierHealthCheckOnce sync.Once

func supplierHealthCheckEnabled() bool {
	common.OptionMapRWMutex.RLock()
	defer common.OptionMapRWMutex.RUnlock()
	return common.OptionMap["SupplierHealthCheckEnabled"] == "true"
}

func supplierHealthCheckInterval() time.Duration {
	common.OptionMapRWMutex.RLock()
	v := common.OptionMap["SupplierHealthCheckIntervalMinutes"]
	common.OptionMapRWMutex.RUnlock()
	m := 30
	if n, err := strconv.Atoi(v); err == nil && n > 0 {
		m = n
	}
	return time.Duration(m) * time.Minute
}

// runSupplierHealthCheckOnce 探测所有供应商渠道，禁用失败的、恢复成功的，并级联供应商状态。
func runSupplierHealthCheckOnce() {
	if !supplierHealthCheckEnabled() {
		return
	}
	userID, err := resolveChannelTestUserID(nil)
	if err != nil {
		common.SysLog("supplier health check: cannot resolve test user: " + err.Error())
		return
	}
	var channels []*model.Channel
	// 仅供应商渠道，启用或自动禁用（用于检测恢复）
	model.DB.Where("supplier_id > 0 AND status IN ?", []int{common.ChannelStatusEnabled, common.ChannelStatusAutoDisabled}).Find(&channels)
	affectedSuppliers := map[int]struct{}{}
	for _, ch := range channels {
		result := testChannel(ch, userID, "", "", false)
		ok := result.localErr == nil && result.newAPIError == nil
		if ch.Status == common.ChannelStatusEnabled && !ok {
			// 失败 → 自动禁用
			reason := "supplier health check failed"
			if result.newAPIError != nil {
				reason = result.newAPIError.Error()
			} else if result.localErr != nil {
				reason = result.localErr.Error()
			}
			service.DisableChannel(*types.NewChannelError(ch.Id, ch.Type, ch.Name, ch.ChannelInfo.IsMultiKey, "", true), reason)
			affectedSuppliers[ch.SupplierId] = struct{}{}
		} else if ch.Status == common.ChannelStatusAutoDisabled && ok {
			service.EnableChannel(ch.Id, "", ch.Name)
			affectedSuppliers[ch.SupplierId] = struct{}{}
		}
	}
	for sid := range affectedSuppliers {
		_ = model.CascadeSupplierBySupplierId(sid)
	}
	common.SysLog("supplier health check finished")
}

// StartSupplierHealthCheckTask registers the periodic supplier-channel health-check background task.
// It is gated by the SupplierHealthCheckEnabled option (default false) and only runs on the master node.
func StartSupplierHealthCheckTask() {
	if !common.IsMasterNode {
		return
	}
	supplierHealthCheckOnce.Do(func() {
		go func() {
			common.SysLog("supplier health check task started")
			for {
				if supplierHealthCheckEnabled() {
					runSupplierHealthCheckOnce()
				}
				time.Sleep(supplierHealthCheckInterval())
			}
		}()
	})
}
