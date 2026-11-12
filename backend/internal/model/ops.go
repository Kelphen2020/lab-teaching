package model

import "time"

// ============ V1.x-20 运维工具：Agent 批量升级 ============

// UpgradeTask Agent 批量升级任务
type UpgradeTask struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:128"`
	Version     string    `json:"version" gorm:"size:32;index"` // 目标 Agent 版本
	Arch        string    `json:"arch" gorm:"size:32"`          // amd64/arm64/loong64
	RoomIDs     string    `json:"room_ids" gorm:"size:512"`     // 目标机房 ID 逗号分隔
	GrayPct     int       `json:"gray_pct"`                     // 灰度比例 0-100
	Status      string    `json:"status" gorm:"size:16;index"`  // pending/rolling/success/failed/rolled_back
	Total       int       `json:"total"`
	Succeeded   int       `json:"succeeded"`
	Failed      int       `json:"failed"`
	Remark      string    `json:"remark" gorm:"size:256"`
	CreatedBy   uint      `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (UpgradeTask) TableName() string { return "t_upgrade_task" }

// UpgradeNode 单台终端升级结果
type UpgradeNode struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	TaskID    uint      `json:"task_id" gorm:"index"`
	SeatID    uint      `json:"seat_id" gorm:"index"`
	AgentID   string    `json:"agent_id" gorm:"size:64"`
	OldVer    string    `json:"old_ver" gorm:"size:32"`
	NewVer    string    `json:"new_ver" gorm:"size:32"`
	Status    string    `json:"status" gorm:"size:16;index"` // pending/upgrading/success/failed/rolled_back
	Error     string    `json:"error" gorm:"size:512"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (UpgradeNode) TableName() string { return "t_upgrade_node" }
