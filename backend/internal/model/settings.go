package model

import "time"

// ============ V1.x-21 系统设置 ============

// SystemSetting 键值对系统配置（站点标题/Logo/主题等）
type SystemSetting struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Key       string    `json:"key" gorm:"uniqueIndex;size:64"`
	Value     string    `json:"value" gorm:"type:text"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (SystemSetting) TableName() string { return "t_system_setting" }

// UserGroup 用户分组（权限组）
type UserGroup struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:64"`
	Code        string    `json:"code" gorm:"uniqueIndex;size:32"`
	Description string    `json:"description" gorm:"size:256"`
	Permissions string    `json:"permissions" gorm:"type:text"` // JSON 数组: ["*"] 或 ["dashboard","rooms",...]
	IsDefault   bool      `json:"is_default" gorm:"default:false"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (UserGroup) TableName() string { return "t_user_group" }

// GroupMemberCount 返回带成员数的分组视图
type GroupMemberCount struct {
	UserGroup
	MemberCount int64 `json:"member_count" gorm:"-"`
}
