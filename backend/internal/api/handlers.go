package api

import (
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"lab-teaching/backend/internal/broadcast"
	"lab-teaching/backend/internal/config"
	"lab-teaching/backend/internal/middleware"
	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/internal/repository"
	ws "lab-teaching/backend/internal/websocket"
	"lab-teaching/backend/pkg/auth"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	Repo           *repository.Repo
	Hub            *ws.Hub
	Remote         *ws.RemoteManager
	RemoteEnhanced *ws.RemoteEnhanced
	Streamer       *broadcast.Manager
	MultiRoom      *broadcast.MultiRoomManager
	FileTransfer   *FileTransferManager
	Homework       *HomeworkManager
	Classroom      *ClassroomManager
	JWTSecret      string
	ExpireHours    int
	Config         *config.Config
	BrowserHub     *ws.BrowserHub // P0-1: 指令 ack 实时推送
}

func New(repo *repository.Repo, hub *ws.Hub, remote *ws.RemoteManager, streamer *broadcast.Manager, secret string, expireHours int, cfg *config.Config) *Handler {
	auth.SetSecret(secret)
	return &Handler{
		Repo:           repo,
		Hub:            hub,
		Remote:         remote,
		RemoteEnhanced: ws.NewRemoteEnhanced(hub),
		Streamer:       streamer,
		MultiRoom:      broadcast.NewMultiRoomManager(streamer),
		FileTransfer:   NewFileTransferManager(""),
		Homework:       NewHomeworkManager(""),
		Classroom:      NewClassroomManager(),
		JWTSecret:      secret,
		ExpireHours:    expireHours,
		Config:         cfg,
	}
}

// ============ 认证 ============
type loginReq struct {
	Username      string `json:"username" binding:"required"`
	Password      string `json:"password" binding:"required"`
	CaptchaID     string `json:"captcha_id"`
	CaptchaAnswer string `json:"captcha_answer"`
}

func (h *Handler) Login(c *gin.Context) {
	// 登录锁定检查
	ip := c.ClientIP()
	if middleware.CheckLoginLock(ip) {
		response.Error(c, 423, "登录失败次数过多，账户已临时锁定，请15分钟后重试")
		return
	}

	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	// 失败 ≥3 次后要求验证码
	fails := middleware.GetLoginFails(ip)
	if fails >= 3 {
		if req.CaptchaID == "" || !verifyCaptcha(req.CaptchaID, req.CaptchaAnswer) {
			response.Error(c, 429, "验证码错误或已过期，请刷新后重试")
			return
		}
	}

	u, err := h.Repo.FindUserByUsername(req.Username)
	if err != nil {
		middleware.RecordLoginFail(ip)
		remaining := middleware.MaxLoginFails() - middleware.GetLoginFails(ip)
		msg := "用户名或密码错误"
		if remaining <= 3 && remaining > 0 { msg = fmt.Sprintf("用户名或密码错误，还剩 %d 次尝试", remaining) }
		response.Unauthorized(c, msg)
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		middleware.RecordLoginFail(ip)
		remaining := middleware.MaxLoginFails() - middleware.GetLoginFails(ip)
		msg := "用户名或密码错误"
		if remaining <= 3 && remaining > 0 { msg = fmt.Sprintf("用户名或密码错误，还剩 %d 次尝试", remaining) }
		response.Unauthorized(c, msg)
		return
	}
	middleware.RecordLoginSuccess(ip)
	token, err := auth.GenerateToken(u.ID, u.Username, u.Role, h.ExpireHours)
	if err != nil {
		response.ServerError(c, "生成 token 失败")
		return
	}
	h.Repo.CreateAudit(&model.Audit{UserID: u.ID, Action: "login", Target: u.Username, IP: c.ClientIP()})
	response.Success(c, gin.H{"token": token, "expires": h.ExpireHours, "user": u, "permissions": h.permissionsForUser(u)})
}

func (h *Handler) Profile(c *gin.Context) {
	uid, _ := c.Get("user_id")
	u, err := h.Repo.FindUserByUsername(c.GetString("username"))
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	_ = uid
	response.Success(c, gin.H{"user": u, "permissions": h.permissionsForUser(u)})
}

