package model

import "time"

// ============ 系统域 ============
type User struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	Username     string    `json:"username" gorm:"uniqueIndex;size:64"`
	PasswordHash string    `json:"-" gorm:"size:256"`
	RealName     string    `json:"real_name" gorm:"size:64"`
	Role         string    `json:"role" gorm:"size:32;default:teacher"`
	Status       string    `json:"status" gorm:"size:16;default:active"`
	GroupID      uint      `json:"group_id" gorm:"default:0"` // V1.x-21 所属权限分组(0=未分组)
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (User) TableName() string { return "t_user" }

type Audit struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id"`
	Action    string    `json:"action" gorm:"size:128"`
	Target    string    `json:"target" gorm:"size:256"`
	IP        string    `json:"ip" gorm:"size:64"`
	Detail    string    `json:"detail"`
	CreatedAt time.Time `json:"created_at"`
}

func (Audit) TableName() string { return "t_audit" }

// ============ 资产域 ============
type Room struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CampusID  *uint     `json:"campus_id" gorm:"index"` // V1.x-18 所属校区（空=主校区/未分配）
	Building  string    `json:"building" gorm:"size:64"`
	Floor     string    `json:"floor" gorm:"size:16"`
	Name      string    `json:"name" gorm:"uniqueIndex;size:128"`
	SeatCount int       `json:"seat_count"`
	NetType   string    `json:"net_type" gorm:"size:16;default:千兆"`
	BootMode  string    `json:"boot_mode" gorm:"size:16;default:混合"`
	Status    string    `json:"status" gorm:"size:16;default:正常"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Room) TableName() string { return "t_room" }

type Device struct {
	ID           uint       `json:"id" gorm:"primaryKey"`
	AssetCode    string     `json:"asset_code" gorm:"uniqueIndex;size:64"`
	Category     string     `json:"category" gorm:"size:32;default:电脑"`
	Brand        string     `json:"brand" gorm:"size:64"`
	Model        string     `json:"model" gorm:"size:64"`
	CpuArch      string     `json:"cpu_arch" gorm:"size:16;default:x86_64"`
	OS           string     `json:"os" gorm:"size:64"`
	MAC          string     `json:"mac" gorm:"size:32"`
	IP           string     `json:"ip" gorm:"size:64"`
	Status       string     `json:"status" gorm:"size:16;default:在用"`
	PurchaseDate *time.Time `json:"purchase_date"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (Device) TableName() string { return "t_device" }

type Seat struct {
	ID       uint   `json:"id" gorm:"primaryKey"`
	RoomID   uint   `json:"room_id"`
	SeatNo   string `json:"seat_no" gorm:"size:16"`
	DeviceID *uint  `json:"device_id"`
	BootMode string `json:"boot_mode" gorm:"size:16;default:无盘"`
	Status   string `json:"status" gorm:"size:16;default:空闲"`
}

func (Seat) TableName() string { return "t_seat" }

// ============ 教学域 ============
type Course struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:128"`
	Credit    float64   `json:"credit"`
	TeacherID *uint     `json:"teacher_id"`
	CreatedAt time.Time `json:"created_at"`
}

func (Course) TableName() string { return "t_course" }

type Clazz struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Name       string    `json:"name" gorm:"size:128"`
	Department string    `json:"department" gorm:"size:128"`
	CreatedAt  time.Time `json:"created_at"`
}

func (Clazz) TableName() string { return "t_clazz" }

type Student struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Sno       string    `json:"sno" gorm:"uniqueIndex;size:32"`
	Name      string    `json:"name" gorm:"size:64"`
	ClazzID   *uint     `json:"clazz_id"`
	CardNo    string    `json:"card_no" gorm:"size:64"`
	FaceID    string    `json:"face_id" gorm:"size:128"`
	CreatedAt time.Time `json:"created_at"`
}

func (Student) TableName() string { return "t_student" }

type Schedule struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CourseID  uint      `json:"course_id"`
	ClazzID   uint      `json:"clazz_id"`
	RoomID    uint      `json:"room_id"`
	TeacherID *uint     `json:"teacher_id"` // V1.x-16: 教务同步课表时教师可缺省
	Weekday   int       `json:"weekday"`
	Period    string    `json:"period" gorm:"size:16"`
	WeekRange string    `json:"week_range" gorm:"size:32"`
	Status    string    `json:"status" gorm:"size:16;default:正常"`
	CreatedAt time.Time `json:"created_at"`
}

