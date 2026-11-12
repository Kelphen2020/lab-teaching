package api

// ============ P1 批次：消息广播 / 屏幕截图 / 桌面服务 ============
//
// P1-1a 消息广播（对标 Veyon "Message"）:
//   POST /msg/send        教师 → 指定 Agent 弹消息（notify 定时消失 / confirm 需学生按键已读）
//   POST /msg/read        Agent 上报已读回执（免登录，agent_id 自报，对齐 exam/violation 模式）
//   GET  /msg/messages    消息历史 + 已读状态
//
// P1-1b 屏幕截图（对标 Veyon "Screen capture"）:
//   POST /ctrl/command action=snapshot  教师手动抓屏
//   POST /screenshots/upload            Agent multipart 上传 JPEG（免登录）
//   GET  /screenshots                   分页查询（agent_id/source 过滤）
//   GET  /static/screenshots/:file      图片访问（免登录）
//   违纪自动截图：ReportViolation 接收 vtype=violation_screenshot 时落库
//
// P1-2 桌面服务（对标 Veyon "Open application/URL"）:
//   t_desktop_services CRUD + POST /desktop-services/:id/launch → action=open

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lab-teaching/backend/internal/model"
	ws "lab-teaching/backend/internal/websocket"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ---------------- Models ----------------

// MessageRecord 广播消息记录
type MessageRecord struct {
	ID        uint64    `json:"id" gorm:"primaryKey"`
	Title     string    `json:"title" gorm:"size:128"`
	Body      string    `json:"body" gorm:"type:text"`
	Mode      string    `json:"mode" gorm:"size:16"` // notify=定时消失 confirm=需已读回执
	Duration  int       `json:"duration" gorm:"default:15"`
	Sender    string    `json:"sender" gorm:"size:64"`
	CreatedAt time.Time `json:"created_at"`
}

func (MessageRecord) TableName() string { return "t_message" }

// MessageRead 消息已读回执
type MessageRead struct {
	ID        uint64    `json:"id" gorm:"primaryKey"`
	MessageID uint64    `json:"message_id" gorm:"index"`
	AgentID   string    `json:"agent_id" gorm:"index;size:64"`
	ReadAt    time.Time `json:"read_at"`
}

func (MessageRead) TableName() string { return "t_message_read" }

// Screenshot 屏幕截图记录
type Screenshot struct {
	ID        uint64    `json:"id" gorm:"primaryKey"`
	AgentID   string    `json:"agent_id" gorm:"index;size:64"`
	Source    string    `json:"source" gorm:"index;size:32"` // manual=教师手动 violation=违纪自动
	FileName  string    `json:"file_name" gorm:"size:128"`
	URL       string    `json:"url" gorm:"size:256"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

func (Screenshot) TableName() string { return "t_screenshot" }

// DesktopService 桌面服务（一键启动应用/网址，对标 Veyon Open Application/URL）
type DesktopService struct {
	ID          uint64    `json:"id" gorm:"primaryKey"`
	Name        string    `json:"name" gorm:"size:64"`
	ServiceType string    `json:"service_type" gorm:"size:16"` // app / url
	Target      string    `json:"target" gorm:"size:512"`
	Description string    `json:"description" gorm:"size:256"`
	Status      string    `json:"status" gorm:"size:16;default:active"`
	CreatedAt   time.Time `json:"created_at"`
}

func (DesktopService) TableName() string { return "t_desktop_service" }

// RegisterP1Models 供 main 调用做 AutoMigrate
func RegisterP1Models() []interface{} {
	return []interface{}{&MessageRecord{}, &MessageRead{}, &Screenshot{}, &DesktopService{}}
}

// ---------------- P1-1a 消息广播 ----------------

type msgSendReq struct {
	AgentIDs []string `json:"agent_ids" binding:"required"`
	Title    string   `json:"title" binding:"required"`
	Body     string   `json:"body" binding:"required"`
	Mode     string   `json:"mode"`     // notify | confirm
	Duration int      `json:"duration"` // notify 模式显示秒数
}

// SendMessage POST /api/v1/msg/send
func (h *Handler) SendMessage(c *gin.Context) {
	var req msgSendReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Mode == "" {
		req.Mode = "notify"
	}
	if req.Mode != "notify" && req.Mode != "confirm" {
		response.BadRequest(c, "mode 仅支持 notify/confirm")
		return
	}
	if req.Duration <= 0 {
		req.Duration = 15
	}
	if req.Duration > 600 {
		req.Duration = 600
	}
	if len(req.Body) > 2000 {
		response.BadRequest(c, "消息内容过长(限2000字)")
		return
	}
	uname, _ := c.Get("username")
	sender, _ := uname.(string)
	if sender == "" {
		sender = "system"
	}

	msg := &MessageRecord{
		Title: req.Title, Body: req.Body, Mode: req.Mode,
		Duration: req.Duration, Sender: sender, CreatedAt: time.Now(),
	}
	if err := h.Repo.DB.Create(msg).Error; err != nil {
		response.ServerError(c, err.Error())
		return
	}

	base := time.Now().UnixNano()
	issued := make([]string, 0, len(req.AgentIDs))
	for i, agentID := range req.AgentIDs {
		msgID := fmt.Sprintf("msg_%d_%d", base, i)
		f := ws.Frame{Type: "cmd", MsgID: msgID,
			Payload: mustJSON(map[string]interface{}{
				"action": "message",
				"params": map[string]interface{}{
					"message_id": msg.ID,
					"title":      req.Title,
					"body":       req.Body,
					"mode":       req.Mode,
					"duration":   req.Duration,
				},
			})}
		if h.Hub.SendToAgent(agentID, f) {
			issued = append(issued, agentID)
		}
	}

	h.Repo.CreateAudit(&model.Audit{
		Action: "msg_send", Target: strings.Join(req.AgentIDs, ","),
		IP: c.ClientIP(), Detail: fmt.Sprintf("sender=%s mode=%s title=%s notified=%d/%d",
			sender, req.Mode, req.Title, len(issued), len(req.AgentIDs)),
	})

	response.Success(c, gin.H{
		"message_id": msg.ID, "notified": len(issued), "total": len(req.AgentIDs),
	})
}

// MessageReadReport POST /api/v1/msg/read（Agent 免登录上报已读）
func (h *Handler) MessageReadReport(c *gin.Context) {
	var req struct {
		MessageID uint64 `json:"message_id" binding:"required"`
		AgentID   string `json:"agent_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	// 幂等：同 agent 同消息只记一条
	var n int64
	h.Repo.DB.Model(&MessageRead{}).Where("message_id = ? AND agent_id = ?", req.MessageID, req.AgentID).Count(&n)
	if n == 0 {
		h.Repo.DB.Create(&MessageRead{MessageID: req.MessageID, AgentID: req.AgentID, ReadAt: time.Now()})
	}
	if h.BrowserHub != nil {
		h.BrowserHub.Broadcast("msg_read", gin.H{"message_id": req.MessageID, "agent_id": req.AgentID})
	}
	response.Success(c, nil)
}