func (h *Handler) ChangePassword(c *gin.Context) {
	uid, _ := c.Get("user_id")
	u, err := h.Repo.FindUserByID(uid.(uint))
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	var req struct {
		OldPassword string `json:"old_password" binding:"required"`
		NewPassword string `json:"new_password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.OldPassword)); err != nil {
		response.Error(c, 403, "旧密码不正确")
		return
	}
	if err := validatePassword(req.NewPassword); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		response.ServerError(c, "密码加密失败")
		return
	}
	u.PasswordHash = string(hash)
	if err := h.Repo.UpdateUser(u); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ============ 机房 ============
func (h *Handler) ListRooms(c *gin.Context) {
	rooms, err := h.Repo.ListRooms()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, rooms)
}

func (h *Handler) CreateRoom(c *gin.Context) {
	var room model.Room
	if err := c.ShouldBindJSON(&room); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if err := h.Repo.CreateRoom(&room); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Created(c, room)
}

func (h *Handler) UpdateRoom(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	room, err := h.Repo.GetRoom(uint(id))
	if err != nil {
		response.NotFound(c, "机房不存在")
		return
	}
	var req struct {
		Name      *string `json:"name"`
		Building  *string `json:"building"`
		Floor     *string `json:"floor"`
		SeatCount *int    `json:"seat_count"`
		NetType   *string `json:"net_type"`
		BootMode  *string `json:"boot_mode"`
		Status    *string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Name != nil {
		if *req.Name == "" {
			response.BadRequest(c, "机房名称不能为空")
			return
		}
		room.Name = *req.Name
	}
	if req.Building != nil {
		room.Building = *req.Building
	}
	if req.Floor != nil {
		room.Floor = *req.Floor
	}
	if req.SeatCount != nil {
		if *req.SeatCount < 0 {
			response.BadRequest(c, "座位数不能为负数")
			return
		}
		room.SeatCount = *req.SeatCount
	}
	if req.NetType != nil {
		room.NetType = *req.NetType
	}
	if req.BootMode != nil {
		room.BootMode = *req.BootMode
	}
	if req.Status != nil {
		room.Status = *req.Status
	}
	if err := h.Repo.UpdateRoom(room); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, room)
}

func (h *Handler) DeleteRoom(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.Repo.DeleteRoom(uint(id)); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"deleted": id})
}

// VerifyPassword 高危操作前的当前用户密码二次验证（如启动升级任务前确认身份）
func (h *Handler) VerifyPassword(c *gin.Context) {
	var req struct {
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	username := c.GetString("username")
	u, err := h.Repo.FindUserByUsername(username)
	if err != nil {
		response.Unauthorized(c, "用户不存在")
		return
	}
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(req.Password)); err != nil {
		response.Unauthorized(c, "密码错误")
		return
	}
	h.Repo.CreateAudit(&model.Audit{UserID: u.ID, Action: "verify_password", Target: u.Username, IP: c.ClientIP(), Detail: "高危操作密码二次验证"})
	response.Success(c, gin.H{"verified": true})
}

func (h *Handler) ListSeats(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	seats, err := h.Repo.ListSeatsByRoom(uint(id))
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, seats)
}

// ============ 设备 ============
func (h *Handler) ListDevices(c *gin.Context) {
	devices, err := h.Repo.ListDevices()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, devices)
}

// ============ Agent 在线 ============
func (h *Handler) AgentOnline(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "0"))
	if page > 0 && size > 0 {
		if page < 1 { page = 1 }
		if size < 1 || size > 100 { size = 20 }
		sessions, total, _ := h.Repo.ListAgentSessionsPaged(page, size)
		response.Success(c, gin.H{
			"online_count": h.Hub.OnlineCount(),
			"online_ids":   h.Hub.OnlineAgents(),
			"sessions":     sessions,
			"total":        total,
			"page":         page,
			"page_size":    size,
		})
		return
	}
	sessions, _ := h.Repo.ListAgentSessions()
	response.Success(c, gin.H{
		"online_count": h.Hub.OnlineCount(),
		"online_ids":   h.Hub.OnlineAgents(),
		"sessions":     sessions,
	})
}

// ============ 教学管控 ============
type broadcastReq struct {
	RoomID  uint     `json:"room_id" binding:"required"`
	Scope   string   `json:"scope"`
	Targets []string `json:"targets"` // agent_id 列表
	Source  string   `json:"source"`  // screen | test | file:/path
	Width   int      `json:"width"`
	Height  int      `json:"height"`
	FPS     int      `json:"fps"`
	Bitrate string   `json:"bitrate"`
}

func (h *Handler) StartBroadcast(c *gin.Context) {
	var req broadcastReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Scope == "" {
		req.Scope = "全班"
	}
	if req.Source == "" {
		req.Source = "screen"
	}

	// 1. 创建广播任务记录
	uid, _ := c.Get("user_id")
	mcast := "239.1.1.1:5004"
	task := &model.BroadcastTask{
		RoomID:    req.RoomID,
		TeacherID: uid.(uint),
		Scope:     req.Scope,
		Targets:   string(mustJSON(req.Targets)),
		McastAddr: mcast,
		Status:    "进行中",
		StartAt:   time.Now(),
	}
	h.Repo.CreateBroadcastTask(task)

	// 2. 启动 ffmpeg 推流进程
	err := h.Streamer.Start(broadcast.StreamConfig{
		TaskID:    task.ID,
		McastAddr: mcast,
		Source:    req.Source,
		Width:     req.Width,
		Height:    req.Height,
		FPS:       req.FPS,
		Bitrate:   req.Bitrate,
		Display:   ":0",
	})
	if err != nil {
		response.ServerError(c, "推流启动失败: "+err.Error())
		return
	}

	// 3. 通过 WebSocket 通知目标 Agent 加入组播接收
	payload, _ := json.Marshal(map[string]string{
		"mcast_addr": mcast,
		"task_id":    strconv.FormatUint(uint64(task.ID), 10),
		"codec":      "H.264",
		"transport":  "rtp_mpegts",
	})
	f := ws.Frame{Type: "cmd", MsgID: "bc_" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Payload: mustJSON(map[string]interface{}{"action": "broadcast_join", "params": json.RawMessage(payload)})}
	n := h.Hub.BroadcastToAgents(req.Targets, f)

	response.Success(c, gin.H{
		"task_id":    task.ID,
		"mcast_addr": mcast,
		"source":     req.Source,
		"notified":   n,
		"status":     "streaming",
	})
}

func (h *Handler) StopBroadcast(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	taskID := uint(id)

	// 1. 停止 ffmpeg 推流
	if err := h.Streamer.Stop(taskID); err != nil {
		response.ServerError(c, err.Error())
		return
	}

	// 2. 通知 Agent 停止接收
	f := ws.Frame{Type: "cmd", MsgID: "bc_stop_" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Payload: mustJSON(map[string]interface{}{"action": "broadcast_stop", "params": map[string]string{
			"task_id": strconv.FormatUint(uint64(taskID), 10),
		}})}
	h.Hub.BroadcastToAll(f)

	// 3. 更新任务状态
	h.Repo.UpdateBroadcastStatus(taskID, "已结束")
	response.Success(c, gin.H{"task_id": taskID, "status": "stopped"})
}

func (h *Handler) BroadcastStatus(c *gin.Context) {
	streams := h.Streamer.ActiveStreams()
	var list []gin.H
	for _, s := range streams {
		list = append(list, gin.H{
			"task_id":    s.Config.TaskID,
			"mcast_addr": s.Config.McastAddr,
			"source":     s.Config.Source,
			"status":     s.Status,
			"start_at":   s.StartAt,
		})
	}
	response.Success(c, gin.H{"active_streams": list, "count": len(list)})
}

func (h *Handler) ListBroadcasts(c *gin.Context) {
	tasks, _ := h.Repo.ListBroadcastTasks()
	response.Success(c, tasks)
}

type cmdReq struct {
	AgentIDs []string               `json:"agent_ids" binding:"required"`
	Action   string                 `json:"action" binding:"required"`
	Params   map[string]interface{} `json:"params"`
}

// SendCommand 通用控制指令下发（power/lock/exec/exam_enter 等）
func (h *Handler) SendCommand(c *gin.Context) {
	var req cmdReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	f := ws.Frame{Type: "cmd", MsgID: "cmd_" + strconv.FormatInt(time.Now().UnixNano(), 10),
		Payload: mustJSON(map[string]interface{}{"action": req.Action, "params": req.Params})}
	n := h.Hub.BroadcastToAgents(req.AgentIDs, f)
	response.Success(c, gin.H{"action": req.Action, "notified": n, "total": len(req.AgentIDs)})
}

// ============ 无盘镜像 ============
func (h *Handler) ListImages(c *gin.Context) {
	images, _ := h.Repo.ListDiskImages()
	response.Success(c, images)
}

// ============ 教学 ============
func (h *Handler) ListCourses(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "0"))
	if page > 0 && size > 0 {
		if page < 1 { page = 1 }
		if size < 1 || size > 100 { size = 20 }
		list, total, _ := h.Repo.ListCoursesPaged(page, size)
		response.Success(c, response.PageData{Items: list, Total: total, Page: page, Size: size})
		return
	}
	list, _ := h.Repo.ListCourses()
	response.Success(c, list)
}
func (h *Handler) ListClazzes(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
	size, _ := strconv.Atoi(c.DefaultQuery("page_size", "0"))
	if page > 0 && size > 0 {
		if page < 1 { page = 1 }
		if size < 1 || size > 100 { size = 20 }
		list, total, _ := h.Repo.ListClazzesPaged(page, size)
		response.Success(c, response.PageData{Items: list, Total: total, Page: page, Size: size})
		return
	}
	list, _ := h.Repo.ListClazzes()
	response.Success(c, list)
}
func (h *Handler) ListStudents(c *gin.Context) {
	list, _ := h.Repo.ListStudents()
	response.Success(c, list)
}

// ============ 统计 ============
func (h *Handler) Dashboard(c *gin.Context) {
	rooms, _ := h.Repo.ListRooms()
	devices, _ := h.Repo.ListDevices()
	agentsOnline, _ := h.Repo.ListAgentSessionsOnline()
	audits, _ := h.Repo.ListAudits(10)
	violations, _ := h.Repo.RecentViolations(10)

	// 房间利用率
	roomStats := make([]gin.H, 0, len(rooms))
	for _, room := range rooms {
		seats, _ := h.Repo.ListSeatsByRoom(room.ID)
		onlineInRoom := 0
		for _, a := range agentsOnline {
			if a.SeatID != nil {
				for _, s := range seats {
					if s.ID == *a.SeatID {
						onlineInRoom++
						break
					}
				}
			}
		}
		rate := 0.0
		if room.SeatCount > 0 {
			rate = float64(onlineInRoom) / float64(room.SeatCount) * 100
		}
		roomStats = append(roomStats, gin.H{
			"room_id":   room.ID,
			"room_name": room.Name,
			"seats":     room.SeatCount,
			"online":    onlineInRoom,
			"rate":      fmt.Sprintf("%.1f%%", rate),
		})
	}

	response.Success(c, gin.H{
		"overview": gin.H{
			"room_count":    len(rooms),
			"device_count":  len(devices),
			"agent_online":  h.Hub.OnlineCount(),
			"agent_db_online": len(agentsOnline),
			"user_count":    h.Repo.CountUsers(),
			"question_count": h.Repo.CountQuestions(),
			"paper_count":   h.Repo.CountPapers(),
			"exam_total":    h.Repo.CountExamsByStatus(""),
			"exam_running":  h.Repo.CountExamsByStatus("进行中"),
			"exam_finished": h.Repo.CountExamsByStatus("已结束"),
			"violation_count": h.Repo.CountViolations(),
			"device_in_use": h.Repo.CountDevicesByStatus("在用"),
			"device_fault":  h.Repo.CountDevicesByStatus("故障"),
		},
		"room_stats":   roomStats,
		"recent_audits": audits,
		"recent_violations": violations,
	})
}

// ============ 审计 ============
func (h *Handler) ListAudits(c *gin.Context) {
	// 支持 ?q=关键词&page=1&size=20；不带参数时兼容旧行为返回最近100条
	q := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("size", "20"))
	list, total, _ := h.Repo.SearchAudits(q, page, size)
	response.Success(c, gin.H{"list": list, "total": total, "page": page, "size": size})
}

func (h *Handler) ExportAudits(c *gin.Context) {
	q := c.Query("q")
	list, _ := h.Repo.SearchAuditsAll(q) // 导出全部匹配结果
	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=audit_logs.csv")
	// BOM for Excel
	c.Writer.Write([]byte{0xEF, 0xBB, 0xBF})
	c.Writer.WriteString("ID,时间,操作,目标,IP,详情\n")
	for _, a := range list {
		line := fmt.Sprintf("%d,%s,%s,%s,%s,\"%s\"\n", a.ID, a.CreatedAt.Format("2006-01-02 15:04:05"), csvEscape(a.Action), csvEscape(a.Target), a.IP, csvEscape(a.Detail))
		c.Writer.WriteString(line)
	}
}

func csvEscape(s string) string { return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\"" }

func mustJSON(v interface{}) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// validatePassword 密码复杂度：≥8位，含字母+数字
func validatePassword(pw string) error {
	if len(pw) < 8 {
		return fmt.Errorf("密码至少8位")
	}
	hasLetter := false
	hasDigit := false
	for _, c := range pw {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' { hasLetter = true }
		if c >= '0' && c <= '9' { hasDigit = true }
	}
	if !hasLetter { return fmt.Errorf("密码需包含字母") }
	if !hasDigit { return fmt.Errorf("密码需包含数字") }
	return nil
}

// ============ 文件收发 ============

// UploadFile 教师上传文件（multipart/form-data）
func (h *Handler) UploadFile(c *gin.Context) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Error(c, 400, "请选择文件")
		return
	}
	defer file.Close()

	roomID, _ := strconv.ParseUint(c.PostForm("room_id"), 10, 32)
	if roomID == 0 {
		response.Error(c, 400, "room_id 必填")
		return
	}

	// 存储到本地目录
	storageDir := "/tmp/lab-files"
	os.MkdirAll(storageDir, 0755)
	filename := fmt.Sprintf("%d_%s", time.Now().UnixNano(), header.Filename)
	storagePath := storageDir + "/" + filename

	dst, err := os.Create(storagePath)
	if err != nil {
		response.Error(c, 500, "存储失败: "+err.Error())
		return
	}
	defer dst.Close()
	size, err := io.Copy(dst, file)
	if err != nil {
		response.Error(c, 500, "写入失败: "+err.Error())
		return
	}

	// 创建分发任务
	task := model.FileTask{
		Name:        header.Filename,
		Size:        size,
		Direction:   "distribute",
		StoragePath: storagePath,
		RoomID:      uint(roomID),
		CreatorID:   1, // TODO: 从JWT取
		Status:      "pending",
	}
	h.Repo.CreateFileTask(&task)
	response.Success(c, gin.H{"task_id": task.ID, "name": task.Name, "size": size})
}

// DistributeFile 发起文件分发（通知Agent下载）
func (h *Handler) DistributeFile(c *gin.Context) {
	var req struct {
		TaskID  uint     `json:"task_id" binding:"required"`
		Targets []string `json:"targets"` // 空=全班在线
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	task, err := h.Repo.GetFileTask(req.TaskID)
	if err != nil {
		response.Error(c, 404, "任务不存在")
		return
	}

	// 确定目标Agent
	targets := req.Targets
	if len(targets) == 0 {
		targets = h.Hub.OnlineAgents()
	}
	if len(targets) == 0 {
		response.Error(c, 400, "无在线终端")
		return
	}

	// 构建下载URL（Agent通过HTTP从服务端拉取）
	downloadURL := fmt.Sprintf("http://%s/api/v1/files/%d/download", c.Request.Host, task.ID)

	// 创建进度记录
	var progress []model.FileProgress
	for _, aid := range targets {
		progress = append(progress, model.FileProgress{
			TaskID:  task.ID,
			AgentID: aid,
			Status:  "pending",
		})
	}
	h.Repo.BatchCreateFileProgress(progress)

	// 更新任务状态
	h.Repo.UpdateFileTask(task.ID, map[string]interface{}{
		"status": "distributing",
		"total":  len(targets),
	})

	// 通过WS通知Agent下载
	notified := h.Hub.BroadcastToAgents(targets, ws.Frame{
		Ver:   1,
		Type:  "cmd",
		MsgID: fmt.Sprintf("file_dist_%d", task.ID),
		TS:    time.Now().Unix(),
		Payload: mustJSON(map[string]interface{}{
			"action": "file_recv",
			"params": map[string]interface{}{
				"task_id":      task.ID,
				"filename":     task.Name,
				"size":         task.Size,
				"download_url": downloadURL,
			},
		}),
	})

	response.Success(c, gin.H{
		"task_id":  task.ID,
		"notified": notified,
		"total":    len(targets),
		"url":      downloadURL,
	})
}

// DownloadFile Agent下载文件
func (h *Handler) DownloadFile(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	task, err := h.Repo.GetFileTask(uint(id))
	if err != nil {
		response.Error(c, 404, "文件不存在")
		return
	}
	c.File(task.StoragePath)
}

// ReportFileProgress Agent上报传输进度
func (h *Handler) ReportFileProgress(c *gin.Context) {
	var req struct {
		TaskID  uint   `json:"task_id" binding:"required"`
		AgentID string `json:"agent_id" binding:"required"`
		Status  string `json:"status"`  // transferring/done/failed
		Percent int    `json:"percent"`
		Error   string `json:"error"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, 400, err.Error())
		return
	}

	updates := map[string]interface{}{
		"status":  req.Status,
		"percent": req.Percent,
		"error":   req.Error,
	}
	h.Repo.UpdateFileProgress(req.TaskID, req.AgentID, updates)

	// 如果完成，更新任务计数
	if req.Status == "done" || req.Status == "failed" {
		progress, _ := h.Repo.ListFileProgress(req.TaskID)
		completed := 0
		for _, p := range progress {
			if p.Status == "done" || p.Status == "failed" {
				completed++
			}
		}
		taskUpdates := map[string]interface{}{"completed": completed}
		if completed >= len(progress) {
			taskUpdates["status"] = "done"
		}
		h.Repo.UpdateFileTask(req.TaskID, taskUpdates)
	}

	response.Success(c, nil)
}

