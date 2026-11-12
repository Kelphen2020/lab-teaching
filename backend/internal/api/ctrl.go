package api

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"lab-teaching/backend/internal/model"
	ws "lab-teaching/backend/internal/websocket"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ============ P0-1 控制指令真实执行：结果跟踪 + exec 白名单 ============

// CmdResult 指令执行结果（Agent ack 落库）
type CmdResult struct {
	ID        uint64    `json:"id" gorm:"primaryKey"`
	MsgID     string    `json:"msg_id" gorm:"index;size:64"`
	AgentID   string    `json:"agent_id" gorm:"index;size:64"`
	Action    string    `json:"action" gorm:"size:32"`
	Status    string    `json:"status" gorm:"size:32"`
	Detail    string    `json:"detail" gorm:"size:1024"`
	IssuedBy  string    `json:"issued_by" gorm:"size:64"`
	CreatedAt time.Time `json:"created_at"`
}

func (CmdResult) TableName() string { return "t_cmd_result" }

// cmdTracker 跟踪已下发、等待 Agent ack 的指令
var (
	cmdMu      sync.Mutex
	cmdPending = map[string]*CmdResult{} // msg_id -> 下发记录
)

// dangerousActions 危险操作：需要密码二次验证 + 双重确认
var dangerousActions = map[string]bool{
	"shutdown": true, "reboot": true, "exec": true, "agent_upgrade": true,
}

// RegisterCmdResultModels 供 main 调用做 AutoMigrate
func RegisterCmdResultModels() []interface{} {
	return []interface{}{&CmdResult{}}
}

// cmdWhitelistKey 系统设置键：exec 白名单（逗号分隔的命令前缀）
const cmdWhitelistKey = "cmd_exec_whitelist"

// GetCmdWhitelist GET /api/v1/system/cmd-whitelist（管理员）
func (h *Handler) GetCmdWhitelist(c *gin.Context) {
	v, ok := h.Repo.GetSetting(cmdWhitelistKey)
	if !ok {
		v = ""
	}
	items := make([]string, 0)
	for _, s := range strings.Split(v, "\n") {
		s = strings.TrimSpace(s)
		if s != "" {
			items = append(items, s)
		}
	}
	response.Success(c, gin.H{"whitelist": items})
}

