package model

import (
	"errors"

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

func GetAllSuppliers(startIdx, num int) ([]*SupplierListItem, int64, error) {
	return querySuppliers("", startIdx, num)
}

func SearchSuppliers(keyword string, startIdx, num int) ([]*SupplierListItem, int64, error) {
	return querySuppliers(keyword, startIdx, num)
}

// querySuppliers 查 role=supplier 的 User（分页），再批量合并 Supplier 资料（避免跨库 JOIN）。
func querySuppliers(keyword string, startIdx, num int) ([]*SupplierListItem, int64, error) {
	q := DB.Model(&User{}).Where("role = ?", common.RoleSupplierUser)
	if keyword != "" {
		like := "%" + keyword + "%"
		q = q.Where("username LIKE ? OR email LIKE ? OR phone LIKE ?", like, like, like)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var users []User
	if err := q.Order("id desc").Limit(num).Offset(startIdx).Omit("password").Find(&users).Error; err != nil {
		return nil, 0, err
	}
	ids := make([]int, 0, len(users))
	for _, u := range users {
		ids = append(ids, u.Id)
	}
	profiles := map[int]Supplier{}
	if len(ids) > 0 {
		var ss []Supplier
		if err := DB.Where("user_id IN ?", ids).Find(&ss).Error; err != nil {
			return nil, 0, err
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
	return items, total, nil
}
