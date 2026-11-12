package api

// P2 批次（对标 Veyon wk 9-12）：
//   P2-1 监控墙：thumb_start/thumb_stop 缩略图帧 + WallHub 广播（wall_hub.go）
//   P2-2 WOL 远程开机：magic packet UDP 广播
//   P2-3 定时电源任务：t_power_task + 后台调度器
//   P2-4 会话控制：session_logoff 指令（前端经 /ctrl/command 下发）
//   P2-5 作业收集：t_collect_task + agent collect_files 打包上传

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"lab-teaching/backend/internal/model"
	ws "lab-teaching/backend/internal/websocket"
	"lab-teaching/backend/pkg/response"
)

// currentUser 取当前登录用户（operator 用户名, uid 数字 ID）
func currentUser(c *gin.Context) (string, uint) {
	uid, _ := c.Get("user_id")
	uname, _ := c.Get("username")
	uidNum, _ := uid.(uint)
	operator, _ := uname.(string)
	if operator == "" {
		operator = "system"
	}
	return operator, uidNum
}

// ============ P2 模型 ============

// PowerTask 定时电源任务（延时关机/开机/重启）
type PowerTask struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Name      string    `json:"name" gorm:"size:128"`
	Action    string    `json:"action" gorm:"size:16"`                  // shutdown | reboot | wol
	AgentIDs  string    `json:"agent_ids" gorm:"type:text"`             // JSON 数组
	ExecuteAt time.Time `json:"execute_at"`
	Status    string    `json:"status" gorm:"size:16;default:pending"` // pending | done | failed | cancelled
	Detail    string    `json:"detail" gorm:"size:255"`
	CreatedBy string    `json:"created_by" gorm:"size:64"`
	CreatedAt time.Time `json:"created_at"`
}

func (PowerTask) TableName() string { return "t_power_task" }

// CollectTask 作业收集任务
type CollectTask struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	Title       string    `json:"title" gorm:"size:128"`
	RoomID      uint      `json:"room_id"`
	AgentIDs    string    `json:"agent_ids" gorm:"type:text"`  // JSON 数组
	PathPattern string    `json:"path_pattern" gorm:"size:255"`
	Status      string    `json:"status" gorm:"size:16;default:running"` // running | done
	Collected   int       `json:"collected" gorm:"default:0"`
	CreatedBy   string    `json:"created_by" gorm:"size:64"`
	CreatedAt   time.Time `json:"created_at"`
}

func (CollectTask) TableName() string { return "t_collect_task" }

// CollectFile 作业收集文件记录
type CollectFile struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	TaskID    uint      `json:"task_id" gorm:"index"`
	AgentID   string    `json:"agent_id" gorm:"size:64"`
	FileName  string    `json:"file_name" gorm:"size:255"`
	FilePath  string    `json:"file_path" gorm:"size:512"`
	FileSize  int64     `json:"file_size"`
	CreatedAt time.Time `json:"created_at"`
}

func (CollectFile) TableName() string { return "t_collect_file" }

// RegisterP2Models 供 main 调用做 AutoMigrate
func RegisterP2Models() []interface{} {
	return []interface{}{&PowerTask{}, &CollectTask{}, &CollectFile{}}
}

// 作业收集落盘根目录：可用环境变量 LAB_COLLECT_DIR 覆盖（生产由 install.sh 创建 /var/lib/lab-teaching/collect 并授权）
func collectBaseDir() string {
	if d := os.Getenv("LAB_COLLECT_DIR"); d != "" {
		return d
	}
	return "/var/lib/lab-teaching/collect"
}

// ============ P2-1 监控墙 ============

// WallSeat 监控墙座位数据
type WallSeat struct {
	SeatID   uint   `json:"seat_id"`
	SeatNo   string `json:"seat_no"`
	AgentID  string `json:"agent_id"`
	Hostname string `json:"hostname"`
	Online   bool   `json:"online"`
}

// wallAgentsOfRoom 解析机房的 agent 列表（座位 → agent 会话）
func (h *Handler) wallAgentsOfRoom(roomID uint) ([]WallSeat, []string) {
	seats, _ := h.Repo.ListSeatsByRoom(roomID)
	sessions, _ := h.Repo.ListAgentSessions()
	sessBySeat := make(map[uint]*model.AgentSession, len(sessions))
	for i := range sessions {
		if sessions[i].SeatID != nil {
			sessBySeat[*sessions[i].SeatID] = &sessions[i]
		}
	}
	grid := make([]WallSeat, 0, len(seats))
	ids := make([]string, 0)
	for _, s := range seats {
		ws := WallSeat{SeatID: s.ID, SeatNo: s.SeatNo}
		if sess, ok := sessBySeat[s.ID]; ok {
			ws.AgentID = sess.AgentID
			ws.Online = sess.Online
			ids = append(ids, sess.AgentID)
		}
		grid = append(grid, ws)
	}
	return grid, ids
}