// FileTaskProgress 查询分发进度
func (h *Handler) FileTaskProgress(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	task, err := h.Repo.GetFileTask(uint(id))
	if err != nil {
		response.Error(c, 404, "任务不存在")
		return
	}
	progress, _ := h.Repo.ListFileProgress(uint(id))
	response.Success(c, gin.H{"task": task, "progress": progress})
}

// ListFileTasks 文件任务列表
func (h *Handler) ListFileTasks(c *gin.Context) {
	list, _ := h.Repo.ListFileTasks()
	response.Success(c, list)
}

// ============ 远程控制 ============

type remoteStartReq struct {
	AgentID string `json:"agent_id" binding:"required"`
	FPS     int    `json:"fps"`
	Quality int    `json:"quality"`
}

// StartRemote 发起远程控制会话
func (h *Handler) StartRemote(c *gin.Context) {
	var req remoteStartReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if !h.Hub.IsOnline(req.AgentID) {
		response.Error(c, 400, "Agent 不在线")
		return
	}

	teacherID := uint(0)
	if uid, exists := c.Get("user_id"); exists {
		teacherID = uid.(uint)
	}

	// 创建会话记录
	session := &model.RemoteSession{
		AgentID:   req.AgentID,
		TeacherID: teacherID,
		Status:    "active",
		StartedAt: time.Now(),
	}
	h.Repo.CreateRemoteSession(session)

	// 通知 Agent 启动远程桌面
	fpsVal := req.FPS
	if fpsVal <= 0 { fpsVal = 5 }
	qualVal := req.Quality
	if qualVal <= 0 { qualVal = 75 }

	f := ws.Frame{
		Type:  "cmd",
		MsgID: fmt.Sprintf("remote_%d", session.ID),
		Payload: mustJSON(map[string]interface{}{
			"action": "remote_start",
			"params": map[string]interface{}{
				"session_id": session.ID,
				"fps":        fpsVal,
				"quality":    qualVal,
			},
		}),
	}
	ok := h.Hub.SendToAgent(req.AgentID, f)
	if !ok {
		response.Error(c, 500, "指令下发失败")
		return
	}

	response.Success(c, gin.H{
		"session_id": session.ID,
		"agent_id":   req.AgentID,
		"status":     "active",
		"ws_url":     fmt.Sprintf("/remote/ws?agent_id=%s", req.AgentID),
	})
}