// ListMessages GET /api/v1/msg/messages
func (h *Handler) ListMessages(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	var total int64
	h.Repo.DB.Model(&MessageRecord{}).Count(&total)
	var list []MessageRecord
	h.Repo.DB.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)

	// 附加已读统计
	type item struct {
		MessageRecord
		ReadAgents []string `json:"read_agents"`
	}
	items := make([]item, 0, len(list))
	for _, m := range list {
		var reads []MessageRead
		h.Repo.DB.Where("message_id = ?", m.ID).Find(&reads)
		agents := make([]string, 0, len(reads))
		for _, r := range reads {
			agents = append(agents, r.AgentID)
		}
		items = append(items, item{MessageRecord: m, ReadAgents: agents})
	}
	response.Success(c, response.PageData{Items: items, Total: total, Page: page, Size: size})
}

// ---------------- P1-1b 屏幕截图 ----------------

const screenshotDir = "/tmp/lab-screenshots"

// UploadScreenshot POST /api/v1/screenshots/upload（Agent multipart 免登录上传）
// form: file(JPEG), agent_id, source(manual|violation)
func (h *Handler) UploadScreenshot(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "缺少 file 字段")
		return
	}
	defer file.Close()
	agentID := c.PostForm("agent_id")
	source := c.PostForm("source")
	if agentID == "" {
		response.BadRequest(c, "agent_id 必填")
		return
	}
	if source != "manual" && source != "violation" {
		source = "manual"
	}
	if header.Size > 10<<20 {
		response.BadRequest(c, "截图过大(限10MB)")
		return
	}
	if err := os.MkdirAll(screenshotDir, 0755); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	fname := fmt.Sprintf("%s_%s_%d.jpg", agentID, source, time.Now().UnixNano())
	fpath := filepath.Join(screenshotDir, fname)
	dst, err := os.Create(fpath)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	size, err := io.Copy(dst, file)
	dst.Close()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}

	shot := &Screenshot{
		AgentID: agentID, Source: source, FileName: fname,
		URL: "/api/v1/static/screenshots/" + fname, Size: size, CreatedAt: time.Now(),
	}
	h.Repo.DB.Create(shot)
	if h.BrowserHub != nil {
		h.BrowserHub.Broadcast("screenshot_new", gin.H{
			"id": shot.ID, "agent_id": agentID, "source": source, "url": shot.URL,
		})
	}
	response.Success(c, gin.H{"id": shot.ID, "url": shot.URL})
}

// ListScreenshots GET /api/v1/screenshots
func (h *Handler) ListScreenshots(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	q := h.Repo.DB.Model(&Screenshot{})
	if aid := c.Query("agent_id"); aid != "" {
		q = q.Where("agent_id = ?", aid)
	}
	if src := c.Query("source"); src != "" {
		q = q.Where("source = ?", src)
	}
	var total int64
	q.Count(&total)
	var list []Screenshot
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
	response.Success(c, response.PageData{Items: list, Total: total, Page: page, Size: size})
}