func (Schedule) TableName() string { return "t_schedule" }

// ============ Agent 管控域 ============
type AgentSession struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	SeatID        *uint      `json:"seat_id"`
	DeviceID      *uint      `json:"device_id"`
	AgentID       string     `json:"agent_id" gorm:"uniqueIndex;size:64"`
	Version       string     `json:"version" gorm:"size:32"`
	IP            string     `json:"ip" gorm:"size:64"`
	MAC           string     `json:"mac" gorm:"size:64"`
	OS            string     `json:"os" gorm:"size:64"`
	Arch          string     `json:"arch" gorm:"size:16"`
	BootMode      string     `json:"boot_mode" gorm:"size:16"`
	Online        bool       `json:"online"`
	LastHeartbeat *time.Time `json:"last_heartbeat"`
	CreatedAt     time.Time  `json:"created_at"`
}

func (AgentSession) TableName() string { return "t_agent_session" }

type BroadcastTask struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	RoomID     uint       `json:"room_id"`
	TeacherID  uint       `json:"teacher_id"`
	Scope      string     `json:"scope" gorm:"size:32;default:全班"`
	Targets    string     `json:"targets"`
	McastAddr  string     `json:"mcast_addr" gorm:"size:32"`
	Status     string     `json:"status" gorm:"size:16;default:进行中"`
	RecordPath string     `json:"record_path" gorm:"size:256"`
	StartAt    time.Time  `json:"start_at"`
	EndAt      *time.Time `json:"end_at"`
}

func (BroadcastTask) TableName() string { return "t_broadcast_task" }

type DiskImage struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:128"`
	OS        string    `json:"os" gorm:"size:64"`
	Arch      string    `json:"arch" gorm:"size:16"`
	Path      string    `json:"path" gorm:"size:256"`
	Version   string    `json:"version" gorm:"size:32"`
	Status    string    `json:"status" gorm:"size:16;default:可用"`
	CreatedAt time.Time `json:"created_at"`
}

func (DiskImage) TableName() string { return "t_disk_image" }

type Policy struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	RoomID    *uint     `json:"room_id"`
	USB       string    `json:"usb" gorm:"size:16;default:允许"`
	Print     string    `json:"print" gorm:"size:16;default:允许"`
	NetMode   string    `json:"net_mode" gorm:"size:16;default:开放"`
	URLList   string    `json:"url_list"`
	TimeRange string    `json:"time_range" gorm:"size:64"`
	CreatedAt time.Time `json:"created_at"`
}

func (Policy) TableName() string { return "t_policy" }

// ============ 考试域 ============
type Question struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Type       string    `json:"type" gorm:"size:16"` // single/multi/judge/fill
	Content    string    `json:"content"`
	Options    string    `json:"options"`   // JSON: ["A. xxx","B. xxx"]
	Answer     string    `json:"answer"`    // "A" / "AB" / "true" / 填空文本
	Analysis   string    `json:"analysis"`
	Difficulty int       `json:"difficulty"` // 1-5
	Score      float64   `json:"score"`
	Knowledge  string    `json:"knowledge" gorm:"size:128"`
	Version    int       `json:"version"`
	CreatedAt  time.Time `json:"created_at"`
}

func (Question) TableName() string { return "t_question" }

type Paper struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:128"`
	TotalScore  float64   `json:"total_score"`
	Duration    int       `json:"duration"` // 分钟
	Strategy    string    `json:"strategy" gorm:"size:32;default:固定"` // 固定/随机
	Status      string    `json:"status" gorm:"size:16;default:草稿"`   // 草稿/已发布
	CreatorID   uint      `json:"creator_id"`
	CreatedAt   time.Time `json:"created_at"`
}

func (Paper) TableName() string { return "t_paper" }

type PaperQuestion struct {
	ID        uint    `json:"id" gorm:"primaryKey"`
	PaperID   uint    `json:"paper_id" gorm:"index"`
	QuestionID uint   `json:"question_id"`
	SortOrder int     `json:"sort_order"`
	Score     float64 `json:"score"` // 本题在本卷中的分值
}