// StopRemote 停止远程控制
func (h *Handler) StopRemote(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	session, err := h.Repo.GetRemoteSession(uint(id))
	if err != nil {
		response.Error(c, 404, "会话不存在")
		return
	}

	now := time.Now()
	h.Repo.UpdateRemoteSession(uint(id), map[string]interface{}{
		"status":     "stopped",
		"stopped_at": &now,
	})

	// 通知 Agent 停止
	f := ws.Frame{
		Type:  "cmd",
		MsgID: fmt.Sprintf("remote_stop_%d", id),
		Payload: mustJSON(map[string]interface{}{
			"action": "remote_stop",
			"params": map[string]interface{}{"session_id": id},
		}),
	}
	h.Hub.SendToAgent(session.AgentID, f)

	response.Success(c, gin.H{"session_id": id, "status": "stopped"})
}

// ListRemoteSessions 远程控制会话列表
func (h *Handler) ListRemoteSessions(c *gin.Context) {
	list, _ := h.Repo.ListRemoteSessions()
	response.Success(c, list)
}

// RemoteStatus 远程控制状态
func (h *Handler) RemoteStatus(c *gin.Context) {
	response.Success(c, gin.H{
		"viewers": h.Remote.ViewerCount(),
	})
}

// ============ 考试域 ============