// UpdateCmdWhitelist PUT /api/v1/system/cmd-whitelist（管理员）
func (h *Handler) UpdateCmdWhitelist(c *gin.Context) {
	var req struct {
		Whitelist []string `json:"whitelist"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	items := make([]string, 0, len(req.Whitelist))
	for _, s := range req.Whitelist {
		s = strings.TrimSpace(s)
		if s != "" {
			items = append(items, s)
		}
	}
	if err := h.Repo.UpsertSetting(cmdWhitelistKey, strings.Join(items, "\n")); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"whitelist": items})
}

// execAllowed 检查命令是否命中白名单（前缀匹配，按空白分词后的完整前缀）
func (h *Handler) execAllowed(cmd string) bool {
	v, ok := h.Repo.GetSetting(cmdWhitelistKey)
	if !ok || strings.TrimSpace(v) == "" {
		return false
	}
	cmd = strings.TrimSpace(cmd)
	for _, entry := range strings.Split(v, "\n") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if cmd == entry || strings.HasPrefix(cmd, entry+" ") {
			return true
		}
	}
	return false
}

type cmdReqP0 struct {
	AgentIDs []string               `json:"agent_ids" binding:"required"`
	Action   string                 `json:"action" binding:"required"`
	Params   map[string]interface{} `json:"params"`
	Password string                 `json:"password"` // 危险操作二次验证
}

// SendCommandV2 POST /api/v1/ctrl/command（P0-1 增强版）
// - 危险操作（shutdown/reboot/exec/agent_upgrade）要求密码二次验证
// - exec 必须命中服务端白名单，否则拒绝并审计
// - 每条下发指令登记 pending，Agent ack 回来后落库 t_cmd_result + 推送 cmd_ack 事件
func (h *Handler) SendCommandV2(c *gin.Context) {
	var req cmdReqP0
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if len(req.AgentIDs) == 0 {
		response.BadRequest(c, "agent_ids 不能为空")
		return
	}
	uid, _ := c.Get("user_id")
	uname, _ := c.Get("username")
	uidNum, _ := uid.(uint)
	operator, _ := uname.(string)
	if operator == "" {
		operator = "system"
	}

	// 危险操作：密码二次验证
	if dangerousActions[req.Action] {
		if req.Password == "" {
			response.Error(c, 428, "危险操作需要密码二次验证")
			return
		}
		var u model.User
		if err := h.Repo.DB.Where("username = ?", operator).First(&u).Error; err != nil ||
			bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)) != nil {
			h.Repo.CreateAudit(&model.Audit{UserID: uidNum, Action: "cmd_auth_fail", Target: req.Action, IP: c.ClientIP(), Detail: "operator=" + operator})
			response.Error(c, 401, "密码验证失败")
			return
		}
	}

	// exec 白名单校验（服务端强制，Agent 侧还有第二道防线）
	if req.Action == "exec" {
		cmd, _ := req.Params["cmd"].(string)
		cmd = strings.TrimSpace(cmd)
		if cmd == "" {
			response.BadRequest(c, "exec 缺少 cmd 参数")
			return
		}
		if !h.execAllowed(cmd) {
			h.Repo.CreateAudit(&model.Audit{UserID: uidNum, Action: "cmd_exec_rejected", Target: cmd, IP: c.ClientIP(), Detail: "operator=" + operator + " 命令不在白名单"})
			response.Error(c, 403, "命令不在执行白名单中，已拒绝并记录审计")
			return
		}
		req.Params["cmd"] = cmd
	}

	// 逐条下发并登记 pending
	base := time.Now().UnixNano()
	issued := make([]string, 0, len(req.AgentIDs))
	for i, agentID := range req.AgentIDs {
		msgID := fmt.Sprintf("cmd_%d_%d", base, i)
		f := ws.Frame{Type: "cmd", MsgID: msgID,
			Payload: mustJSON(map[string]interface{}{"action": req.Action, "params": req.Params})}
		if !h.Hub.SendToAgent(agentID, f) {
			// 离线/缓冲满 → 直接记录 failed
			h.Repo.DB.Create(&CmdResult{MsgID: msgID, AgentID: agentID, Action: req.Action,
				Status: "failed", Detail: "agent 离线或发送缓冲已满", IssuedBy: operator, CreatedAt: time.Now()})
			continue
		}
		cmdMu.Lock()
		cmdPending[msgID] = &CmdResult{MsgID: msgID, AgentID: agentID, Action: req.Action,
			Status: "pending", IssuedBy: operator, CreatedAt: time.Now()}
		cmdMu.Unlock()
		issued = append(issued, msgID)
	}

	// 危险操作追加显式审计（AuditLog 中间件只记通用路由）
	if dangerousActions[req.Action] {
		detail := fmt.Sprintf("operator=%s agents=%d", operator, len(req.AgentIDs))
		if req.Action == "exec" {
			detail += " cmd=" + fmt.Sprintf("%v", req.Params["cmd"])
		}
		h.Repo.CreateAudit(&model.Audit{UserID: uidNum, Action: "cmd_" + req.Action, Target: strings.Join(req.AgentIDs, ","), IP: c.ClientIP(), Detail: detail})
	}

	response.Success(c, gin.H{
		"action": req.Action, "notified": len(issued), "total": len(req.AgentIDs),
		"msg_ids": issued,
	})
}

// HandleCmdAck 处理 Agent 的 ack 帧（main.go OnEvent 中调用）
func (h *Handler) HandleCmdAck(f ws.Frame, agentID string) {
	var p struct {
		Action string `json:"action"`
		Status string `json:"status"`
		Error  string `json:"error"`
		Detail string `json:"detail"`
	}
	if err := json.Unmarshal(f.Payload, &p); err != nil {
		return
	}

	cmdMu.Lock()
	pending, ok := cmdPending[f.MsgID]
	if ok {
		delete(cmdPending, f.MsgID)
	}
	cmdMu.Unlock()

	detail := p.Detail
	if detail == "" {
		detail = p.Error
	}
	if len(detail) > 1000 {
		detail = detail[:1000]
	}

	if ok {
		pending.Status = p.Status
		pending.Detail = detail
		h.Repo.DB.Create(pending)
		// 浏览器实时推送
		if h.BrowserHub != nil {
			h.BrowserHub.Broadcast("cmd_ack", gin.H{
				"msg_id": f.MsgID, "agent_id": agentID, "action": p.Action,
				"status": p.Status, "detail": detail,
			})
		}
	} else {
		// 无 pending 记录（服务重启后到达的迟到 ack）→ 仍然落库
		h.Repo.DB.Create(&CmdResult{MsgID: f.MsgID, AgentID: agentID, Action: p.Action,
			Status: p.Status, Detail: detail, IssuedBy: "unknown", CreatedAt: time.Now()})
		if h.BrowserHub != nil {
			h.BrowserHub.Broadcast("cmd_ack", gin.H{
				"msg_id": f.MsgID, "agent_id": agentID, "action": p.Action,
				"status": p.Status, "detail": detail,
			})
		}
	}
}

// ListCmdResults GET /api/v1/ctrl/results（分页查询指令执行历史）
func (h *Handler) ListCmdResults(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	var total int64
	q := h.Repo.DB.Model(&CmdResult{})
	if aid := c.Query("agent_id"); aid != "" {
		q = q.Where("agent_id = ?", aid)
	}
	if action := c.Query("action"); action != "" {
		q = q.Where("action = ?", action)
	}
	q.Count(&total)
	var list []CmdResult
	q.Order("id desc").Offset((page - 1) * size).Limit(size).Find(&list)
	response.Success(c, response.PageData{Items: list, Total: total, Page: page, Size: size})
}