func (PaperQuestion) TableName() string { return "t_paper_question" }

type Exam struct {
	ID         uint       `json:"id" gorm:"primaryKey"`
	Name       string     `json:"name" gorm:"size:128"`
	PaperID    uint       `json:"paper_id"`
	RoomID     uint       `json:"room_id"`
	TeacherID  uint       `json:"teacher_id"`
	Strategy   string     `json:"strategy" gorm:"size:32;default:固定"`
	Status     string     `json:"status" gorm:"size:16;default:草稿"` // 草稿/进行中/已结束
	AccessCode string     `json:"access_code" gorm:"size:16;index"`  // 学生加入考试码
	StartAt    *time.Time `json:"start_at"`
	EndAt      *time.Time `json:"end_at"`
	AntiCheat  bool       `json:"anti_cheat" gorm:"default:true"` // 防切屏
	CreatedAt  time.Time  `json:"created_at"`
}

func (Exam) TableName() string { return "t_exam" }

type ExamAnswer struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	ExamID     uint      `json:"exam_id" gorm:"index"`
	AgentID    string    `json:"agent_id" gorm:"size:64;index"`
	StudentSno string    `json:"student_sno" gorm:"size:32"`
	QuestionID uint      `json:"question_id"`
	Answer     string    `json:"answer"`
	Correct    *bool     `json:"correct"`
	Score      float64   `json:"score"`
	SubmittedAt time.Time `json:"submitted_at"`
}

func (ExamAnswer) TableName() string { return "t_exam_answer" }

type ExamViolation struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	ExamID    uint      `json:"exam_id" gorm:"index"`
	AgentID   string    `json:"agent_id" gorm:"size:64;index"`
	Type      string    `json:"type" gorm:"size:32"` // switch_window/usb_insert/copy_paste
	Detail    string    `json:"detail" gorm:"size:512"`
	CreatedAt time.Time `json:"created_at"`
}

func (ExamViolation) TableName() string { return "t_exam_violation" }

// ============ 文件收发域 ============
type FileTask struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	Name       string    `json:"name" gorm:"size:256"`       // 文件名
	Size       int64     `json:"size"`                       // 字节数
	Direction  string    `json:"direction" gorm:"size:16"`   // distribute(教师→学生) | collect(学生→教师)
	StoragePath string   `json:"storage_path" gorm:"size:512"` // 服务端存储路径
	RoomID     uint      `json:"room_id"`
	CreatorID  uint      `json:"creator_id"`
	Status     string    `json:"status" gorm:"size:16;default:pending"` // pending/distributing/done/failed
	Total      int       `json:"total"`                      // 目标终端数
	Completed  int       `json:"completed"`                  // 已完成数
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (FileTask) TableName() string { return "t_file_task" }

// AgentLog Agent 上传日志
type AgentLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	AgentID   string    `json:"agent_id" gorm:"index;size:64"`
	Level     string    `json:"level" gorm:"size:16"` // info/warn/error
	Content   string    `json:"content" gorm:"type:text"`
	CreatedAt time.Time `json:"created_at"`
}

func (AgentLog) TableName() string { return "t_agent_log" }

type FileProgress struct {
	ID       uint      `json:"id" gorm:"primaryKey"`
	TaskID   uint      `json:"task_id" gorm:"index"`
	AgentID  string    `json:"agent_id" gorm:"size:64;index"`
	Status   string    `json:"status" gorm:"size:16;default:pending"` // pending/transferring/done/failed
	Percent  int       `json:"percent" gorm:"default:0"`
	Error    string    `json:"error" gorm:"size:256"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (FileProgress) TableName() string { return "t_file_progress" }

// ============ 远程控制域 ============
type RemoteSession struct {
	ID        uint       `json:"id" gorm:"primaryKey"`
	AgentID   string     `json:"agent_id" gorm:"size:64;index"`
	TeacherID uint       `json:"teacher_id"`
	Status    string     `json:"status" gorm:"size:16;default:active"` // active / stopped
	StartedAt time.Time  `json:"started_at"`
	StoppedAt *time.Time `json:"stopped_at"`
}

func (RemoteSession) TableName() string { return "t_remote_session" }