// WallSeats GET /wall/seats?room_id= 监控墙座位网格数据
func (h *Handler) WallSeats(c *gin.Context) {
	roomID, _ := strconv.ParseUint(c.Query("room_id"), 10, 64)
	if roomID == 0 {
		response.BadRequest(c, "room_id 必填")
		return
	}
	grid, _ := h.wallAgentsOfRoom(uint(roomID))
	response.Success(c, grid)
}

// WallStart POST /wall/start {room_id, fps, quality} 下发 thumb_start
func (h *Handler) WallStart(c *gin.Context) {
	var req struct {
		RoomID  uint `json:"room_id" binding:"required"`
		FPS     int  `json:"fps"`
		Quality int  `json:"quality"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.FPS <= 0 || req.FPS > 5 {
		req.FPS = 1
	}
	if req.Quality <= 0 || req.Quality > 80 {
		req.Quality = 40
	}
	_, ids := h.wallAgentsOfRoom(req.RoomID)
	notified := 0
	for _, agentID := range ids {
		f := ws.Frame{Type: "cmd", MsgID: fmt.Sprintf("wall_%d_%s", time.Now().UnixNano(), agentID),
			Payload: mustJSON(map[string]interface{}{"action": "thumb_start",
				"params": map[string]interface{}{"fps": req.FPS, "quality": req.Quality}})}
		if h.Hub.SendToAgent(agentID, f) {
			notified++
		}
	}
	operator, uidNum := currentUser(c)
	h.Repo.CreateAudit(&model.Audit{UserID: uidNum, Action: "wall_start", Target: fmt.Sprintf("room=%d", req.RoomID), IP: c.ClientIP(), Detail: fmt.Sprintf("operator=%s notified=%d/%d fps=%d q=%d", operator, notified, len(ids), req.FPS, req.Quality)})
	response.Success(c, gin.H{"notified": notified, "total": len(ids)})
}

// WallStop POST /wall/stop {room_id} 下发 thumb_stop
func (h *Handler) WallStop(c *gin.Context) {
	var req struct {
		RoomID uint `json:"room_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	_, ids := h.wallAgentsOfRoom(req.RoomID)
	notified := 0
	for _, agentID := range ids {
		f := ws.Frame{Type: "cmd", MsgID: fmt.Sprintf("wallstop_%d_%s", time.Now().UnixNano(), agentID),
			Payload: mustJSON(map[string]interface{}{"action": "thumb_stop"})}
		if h.Hub.SendToAgent(agentID, f) {
			notified++
		}
	}
	operator, uidNum := currentUser(c)
	h.Repo.CreateAudit(&model.Audit{UserID: uidNum, Action: "wall_stop", Target: fmt.Sprintf("room=%d", req.RoomID), IP: c.ClientIP(), Detail: fmt.Sprintf("operator=%s notified=%d/%d", operator, notified, len(ids))})
	response.Success(c, gin.H{"notified": notified, "total": len(ids)})
}

// ============ P2-2 WOL 远程开机 ============

// parseMAC 规范化 MAC 为 6 字节
func parseMAC(s string) (net.HardwareAddr, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.ReplaceAll(s, "-", ":")
	return net.ParseMAC(s)
}

// sendMagicPacket 发送 WOL magic packet（UDP 广播 :9 和 :7）
func sendMagicPacket(mac net.HardwareAddr) error {
	pkt := make([]byte, 0, 102)
	pkt = append(pkt, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF)
	for i := 0; i < 16; i++ {
		pkt = append(pkt, mac...)
	}
	var lastErr error
	for _, addr := range []string{"255.255.255.255:9", "255.255.255.255:7"} {
		conn, err := net.ListenPacket("udp4", ":0")
		if err != nil {
			lastErr = err
			continue
		}
		dst, _ := net.ResolveUDPAddr("udp4", addr)
		if _, err := conn.WriteTo(pkt, dst); err != nil {
			lastErr = err
		}
		conn.Close()
	}
	return lastErr
}

// WakeOnLan POST /wol {agent_ids[] | agent_id | mac | device_id}
func (h *Handler) WakeOnLan(c *gin.Context) {
	var req struct {
		AgentIDs []string `json:"agent_ids"`
		AgentID  string   `json:"agent_id"`
		MAC      string   `json:"mac"`
		DeviceID uint     `json:"device_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.AgentID != "" {
		req.AgentIDs = append(req.AgentIDs, req.AgentID)
	}
	if len(req.AgentIDs) == 0 && req.MAC == "" && req.DeviceID == 0 {
		response.BadRequest(c, "请提供 agent_ids / mac / device_id")
		return
	}
	sessions, _ := h.Repo.ListAgentSessions()
	devices, _ := h.Repo.ListDevices()
	type sentItem struct {
		Target string `json:"target"`
		MAC    string `json:"mac"`
		OK     bool   `json:"ok"`
	}
	var sent []sentItem
	failReasons := []string{}
	for _, agentID := range req.AgentIDs {
		macStr := ""
		for i := range sessions {
			if sessions[i].AgentID == agentID && sessions[i].MAC != "" {
				macStr = sessions[i].MAC
				break
			}
		}
		if macStr == "" {
			for i := range devices {
				if devices[i].AssetCode == agentID && devices[i].MAC != "" {
					macStr = devices[i].MAC
					break
				}
			}
		}
		if macStr == "" {
			failReasons = append(failReasons, agentID+": 无 MAC 记录")
			sent = append(sent, sentItem{Target: agentID, OK: false})
			continue
		}
		mac, err := parseMAC(macStr)
		if err != nil || sendMagicPacket(mac) != nil {
			failReasons = append(failReasons, agentID+": MAC 格式错误/发送失败")
			sent = append(sent, sentItem{Target: agentID, MAC: macStr, OK: false})
			continue
		}
		sent = append(sent, sentItem{Target: agentID, MAC: mac.String(), OK: true})
	}
	if req.MAC != "" {
		mac, err := parseMAC(req.MAC)
		if err != nil {
			response.BadRequest(c, "MAC 格式错误: "+err.Error())
			return
		}
		ok := sendMagicPacket(mac) == nil
		sent = append(sent, sentItem{Target: req.MAC, MAC: mac.String(), OK: ok})
	}
	if req.DeviceID > 0 {
		for i := range devices {
			if devices[i].ID == req.DeviceID && devices[i].MAC != "" {
				mac, err := parseMAC(devices[i].MAC)
				ok := err == nil && sendMagicPacket(mac) == nil
				sent = append(sent, sentItem{Target: devices[i].AssetCode, MAC: devices[i].MAC, OK: ok})
				break
			}
		}
	}
	sentCount := 0
	for _, s := range sent {
		if s.OK {
			sentCount++
		}
	}
	operator, uidNum := currentUser(c)
	targets := make([]string, 0, len(sent))
	for _, s := range sent {
		targets = append(targets, s.Target)
	}
	detail := fmt.Sprintf("operator=%s sent=%d/%d", operator, sentCount, len(sent))
	if len(failReasons) > 0 {
		detail += " fails=" + strings.Join(failReasons, "; ")
	}
	h.Repo.CreateAudit(&model.Audit{UserID: uidNum, Action: "wol", Target: strings.Join(targets, ","), IP: c.ClientIP(), Detail: detail})
	response.Success(c, gin.H{"sent": sentCount, "total": len(sent), "items": sent, "detail": detail})
}

// ============ P2-3 定时电源任务 ============

func parsePowerAgents(s string) []string {
	var ids []string
	if err := json.Unmarshal([]byte(s), &ids); err != nil {
		return nil
	}
	return ids
}

// CreatePowerTask POST /power-tasks
func (h *Handler) CreatePowerTask(c *gin.Context) {
	var req struct {
		Name      string   `json:"name" binding:"required"`
		Action    string   `json:"action" binding:"required"`
		AgentIDs  []string `json:"agent_ids"`
		ExecuteAt string   `json:"execute_at" binding:"required"` // RFC3339 或 2006-01-02 15:04
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Action != "shutdown" && req.Action != "reboot" && req.Action != "wol" {
		response.BadRequest(c, "action 仅支持 shutdown/reboot/wol")
		return
	}
	if len(req.AgentIDs) == 0 {
		response.BadRequest(c, "agent_ids 不能为空")
		return
	}
	t, err := time.ParseInLocation("2006-01-02 15:04", req.ExecuteAt, time.Local)
	if err != nil {
		t, err = time.Parse(time.RFC3339, req.ExecuteAt)
		if err != nil {
			response.BadRequest(c, "execute_at 格式错误（支持 2006-01-02 15:04 或 RFC3339）")
			return
		}
	}
	idsJSON, _ := json.Marshal(req.AgentIDs)
	operator, _ := currentUser(c)
	task := PowerTask{Name: req.Name, Action: req.Action, AgentIDs: string(idsJSON),
		ExecuteAt: t, Status: "pending", CreatedBy: operator, CreatedAt: time.Now()}
	if err := h.Repo.DB.Create(&task).Error; err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Created(c, task)
}

// ListPowerTasks GET /power-tasks
func (h *Handler) ListPowerTasks(c *gin.Context) {
	var tasks []PowerTask
	h.Repo.DB.Order("id DESC").Limit(100).Find(&tasks)
	response.Success(c, tasks)
}

// DeletePowerTask DELETE /power-tasks/:id（仅 pending 可删）
func (h *Handler) DeletePowerTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var task PowerTask
	if h.Repo.DB.First(&task, id).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}
	if task.Status != "pending" {
		response.BadRequest(c, "仅待执行任务可删除")
		return
	}
	h.Repo.DB.Delete(&PowerTask{}, id)
	response.Success(c, nil)
}

// RunDuePowerTasks 后台调度：执行到期的电源任务（main.go 中起 goroutine 调用）
func (h *Handler) RunDuePowerTasks(stop <-chan struct{}) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			var tasks []PowerTask
			h.Repo.DB.Where("status = ? AND execute_at <= ?", "pending", time.Now()).Find(&tasks)
			for i := range tasks {
				h.executePowerTask(&tasks[i])
			}
		}
	}
}

func (h *Handler) executePowerTask(task *PowerTask) {
	ids := parsePowerAgents(task.AgentIDs)
	notified := 0
	switch task.Action {
	case "shutdown", "reboot":
		for _, agentID := range ids {
			f := ws.Frame{Type: "cmd", MsgID: fmt.Sprintf("pt_%d_%s", task.ID, agentID),
				Payload: mustJSON(map[string]interface{}{"action": task.Action})}
			if h.Hub.SendToAgent(agentID, f) {
				notified++
			}
		}
	case "wol":
		sessions, _ := h.Repo.ListAgentSessions()
		devices, _ := h.Repo.ListDevices()
		for _, agentID := range ids {
			macStr := ""
			for _, s := range sessions {
				if s.AgentID == agentID && s.MAC != "" {
					macStr = s.MAC
					break
				}
			}
			if macStr == "" {
				for _, d := range devices {
					if d.MAC != "" && strings.EqualFold(d.AssetCode, agentID) {
						macStr = d.MAC
						break
					}
				}
			}
			if mac, err := parseMAC(macStr); err == nil {
				if sendMagicPacket(mac) == nil {
					notified++
				}
			}
		}
	}
	status := "done"
	detail := fmt.Sprintf("notified=%d/%d", notified, len(ids))
	if notified == 0 && len(ids) > 0 {
		status = "failed"
	}
	h.Repo.DB.Model(&PowerTask{}).Where("id = ?", task.ID).
		Updates(map[string]interface{}{"status": status, "detail": detail})
	h.Repo.CreateAudit(&model.Audit{Action: "power_task_run", Target: task.Name,
		Detail: fmt.Sprintf("action=%s %s by=%s", task.Action, detail, task.CreatedBy)})
}

// ============ P2-5 作业收集 ============

// CreateCollectTask POST /collect/tasks {title, room_id, path_pattern, agent_ids?}
func (h *Handler) CreateCollectTask(c *gin.Context) {
	var req struct {
		Title       string   `json:"title" binding:"required"`
		RoomID      uint     `json:"room_id"`
		PathPattern string   `json:"path_pattern" binding:"required"`
		AgentIDs    []string `json:"agent_ids"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.RoomID == 0 && len(req.AgentIDs) == 0 {
		response.BadRequest(c, "room_id 与 agent_ids 至少提供一个")
		return
	}
	ids := req.AgentIDs
	if req.RoomID > 0 {
		_, roomIDs := h.wallAgentsOfRoom(req.RoomID)
		ids = roomIDs
	}
	if len(ids) == 0 {
		response.BadRequest(c, "该机房当前无已登记 Agent")
		return
	}
	idsJSON, _ := json.Marshal(ids)
	operator, _ := currentUser(c)
	task := CollectTask{Title: req.Title, RoomID: req.RoomID, AgentIDs: string(idsJSON),
		PathPattern: req.PathPattern, Status: "running", CreatedBy: operator, CreatedAt: time.Now()}
	if err := h.Repo.DB.Create(&task).Error; err != nil {
		response.ServerError(c, err.Error())
		return
	}
	// 下发 collect_files 指令
	notified := 0
	for _, agentID := range ids {
		f := ws.Frame{Type: "cmd", MsgID: fmt.Sprintf("ct_%d_%s", task.ID, agentID),
			Payload: mustJSON(map[string]interface{}{"action": "collect_files",
				"params": map[string]interface{}{"task_id": task.ID, "pattern": req.PathPattern}})}
		if h.Hub.SendToAgent(agentID, f) {
			notified++
		}
	}
	h.Repo.CreateAudit(&model.Audit{Action: "collect_create", Target: req.Title,
		Detail: fmt.Sprintf("operator=%s pattern=%s agents=%d notified=%d", operator, req.PathPattern, len(ids), notified)})
	response.Created(c, gin.H{"task": task, "notified": notified, "total": len(ids)})
}

// ListCollectTasks GET /collect/tasks
func (h *Handler) ListCollectTasks(c *gin.Context) {
	var tasks []CollectTask
	h.Repo.DB.Order("id DESC").Limit(50).Find(&tasks)
	response.Success(c, tasks)
}

// GetCollectTask GET /collect/tasks/:id（含文件列表）
func (h *Handler) GetCollectTask(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var task CollectTask
	if h.Repo.DB.First(&task, id).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}
	var files []CollectFile
	h.Repo.DB.Where("task_id = ?", id).Order("id").Find(&files)
	response.Success(c, gin.H{"task": task, "files": files})
}

// CollectUpload POST /collect/upload（Agent 免登录）multipart: task_id, agent_id, file
func (h *Handler) CollectUpload(c *gin.Context) {
	taskID, _ := strconv.ParseUint(c.PostForm("task_id"), 10, 64)
	agentID := c.PostForm("agent_id")
	if taskID == 0 || agentID == "" {
		response.BadRequest(c, "task_id/agent_id 必填")
		return
	}
	var task CollectTask
	if h.Repo.DB.First(&task, taskID).Error != nil {
		response.NotFound(c, "任务不存在")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.BadRequest(c, "file 必填")
		return
	}
	defer file.Close()
	if header.Size > 200*1024*1024 {
		response.Error(c, 413, "文件超过 200MB 上限")
		return
	}
	// 落盘：/var/lib/lab-teaching/collect/<task>/<agent>/
	name := filepath.Base(header.Filename)
	dir := filepath.Join(collectBaseDir(), fmt.Sprintf("task_%d", taskID), agentID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	dest := filepath.Join(dir, name)
	out, err := os.Create(dest)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	written, err := io.Copy(out, file)
	out.Close()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	cf := CollectFile{TaskID: uint(taskID), AgentID: agentID, FileName: name,
		FilePath: dest, FileSize: written, CreatedAt: time.Now()}
	h.Repo.DB.Create(&cf)
	h.Repo.DB.Model(&CollectTask{}).Where("id = ?", taskID).
		UpdateColumn("collected", h.Repo.DB.Raw("(SELECT COUNT(*) FROM t_collect_file WHERE task_id = ?)", taskID))
	if h.BrowserHub != nil {
		h.BrowserHub.Broadcast("collect_file", gin.H{"task_id": taskID, "agent_id": agentID, "file_name": name, "size": written})
	}
	response.Success(c, gin.H{"id": cf.ID, "size": written})
}

// DownloadCollectFile GET /collect/download/:file_id
func (h *Handler) DownloadCollectFile(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("file_id"), 10, 64)
	var cf CollectFile
	if h.Repo.DB.First(&cf, id).Error != nil {
		response.NotFound(c, "文件不存在")
		return
	}
	c.Header("Content-Disposition", "attachment; filename="+strconv.Quote(cf.FileName))
	c.File(cf.FilePath)
}

// ============ Agent 免登录路由辅助 ============

// HandleCollectUploadNoAuth 供 main.go 路由注册（与 CollectUpload 同一实现）
func (h *Handler) HandleCollectUploadNoAuth(c *gin.Context) {
	h.CollectUpload(c)
}
