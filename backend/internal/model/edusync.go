package model

import "time"

// ============ V1.x-16 教务系统对接 ============

// EduSyncLog 教务同步日志（审计）
type EduSyncLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Source    string    `json:"source" gorm:"size:32;index"`    // 教务系统标识
	Action    string    `json:"action" gorm:"size:32"`          // students/courses/classes/schedules/scores
	Total     int       `json:"total"`                          // 接收记录数
	Succeeded int       `json:"succeeded"`                      // 成功数
	Failed    int       `json:"failed"`                         // 失败数
	Detail    string    `json:"detail" gorm:"size:1024"`        // 失败详情
	IP        string    `json:"ip" gorm:"size:64"`
	CreatedAt time.Time `json:"created_at"`
}

func (EduSyncLog) TableName() string { return "t_edu_sync_log" }

// EduSyncConfig 教务对接配置（每教务系统一条）
type EduSyncConfig struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Name       string    `json:"name" gorm:"size:64;uniqueIndex"` // 对接名称（如"正方教务"）
	SyncKey    string    `json:"sync_key" gorm:"size:64;index"`   // 推送鉴权密钥（X-Sync-Key 头）
	CallbackURL string   `json:"callback_url" gorm:"size:256"`    // 成绩回推地址（可选）
	Enabled    bool      `json:"enabled" gorm:"default:true"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (EduSyncConfig) TableName() string { return "t_edu_sync_config" }