// --- 题库 ---

type questionReq struct {
	Type       string  `json:"type" binding:"required"`
	Content    string  `json:"content" binding:"required"`
	Options    string  `json:"options"`
	Answer     string  `json:"answer" binding:"required"`
	Analysis   string  `json:"analysis"`
	Difficulty int     `json:"difficulty"`
	Score      float64 `json:"score"`
	Knowledge  string  `json:"knowledge"`
}

func (h *Handler) CreateQuestion(c *gin.Context) {
	var req questionReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	// 校验题型枚举
	validTypes := map[string]bool{"single": true, "multi": true, "judge": true, "fill": true}
	if !validTypes[req.Type] {
		response.BadRequest(c, "题型无效，可选: single/multi/judge/fill")
		return
	}
	if req.Score <= 0 {
		response.BadRequest(c, "分值必须大于0")
		return
	}
	if req.Difficulty < 1 || req.Difficulty > 5 {
		req.Difficulty = 3
	}
	q := model.Question{
		Type:       req.Type,
		Content:    req.Content,
		Options:    req.Options,
		Answer:     req.Answer,
		Analysis:   req.Analysis,
		Difficulty: req.Difficulty,
		Score:      req.Score,
		Knowledge:  req.Knowledge,
		Version:    1,
	}
	if err := h.Repo.CreateQuestion(&q); err != nil {
		response.Error(c, 500, "创建失败")
		return
	}
	response.Success(c, q)
}

func (h *Handler) ListQuestions(c *gin.Context) {
	qType := c.Query("type")
	list, _ := h.Repo.ListQuestions(qType, 200)
	response.Success(c, list)
}

func (h *Handler) GetQuestion(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	q, err := h.Repo.GetQuestion(uint(id))
	if err != nil {
		response.Error(c, 404, "题目不存在")
		return
	}
	response.Success(c, q)
}

// --- 试卷 ---

type paperReq struct {
	Name       string `json:"name" binding:"required"`
	Duration   int    `json:"duration"`
	Strategy   string `json:"strategy"`
	QuestionIDs []uint `json:"question_ids"`
}

func (h *Handler) CreatePaper(c *gin.Context) {
	var req paperReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	teacherID := uint(0)
	if uid, exists := c.Get("user_id"); exists {
		teacherID = uid.(uint)
	}

	paper := model.Paper{
		Name:      req.Name,
		Duration:  req.Duration,
		Strategy:  req.Strategy,
		Status:    "草稿",
		CreatorID: teacherID,
	}
	if paper.Duration <= 0 {
		paper.Duration = 60
	}
	if paper.Strategy == "" {
		paper.Strategy = "固定"
	}
	if err := h.Repo.CreatePaper(&paper); err != nil {
		response.Error(c, 500, "创建试卷失败")
		return
	}

	// 添加题目（批量查询避免 N+1）
	var totalScore float64
	if len(req.QuestionIDs) > 0 {
		questions, _ := h.Repo.GetQuestionsBatch(req.QuestionIDs)
		qScoreMap := make(map[uint]float64)
		for _, q := range questions {
			qScoreMap[q.ID] = q.Score
		}
		var items []model.PaperQuestion
		for i, qid := range req.QuestionIDs {
			score, ok := qScoreMap[qid]
			if !ok {
				continue // 题目不存在，跳过
			}
			items = append(items, model.PaperQuestion{
				PaperID:    paper.ID,
				QuestionID: qid,
				SortOrder:  i + 1,
				Score:      score,
			})
			totalScore += score
		}
		if len(items) > 0 {
			h.Repo.AddPaperQuestions(items)
		}
	}
	h.Repo.UpdatePaper(paper.ID, map[string]interface{}{"total_score": totalScore})
	paper.TotalScore = totalScore

	response.Success(c, paper)
}

func (h *Handler) ListPapers(c *gin.Context) {
	list, _ := h.Repo.ListPapers()
	response.Success(c, list)
}

