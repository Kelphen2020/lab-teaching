package model

import "time"

// ============ V1.x-17 一卡通/门禁对接 ============

// CardEvent 刷卡事件（一卡通/门禁读卡器推送）
type CardEvent struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	CardNo     string    `json:"card_no" gorm:"size:64;index"`
	StudentSno string    `json:"student_sno" gorm:"size:32;index"` // 匹配到的学号（空=未匹配）
	RoomID     *uint     `json:"room_id" gorm:"index"`
	RoomName   string    `json:"room_name" gorm:"size:128"`
	DeviceName string    `json:"device_name" gorm:"size:128"` // 读卡器/门禁设备名
	Type       string    `json:"type" gorm:"size:16"`         // in / out
	Source     string    `json:"source" gorm:"size:32"`       // card(一卡通) / access(门禁)
	CheckInID  *uint     `json:"check_in_id"`                 // 联动签到成功时回填
	Allowed    *bool     `json:"allowed"`                     // 门禁判定结果
	CreatedAt  time.Time `json:"created_at" gorm:"index"`
}

func (CardEvent) TableName() string { return "t_card_event" }

// AccessRule 门禁规则：班级在指定机房/星期/节次是否允许进入
type AccessRule struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	RoomID    uint      `json:"room_id" gorm:"index"`
	ClazzID   *uint     `json:"clazz_id" gorm:"index"` // 空=所有班级
	Weekday   int       `json:"weekday"`               // 1-7，0=不限
	Period    string    `json:"period" gorm:"size:16"` // 如 "1-2节"，空=不限
	Allow     *bool     `json:"allow" gorm:"default:true"` // 指针：false 需显式落库（bool零值会被default覆盖）
	Remark    string    `json:"remark" gorm:"size:256"`
	CreatedAt time.Time `json:"created_at"`
}

func (AccessRule) TableName() string { return "t_access_rule" }
