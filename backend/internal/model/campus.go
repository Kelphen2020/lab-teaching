package model

import "time"

// ============ V1.x-18 多校区管理 ============

// Campus 校区
type Campus struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Code      string    `json:"code" gorm:"uniqueIndex;size:32"` // 校区编码（分布式部署时作为数据同步键）
	Name      string    `json:"name" gorm:"size:128"`
	Address   string    `json:"address" gorm:"size:256"`
	Contact   string    `json:"contact" gorm:"size:64"`  // 负责人
	Phone     string    `json:"phone" gorm:"size:32"`
	Status    string    `json:"status" gorm:"size:16;default:正常"` // 正常/停用
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Campus) TableName() string { return "t_campus" }