func (h *Handler) GetPaperDetail(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	paper, err := h.Repo.GetPaper(uint(id))
	if err != nil {
		response.Error(c, 404, "试卷不存在")
		return
	}
	questions, _ := h.Repo.ListPaperQuestions(uint(id))
	// 附带题目详情
	type pqDetail struct {
		model.PaperQuestion
		Question model.Question `json:"question"`
	}
	var details []pqDetail
	for _, pq := range questions {
		q, err := h.Repo.GetQuestion(pq.QuestionID)
		if err != nil || q == nil {
			continue // 题目已删除，跳过
		}
		details = append(details, pqDetail{PaperQuestion: pq, Question: *q})
	}
	response.Success(c, gin.H{"paper": paper, "questions": details})
}

// --- 考试 ---

type examReq struct {
	Name      string `json:"name" binding:"required"`
	PaperID   uint   `json:"paper_id" binding:"required"`
	RoomID    uint   `json:"room_id"`
	AntiCheat *bool  `json:"anti_cheat"`
}

func (h *Handler) CreateExam(c *gin.Context) {
	var req examReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	teacherID := uint(0)
	if uid, exists := c.Get("user_id"); exists {
		teacherID = uid.(uint)
	}

	antiCheat := true
	if req.AntiCheat != nil {
		antiCheat = *req.AntiCheat
	}

	exam := model.Exam{
		Name:       req.Name,
		PaperID:    req.PaperID,
		RoomID:     req.RoomID,
		TeacherID:  teacherID,
		Status:     "草稿",
		AntiCheat:  antiCheat,
		AccessCode: generateAccessCode(),
	}
	if err := h.Repo.CreateExam(&exam); err != nil {
		response.Error(c, 500, "创建考试失败")
		return
	}
	response.Success(c, exam)
}

func (h *Handler) ListExams(c *gin.Context) {
	list, _ := h.Repo.ListExams()
	response.Success(c, list)
}

func (h *Handler) GetExamDetail(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	exam, err := h.Repo.GetExam(uint(id))
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}
	paper, _ := h.Repo.GetPaper(exam.PaperID)
	questions, _ := h.Repo.ListPaperQuestions(exam.PaperID)
	violations, _ := h.Repo.ListExamViolations(uint(id))
	answers, _ := h.Repo.ListExamAnswers(uint(id), "")
	response.Success(c, gin.H{
		"exam":       exam,
		"paper":      paper,
		"questions":  questions,
		"violations": violations,
		"answers":    answers,
	})
}

// StartExam 开始考试（通知 Agent 进入考试模式）
func (h *Handler) StartExam(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	exam, err := h.Repo.GetExam(uint(id))
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}
	if exam.Status == "进行中" {
		response.Error(c, 400, "考试已在进行中")
		return
	}

	now := time.Now()
	paper, _ := h.Repo.GetPaper(exam.PaperID)
	duration := 60
	if paper != nil && paper.Duration > 0 {
		duration = paper.Duration
	}
	endAt := now.Add(time.Duration(duration) * time.Minute)

	h.Repo.UpdateExam(uint(id), map[string]interface{}{
		"status":   "进行中",
		"start_at": &now,
		"end_at":   &endAt,
	})

	// 通知所有在线 Agent 进入考试模式
	onlineAgents := h.Hub.OnlineAgents()
	for _, agentID := range onlineAgents {
		f := ws.Frame{
			Type:  "cmd",
			MsgID: fmt.Sprintf("exam_start_%d", id),
			Payload: mustJSON(map[string]interface{}{
				"action": "exam_start",
				"params": map[string]interface{}{
					"exam_id":    id,
					"anti_cheat": exam.AntiCheat,
					"duration":   duration,
					"end_at":     endAt.Format(time.RFC3339),
				},
			}),
		}
		h.Hub.SendToAgent(agentID, f)
	}

	response.Success(c, gin.H{"exam_id": id, "status": "进行中", "end_at": endAt})
}

// EndExam 结束考试
func (h *Handler) EndExam(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	_, err := h.Repo.GetExam(uint(id))
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}

	now := time.Now()
	h.Repo.UpdateExam(uint(id), map[string]interface{}{
		"status": "已结束",
		"end_at": &now,
	})

	// 通知 Agent 退出考试模式
	onlineAgents := h.Hub.OnlineAgents()
	for _, agentID := range onlineAgents {
		f := ws.Frame{
			Type:  "cmd",
			MsgID: fmt.Sprintf("exam_end_%d", id),
			Payload: mustJSON(map[string]interface{}{
				"action": "exam_end",
				"params": map[string]interface{}{"exam_id": id},
			}),
		}
		h.Hub.SendToAgent(agentID, f)
	}

	response.Success(c, gin.H{"exam_id": id, "status": "已结束"})
}

// --- 答题 & 判分 ---

type submitAnswerReq struct {
	ExamID     uint   `json:"exam_id" binding:"required"`
	AgentID    string `json:"agent_id" binding:"required"`
	StudentSno string `json:"student_sno"`
	Answers    []struct {
		QuestionID uint   `json:"question_id"`
		Answer     string `json:"answer"`
	} `json:"answers" binding:"required"`
}

// normalizeAnswer 标准化答案用于比较：去空格、统一大写、多选题排序
func normalizeAnswer(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ToUpper(s)
	// 多选题答案如 "BA" → 排序为 "AB"
	chars := []rune(s)
	sort.Slice(chars, func(i, j int) bool { return chars[i] < chars[j] })
	return string(chars)
}