// ServeScreenshot GET /api/v1/static/screenshots/:file（免登录）
func (h *Handler) ServeScreenshot(c *gin.Context) {
	fname := c.Param("file")
	if strings.Contains(fname, "..") || strings.Contains(fname, "/") || strings.Contains(fname, "\\") {
		response.BadRequest(c, "非法文件名")
		return
	}
	fpath := filepath.Join(screenshotDir, fname)
	if _, err := os.Stat(fpath); err != nil {
		response.NotFound(c, "截图不存在")
		return
	}
	c.File(fpath)
}

// ---------------- P1-2 桌面服务 ----------------

// CreateDesktopService POST /api/v1/desktop-services
func (h *Handler) CreateDesktopService(c *gin.Context) {
	var s DesktopService
	if err := c.ShouldBindJSON(&s); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if s.Name == "" || s.Target == "" {
		response.BadRequest(c, "名称和目标必填")
		return
	}
	if s.ServiceType != "app" && s.ServiceType != "url" {
		response.BadRequest(c, "service_type 仅支持 app/url")
		return
	}
	if s.Status == "" {
		s.Status = "active"
	}
	if len(s.Target) > 500 {
		response.BadRequest(c, "目标过长")
		return
	}
	if err := h.Repo.DB.Create(&s).Error; err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Created(c, s)
}

// ListDesktopServices GET /api/v1/desktop-services
func (h *Handler) ListDesktopServices(c *gin.Context) {
	var list []DesktopService
	h.Repo.DB.Order("id asc").Find(&list)
	response.Success(c, list)
}

// UpdateDesktopService PUT /api/v1/desktop-services/:id
func (h *Handler) UpdateDesktopService(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效ID")
		return
	}
	var s DesktopService
	if h.Repo.DB.First(&s, id).Error != nil {
		response.NotFound(c, "服务不存在")
		return
	}
	var req struct {
		Name        *string `json:"name"`
		ServiceType *string `json:"service_type"`
		Target      *string `json:"target"`
		Description *string `json:"description"`
		Status      *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Name != nil && *req.Name != "" {
		s.Name = *req.Name
	}
	if req.ServiceType != nil {
		if *req.ServiceType != "app" && *req.ServiceType != "url" {
			response.BadRequest(c, "service_type 仅支持 app/url")
			return
		}
		s.ServiceType = *req.ServiceType
	}
	if req.Target != nil && *req.Target != "" {
		s.Target = *req.Target
	}
	if req.Description != nil {
		s.Description = *req.Description
	}
	if req.Status != nil {
		if *req.Status != "active" && *req.Status != "disabled" {
			response.BadRequest(c, "status 仅支持 active/disabled")
			return
		}
		s.Status = *req.Status
	}
	h.Repo.DB.Save(&s)
	response.Success(c, s)
}

// DeleteDesktopService DELETE /api/v1/desktop-services/:id
func (h *Handler) DeleteDesktopService(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效ID")
		return
	}
	h.Repo.DB.Delete(&DesktopService{}, id)
	response.Success(c, nil)
}

type launchReq struct {
	AgentIDs []string `json:"agent_ids" binding:"required"`
}

// LaunchDesktopService POST /api/v1/desktop-services/:id/launch
// 向选定 Agent 下发 open 指令（app → 直接启动，url → xdg-open）
func (h *Handler) LaunchDesktopService(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效ID")
		return
	}
	var s DesktopService
	if h.Repo.DB.First(&s, id).Error != nil {
		response.NotFound(c, "服务不存在")
		return
	}
	if s.Status != "active" {
		response.BadRequest(c, "服务已停用")
		return
	}
	var req launchReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	uname, _ := c.Get("username")
	operator, _ := uname.(string)
	if operator == "" {
		operator = "system"
	}

	base := time.Now().UnixNano()
	issued := make([]string, 0, len(req.AgentIDs))
	for i, agentID := range req.AgentIDs {
		msgID := fmt.Sprintf("svc_%d_%d", base, i)
		f := ws.Frame{Type: "cmd", MsgID: msgID,
			Payload: mustJSON(map[string]interface{}{
				"action": "open",
				"params": map[string]interface{}{
					"type":   s.ServiceType,
					"target": s.Target,
				},
			})}
		if h.Hub.SendToAgent(agentID, f) {
			issued = append(issued, agentID)
		}
	}
	h.Repo.CreateAudit(&model.Audit{
		Action: "svc_launch", Target: s.Name,
		IP: c.ClientIP(), Detail: fmt.Sprintf("operator=%s type=%s target=%s notified=%d/%d",
			operator, s.ServiceType, s.Target, len(issued), len(req.AgentIDs)),
	})
	response.Success(c, gin.H{"notified": len(issued), "total": len(req.AgentIDs)})
}

// 保留 json import（mustJSON 在 ctrl.go 定义）
var _ = json.Marshal
