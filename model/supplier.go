package model

import (
	"errors"
	"sort"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"gorm.io/gorm"
)

// Supplier 供应商资料（1:1 user_id），与 User 解耦。
// 仅 role=RoleSupplierUser 的用户拥有该资料。
type Supplier struct {
	UserId          int    `json:"user_id" gorm:"primaryKey;autoIncrement:false"`
	Priority        int    `json:"priority" gorm:"type:int;default:0;index"` // 管理员设，优先级调度模式用
	Enabled         bool   `json:"enabled" gorm:"default:true"`
	SettlementMode  string `json:"settlement_mode" gorm:"type:varchar(16);default:'manual'"` // manual|auto
	SettlementCycle string `json:"settlement_cycle" gorm:"type:varchar(16);default:'month'"` // day|week|month
	Remark          string `json:"remark" gorm:"type:varchar(255)"`
	CreatedAt       int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt       int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func GetSupplierByUserId(userId int) (*Supplier, error) {
	var s Supplier
	err := DB.First(&s, "user_id = ?", userId).Error
	return &s, err
}

// GetSupplierIdsByKeyword 模糊匹配供应商用户名/邮箱，返回匹配的供应商用户 id 列表。
// 仅匹配 role=RoleSupplierUser 的用户（排除非供应商）。
// keyword 为空/纯空白 → 返回 nil（调用方视为"无关键词过滤"）。
func GetSupplierIdsByKeyword(keyword string) ([]int, error) {
	keyword = strings.TrimSpace(keyword)
	if keyword == "" {
		return nil, nil
	}
	like := "%" + keyword + "%"
	var ids []int
	if err := DB.Model(&User{}).
		Where("role = ?", common.RoleSupplierUser).
		Where("username LIKE ? OR email LIKE ?", like, like).
		Pluck("id", &ids).Error; err != nil {
		return nil, err
	}
	return ids, nil
}

// SupplierListItem 供应商管理页列表项 = User 基本信息 + Supplier 资料
type SupplierListItem struct {
	UserId          int    `json:"user_id"`
	Username        string `json:"username"`
	Email           string `json:"email"`
	Phone           string `json:"phone"`
	UserStatus      int    `json:"user_status"`
	Priority        int    `json:"priority"`
	Enabled         bool   `json:"enabled"`
	SettlementMode  string `json:"settlement_mode"`
	SettlementCycle string `json:"settlement_cycle"`
	Remark          string `json:"remark"`
	PendingCNY      float64 `json:"pending_cny"` // 待结算应付人民币 = GetSupplierPendingStat.PayableCNY
	PendingUsd      float64 `json:"pending_usd"` // 待结算官方价($) = GetSupplierPendingStat.OfficialUsd
	SettledCNY      float64 `json:"settled_cny"` // 已结算人民币总额 = Σ settled settlements.computed_cny
	ChannelTotal    int     `json:"channel_total"`   // 上架渠道数（全部状态，V12）
	ChannelEnabled  int     `json:"channel_enabled"` // 启用渠道数（status=enabled，V12）
}

// CreateSupplierProfile 为供应商用户创建资料，幂等。
func CreateSupplierProfile(userId int) (*Supplier, error) {
	var existing Supplier
	err := DB.First(&existing, "user_id = ?", userId).Error
	if err == nil {
		return &existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	s := &Supplier{
		UserId:          userId,
		Enabled:         true,
		SettlementMode:  "manual",
		SettlementCycle: "month",
	}
	if err := DB.Create(s).Error; err != nil {
		return nil, err
	}
	return s, nil
}

// UpdateSupplier 更新供应商资料（仅白名单字段）。
func UpdateSupplier(userId int, fields map[string]interface{}) error {
	allowed := map[string]bool{
		"priority": true, "enabled": true,
		"settlement_mode": true, "settlement_cycle": true, "remark": true,
	}
	patch := map[string]interface{}{}
	for k, v := range fields {
		if allowed[k] {
			patch[k] = v
		}
	}
	if len(patch) == 0 {
		return nil
	}
	result := DB.Model(&Supplier{}).Where("user_id = ?", userId).Updates(patch)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

// CascadeSupplierBySupplierId 根据供应商名下渠道的可用情况，级联更新供应商 enabled。
// 全部渠道不可用 → enabled=false；存在可用渠道 → enabled=true。supplierId<=0 时为 no-op。
func CascadeSupplierBySupplierId(supplierId int) error {
	if supplierId <= 0 {
		return nil
	}
	var enabledCount int64
	if err := DB.Model(&Channel{}).
		Where("supplier_id = ? AND status = ?", supplierId, common.ChannelStatusEnabled).
		Count(&enabledCount).Error; err != nil {
		return err
	}
	return DB.Model(&Supplier{}).Where("user_id = ?", supplierId).
		Update("enabled", enabledCount > 0).Error
}

func GetAllSuppliers(startIdx, num int, sortBy, sortOrder string) ([]*SupplierListItem, int64, error) {
	return querySuppliers("", startIdx, num, sortBy, sortOrder)
}

func SearchSuppliers(keyword string, startIdx, num int, sortBy, sortOrder string) ([]*SupplierListItem, int64, error) {
	return querySuppliers(keyword, startIdx, num, sortBy, sortOrder)
}

// querySuppliers 查 role=supplier 的 User，再批量合并 Supplier 资料 + 待结算/已结算统计（避免跨库 JOIN）。
//
// 排序语义：
//   - sortBy 空 / "id"：DB `id desc` + DB 分页（原行为）。
//   - sortBy ∈ {priority, pending_cny, pending_usd, settled_cny}（计算列，资料/统计均不在 users 表）：
//     取全量匹配集 → 统一填充资料(priority)与 pending/settled → 内存 sort.Slice 按方向排序 → 内存分页。
//     注：priority 存于 suppliers 表(非 users 列)，故不能在 users 查询上 ORDER BY priority。
func querySuppliers(keyword string, startIdx, num int, sortBy, sortOrder string) ([]*SupplierListItem, int64, error) {
	dir := "desc"
	if sortOrder == "asc" {
		dir = "asc"
	}
	computed := sortBy == "priority" || sortBy == "pending_cny" || sortBy == "pending_usd" || sortBy == "settled_cny"

	if !computed {
		order := "id desc"
		// DB 分页：只取本页用户，再补本页的资料 + 统计。
		users, total, err := pagedSupplierUsers(keyword, startIdx, num, order)
		if err != nil {
			return nil, 0, err
		}
		items, err := buildSupplierItems(users)
		if err != nil {
			return nil, 0, err
		}
		if err := fillSupplierStats(items); err != nil {
			return nil, 0, err
		}
		return items, total, nil
	}

	// 计算列：取全量匹配集 → 补资料 + 统计 → 内存排序 → 内存分页。
	users, total, err := pagedSupplierUsers(keyword, 0, -1, "id desc")
	if err != nil {
		return nil, 0, err
	}
	all, err := buildSupplierItems(users)
	if err != nil {
		return nil, 0, err
	}
	if err := fillSupplierStats(all); err != nil {
		return nil, 0, err
	}
	asc := dir == "asc"
	sort.Slice(all, func(i, j int) bool {
		var a, b float64
		switch sortBy {
		case "priority":
			a, b = float64(all[i].Priority), float64(all[j].Priority)
		case "pending_usd":
			a, b = all[i].PendingUsd, all[j].PendingUsd
		case "settled_cny":
			a, b = all[i].SettledCNY, all[j].SettledCNY
		default: // pending_cny
			a, b = all[i].PendingCNY, all[j].PendingCNY
		}
		if asc {
			return a < b
		}
		return a > b
	})
	if startIdx < 0 {
		startIdx = 0
	}
	if startIdx > len(all) {
		startIdx = len(all)
	}
	end := startIdx + num
	if num < 0 || end > len(all) {
		end = len(all)
	}
	return all[startIdx:end], total, nil
}

// pagedSupplierUsers 查 role=supplier 的 User（可选关键词、可选分页）。
// num < 0 表示不分页（取全量）。返回用户切片与总数。
func pagedSupplierUsers(keyword string, startIdx, num int, order string) ([]User, int64, error) {
	q := DB.Model(&User{}).Where("role = ?", common.RoleSupplierUser)
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR email LIKE ? OR phone LIKE ?", like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	q = q.Order(order).Omit("password")
	if num >= 0 {
		q = q.Limit(num).Offset(startIdx)
	}
	var users []User
	if err := q.Find(&users).Error; err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

// buildSupplierItems 把 User 切片合并 Supplier 资料，构造列表项（不含统计字段）。
func buildSupplierItems(users []User) ([]*SupplierListItem, error) {
	ids := make([]int, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.Id)
	}
	profiles := map[int]Supplier{}
	if len(ids) > 0 {
		var ss []Supplier
		if err := DB.Where("user_id IN ?", ids).Find(&ss).Error; err != nil {
			return nil, err
		}
		for _, s := range ss {
			profiles[s.UserId] = s
		}
	}
	items := make([]*SupplierListItem, 0, len(users))
	for _, u := range users {
		it := &SupplierListItem{
			UserId: u.Id, Username: u.Username, Email: u.Email,
			Phone: u.Phone, UserStatus: u.Status,
			SettlementMode: "manual", SettlementCycle: "month", Enabled: true,
		}
		if s, ok := profiles[u.Id]; ok {
			it.Priority = s.Priority
			it.Enabled = s.Enabled
			it.SettlementMode = s.SettlementMode
			it.SettlementCycle = s.SettlementCycle
			it.Remark = s.Remark
		}
		items = append(items, it)
	}
	return items, nil
}

// fillSupplierStats 一次性回填每个供应商的待结算(PendingCNY/PendingUsd)与已结算(SettledCNY)。
// 用全量聚合 map（GetAllSuppliersPendingStat/GetAllSuppliersSettledTotal）一次填充，避免 N 次单查。
func fillSupplierStats(items []*SupplierListItem) error {
	if len(items) == 0 {
		return nil
	}
	perSupplier, _, err := GetAllSuppliersPendingStat()
	if err != nil {
		return err
	}
	settledTotal, err := GetAllSuppliersSettledTotal()
	if err != nil {
		return err
	}
	channelCounts, err := GetAllSuppliersChannelCounts()
	if err != nil {
		return err
	}
	for _, it := range items {
		if p, ok := perSupplier[it.UserId]; ok {
			it.PendingCNY = p.PayableCNY
			it.PendingUsd = p.OfficialUsd
		}
		it.SettledCNY = settledTotal[it.UserId]
		if cc, ok := channelCounts[it.UserId]; ok {
			it.ChannelTotal = cc.Total
			it.ChannelEnabled = cc.Enabled
		}
	}
	return nil
}