func (h *Handler) SubmitAnswers(c *gin.Context) {
	var req submitAnswerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	exam, err := h.Repo.GetExam(req.ExamID)
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}
	if exam.Status != "进行中" {
		response.Error(c, 400, "考试未在进行中")
		return
	}

	// 防重复提交：检查该 agent 是否已提交过
	existing, _ := h.Repo.ListExamAnswers(req.ExamID, req.AgentID)
	if len(existing) > 0 {
		response.Error(c, 400, "该考生已提交过答案，不可重复提交")
		return
	}

	// 一次性加载试卷题目（避免 N+1 查询）
	pqs, _ := h.Repo.ListPaperQuestions(exam.PaperID)
	pqScoreMap := make(map[uint]float64)
	pqIDSet := make(map[uint]bool)
	for _, pq := range pqs {
		pqScoreMap[pq.QuestionID] = pq.Score
		pqIDSet[pq.QuestionID] = true
	}

	now := time.Now()
	var totalScore float64
	var results []model.ExamAnswer

	for _, a := range req.Answers {
		// 校验题目是否属于该试卷
		if !pqIDSet[a.QuestionID] {
			continue
		}
		q, err := h.Repo.GetQuestion(a.QuestionID)
		if err != nil {
			continue
		}
		// 自动判分（多选题排序后比较）
		correct := normalizeAnswer(a.Answer) == normalizeAnswer(q.Answer)
		score := 0.0
		if correct {
			score = pqScoreMap[a.QuestionID]
			if score == 0 {
				score = q.Score
			}
		}
		totalScore += score

		results = append(results, model.ExamAnswer{
			ExamID:      req.ExamID,
			AgentID:     req.AgentID,
			StudentSno:  req.StudentSno,
			QuestionID:  a.QuestionID,
			Answer:      a.Answer,
			Correct:     &correct,
			Score:       score,
			SubmittedAt: now,
		})
	}

	if len(results) > 0 {
		h.Repo.BatchCreateExamAnswers(results)
	}

	response.Success(c, gin.H{
		"exam_id":     req.ExamID,
		"agent_id":    req.AgentID,
		"total_score": totalScore,
		"submitted":   len(results),
	})
}

// --- 违规上报 ---

type violationReq struct {
	ExamID  uint   `json:"exam_id" binding:"required"`
	AgentID string `json:"agent_id" binding:"required"`
	Type    string `json:"type" binding:"required"`
	Detail  string `json:"detail"`
}

func (h *Handler) ReportViolation(c *gin.Context) {
	var req violationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	// 校验考试是否存在且在进行中
	exam, err := h.Repo.GetExam(req.ExamID)
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}
	if exam.Status != "进行中" {
		response.Error(c, 400, "考试未在进行中，不接受违规上报")
		return
	}
	v := model.ExamViolation{
		ExamID:  req.ExamID,
		AgentID: req.AgentID,
		Type:    req.Type,
		Detail:  req.Detail,
	}
	if err := h.Repo.CreateExamViolation(&v); err != nil {
		response.Error(c, 500, "记录失败")
		return
	}
	response.Success(c, v)
}

func (h *Handler) ListViolations(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	list, _ := h.Repo.ListExamViolations(uint(id))
	response.Success(c, list)
}

// GetExamResults 考试成绩汇总
func (h *Handler) GetExamResults(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	answers, _ := h.Repo.ListExamAnswers(uint(id), "")

	// 按 agent 汇总
	type scoreSummary struct {
		AgentID    string  `json:"agent_id"`
		StudentSno string  `json:"student_sno"`
		TotalScore float64 `json:"total_score"`
		Correct    int     `json:"correct"`
		Total      int     `json:"total"`
	}
	summaryMap := make(map[string]*scoreSummary)
	for _, a := range answers {
		key := a.AgentID
		if _, ok := summaryMap[key]; !ok {
			summaryMap[key] = &scoreSummary{AgentID: a.AgentID, StudentSno: a.StudentSno}
		}
		s := summaryMap[key]
		s.TotalScore += a.Score
		s.Total++
		if a.Correct != nil && *a.Correct {
			s.Correct++
		}
	}
	var summaries []scoreSummary
	for _, s := range summaryMap {
		summaries = append(summaries, *s)
	}
	response.Success(c, summaries)
}

// ============ CRUD 补全 ============

func (h *Handler) UpdateQuestion(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if id == 0 {
		response.BadRequest(c, "无效ID")
		return
	}
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	// 白名单字段
	allowed := map[string]bool{
		"type": true, "content": true, "options": true, "answer": true,
		"analysis": true, "score": true, "difficulty": true, "knowledge": true,
	}
	updates := map[string]interface{}{}
	for k, v := range body {
		if allowed[k] {
			updates[k] = v
		}
	}
	// 题型校验
	if t, ok := updates["type"].(string); ok && t != "" {
		validTypes := map[string]bool{"single": true, "multi": true, "judge": true, "fill": true}
		if !validTypes[t] {
			response.BadRequest(c, "题型无效")
			return
		}
	}
	if len(updates) == 0 {
		response.BadRequest(c, "无更新内容")
		return
	}
	if err := h.Repo.UpdateQuestion(uint(id), updates); err != nil {
		response.Error(c, 500, "更新失败")
		return
	}
	response.Success(c, nil)
}

func (h *Handler) DeleteQuestion(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if id == 0 {
		response.BadRequest(c, "无效ID")
		return
	}
	if err := h.Repo.DeleteQuestion(uint(id)); err != nil {
		response.Error(c, 500, "删除失败")
		return
	}
	response.Success(c, nil)
}

func (h *Handler) UpdatePaper(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if id == 0 {
		response.BadRequest(c, "无效ID")
		return
	}
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	// 只允许更新有限字段
	allowed := map[string]bool{"name": true, "duration": true, "strategy": true, "status": true}
	updates := map[string]interface{}{}
	for k, v := range body {
		if allowed[k] {
			updates[k] = v
		}
	}
	if len(updates) == 0 {
		response.BadRequest(c, "无有效更新字段")
		return
	}
	if err := h.Repo.UpdatePaper(uint(id), updates); err != nil {
		response.Error(c, 500, "更新失败")
		return
	}
	response.Success(c, nil)
}

func (h *Handler) DeletePaper(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if id == 0 {
		response.BadRequest(c, "无效ID")
		return
	}
	if err := h.Repo.DeletePaper(uint(id)); err != nil {
		response.Error(c, 500, "删除失败")
		return
	}
	response.Success(c, nil)
}

func (h *Handler) DeleteExam(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	if id == 0 {
		response.BadRequest(c, "无效ID")
		return
	}
	exam, err := h.Repo.GetExam(uint(id))
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}
	if exam.Status == "进行中" {
		response.Error(c, 400, "进行中的考试不可删除，请先结束")
		return
	}
	if err := h.Repo.DeleteExam(uint(id)); err != nil {
		response.Error(c, 500, "删除失败")
		return
	}
	response.Success(c, nil)
}

// ============ 学生端 API ============

// generateAccessCode 生成6位数字考试码
func generateAccessCode() string {
	return fmt.Sprintf("%06d", rand.Intn(1000000))
}

// StudentJoinExam 学生通过考试码+学号加入考试
type studentJoinReq struct {
	AccessCode  string `json:"access_code" binding:"required"`
	StudentSno  string `json:"student_sno" binding:"required"`
	StudentName string `json:"student_name"`
}

func (h *Handler) StudentJoinExam(c *gin.Context) {
	var req studentJoinReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	exam, err := h.Repo.GetExamByAccessCode(req.AccessCode)
	if err != nil {
		response.Error(c, 404, "考试码无效")
		return
	}
	if exam.Status != "进行中" {
		response.Error(c, 400, "考试未在进行中")
		return
	}
	paper, _ := h.Repo.GetPaper(exam.PaperID)
	response.Success(c, gin.H{
		"exam_id":    exam.ID,
		"exam_name":  exam.Name,
		"paper_id":   exam.PaperID,
		"duration":   paper.Duration,
		"anti_cheat": exam.AntiCheat,
	})
}

// StudentGetQuestions 学生获取试卷题目（不含答案和解析）
func (h *Handler) StudentGetQuestions(c *gin.Context) {
	examID, _ := strconv.ParseUint(c.Param("exam_id"), 10, 32)
	exam, err := h.Repo.GetExam(uint(examID))
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}
	if exam.Status != "进行中" {
		response.Error(c, 400, "考试未在进行中")
		return
	}
	pqs, _ := h.Repo.ListPaperQuestions(exam.PaperID)
	type studentQuestion struct {
		ID        uint    `json:"id"`
		Type      string  `json:"type"`
		Content   string  `json:"content"`
		Options   string  `json:"options"`
		Score     float64 `json:"score"`
		SortOrder int     `json:"sort_order"`
	}
	var questions []studentQuestion
	for _, pq := range pqs {
		q, err := h.Repo.GetQuestion(pq.QuestionID)
		if err != nil || q == nil {
			continue
		}
		questions = append(questions, studentQuestion{
			ID:        q.ID,
			Type:      q.Type,
			Content:   q.Content,
			Options:   q.Options,
			Score:     pq.Score,
			SortOrder: pq.SortOrder,
		})
	}
	response.Success(c, gin.H{
		"exam_id":   exam.ID,
		"exam_name": exam.Name,
		"questions": questions,
	})
}

// StudentSubmit 学生提交答案（带学号）
func (h *Handler) StudentSubmit(c *gin.Context) {
	var req submitAnswerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.StudentSno == "" {
		response.BadRequest(c, "学号不能为空")
		return
	}
	exam, err := h.Repo.GetExam(req.ExamID)
	if err != nil {
		response.Error(c, 404, "考试不存在")
		return
	}
	if exam.Status != "进行中" {
		response.Error(c, 400, "考试未在进行中")
		return
	}
	existing, _ := h.Repo.ListExamAnswersBySno(req.ExamID, req.StudentSno)
	if len(existing) > 0 {
		response.Error(c, 400, "该学号已提交过答案，不可重复提交")
		return
	}
	var questionIDs []uint
	for _, a := range req.Answers {
		questionIDs = append(questionIDs, a.QuestionID)
	}
	questions, _ := h.Repo.GetQuestionsBatch(questionIDs)
	qMap := make(map[uint]*model.Question)
	for i := range questions {
		qMap[questions[i].ID] = &questions[i]
	}
	var totalScore float64
	var answers []model.ExamAnswer
	for _, a := range req.Answers {
		q := qMap[a.QuestionID]
		if q == nil {
			continue
		}
		correct := normalizeAnswer(a.Answer) == normalizeAnswer(q.Answer)
		var score float64
		if correct {
			score = q.Score
		}
		totalScore += score
		answers = append(answers, model.ExamAnswer{
			ExamID:      req.ExamID,
			AgentID:     req.AgentID,
			StudentSno:  req.StudentSno,
			QuestionID:  a.QuestionID,
			Answer:      a.Answer,
			Correct:     &correct,
			Score:       score,
			SubmittedAt: time.Now(),
		})
	}
	if err := h.Repo.BatchCreateExamAnswers(answers); err != nil {
		response.Error(c, 500, "提交失败")
		return
	}
	response.Success(c, gin.H{
		"total_score":    totalScore,
		"question_count": len(answers),
	})
}

// StudentGetResult 学生查看自己的成绩
func (h *Handler) StudentGetResult(c *gin.Context) {
	examID, _ := strconv.ParseUint(c.Param("exam_id"), 10, 32)
	sno := c.Query("sno")
	if sno == "" {
		response.BadRequest(c, "学号不能为空")
		return
	}
	answers, err := h.Repo.ListExamAnswersBySno(uint(examID), sno)
	if err != nil || len(answers) == 0 {
		response.Error(c, 404, "未找到提交记录")
		return
	}
	var totalScore float64
	var correctCount int
	type answerDetail struct {
		QuestionID uint    `json:"question_id"`
		Answer     string  `json:"answer"`
		Correct    bool    `json:"correct"`
		Score      float64 `json:"score"`
	}
	var details []answerDetail
	for _, a := range answers {
		totalScore += a.Score
		if a.Correct != nil && *a.Correct {
			correctCount++
		}
		correct := false
		if a.Correct != nil {
			correct = *a.Correct
		}
		details = append(details, answerDetail{
			QuestionID: a.QuestionID,
			Answer:     a.Answer,
			Correct:    correct,
			Score:      a.Score,
		})
	}
	response.Success(c, gin.H{
		"total_score":   totalScore,
		"correct_count": correctCount,
		"total_count":   len(answers),
		"details":       details,
	})
}
