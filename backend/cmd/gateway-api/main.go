package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lab-teaching/backend/internal/api"
	"lab-teaching/backend/internal/broadcast"
	"lab-teaching/backend/internal/config"
	"lab-teaching/backend/internal/database"
	"lab-teaching/backend/internal/middleware"
	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/internal/repository"
	ws "lab-teaching/backend/internal/websocket"
	"lab-teaching/backend/pkg/auth"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfgPath := flag.String("c", "configs/dev.yaml", "配置文件路径")
	flag.Parse()
	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 数据库连接
	// V1.0-14: 支持 PostgreSQL / 人大金仓 KingbaseES / 达梦 DM8
	db, err := database.Open(&cfg.Database)
	if err != nil {
		log.Fatalf("数据库连接失败: %v", err)
	}
	// 连接池优化（支持200+终端并发）
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatalf("获取底层连接池失败: %v", err)
	}
	sqlDB.SetMaxOpenConns(cfg.Pool.DBMaxOpen)
	sqlDB.SetMaxIdleConns(cfg.Pool.DBMaxIdle)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.Pool.DBMaxLifeSec) * time.Second)
	log.Printf("数据库连接成功 (max_open=%d, max_idle=%d)", cfg.Pool.DBMaxOpen, cfg.Pool.DBMaxIdle)

	// 自动建表（与 migrations/001_init.sql 对应）
	if err := db.AutoMigrate(
		&model.User{}, &model.Audit{},
		&model.Room{}, &model.Device{}, &model.Seat{},
		&model.Course{}, &model.Clazz{}, &model.Student{}, &model.Schedule{},
		&model.AgentSession{}, &model.BroadcastTask{}, &model.DiskImage{}, &model.Policy{},
		&model.Question{}, &model.Exam{},
		&model.FileTask{}, &model.FileProgress{},
	); err != nil {
		log.Printf("AutoMigrate 警告(可忽略): %v", err)
	}
	// 确保文件收发/远程控制/考试表存在（独立建表，不受旧表约束冲突影响）
	db.AutoMigrate(&model.FileTask{}, &model.FileProgress{}, &model.RemoteSession{})
	db.AutoMigrate(&model.Question{}, &model.Paper{}, &model.PaperQuestion{},
		&model.Exam{}, &model.ExamAnswer{}, &model.ExamViolation{})
	// V1.x-16 教务对接表
	db.AutoMigrate(&model.EduSyncLog{}, &model.EduSyncConfig{})
	// V1.x-17 一卡通/门禁表
	db.AutoMigrate(&model.CardEvent{}, &model.AccessRule{})
	// V1.x-18 多校区表（Room 补 campus_id 列）
	db.AutoMigrate(&model.Campus{}, &model.Room{})
	// V1.x-20 运维升级表
	db.AutoMigrate(&model.UpgradeTask{}, &model.UpgradeNode{})
	// V1.x-21 系统设置 + 用户分组 + User.GroupID 列
	db.AutoMigrate(&model.SystemSetting{}, &model.UserGroup{}, &model.AgentLog{})
	// P0-1 控制指令结果表
	db.AutoMigrate(&api.CmdResult{})
	db.AutoMigrate(api.RegisterP1Models()...)
	db.AutoMigrate(api.RegisterP2Models()...)
	// 显式补列（AutoMigrate 对已存在表的零值默认列偶发跳过）
	if !db.Migrator().HasColumn(&model.User{}, "GroupID") {
		if err := db.Migrator().AddColumn(&model.User{}, "GroupID"); err != nil {
			// 回退：直接 DDL
			db.Exec("ALTER TABLE t_user ADD COLUMN IF NOT EXISTS group_id bigint DEFAULT 0")
		}
	}
	// V1.x-16: 课表教师可缺省，移除历史外键约束（teacher_id 改可空）
	if db.Migrator().HasConstraint(&model.Schedule{}, "t_schedule_teacher_id_fkey") {
		if err := db.Migrator().DropConstraint(&model.Schedule{}, "t_schedule_teacher_id_fkey"); err != nil {
			log.Printf("移除 t_schedule 教师外键约束失败(可忽略): %v", err)
		}
	}
	log.Println("数据表就绪")

	repo := repository.New(db)
	seedAdmin(repo)

	// WebSocket Hub（Agent 通信核心）
	hub := ws.NewHub()
	hub.SendBuf = cfg.Pool.WSSendBuf
	hub.ReadLimitMB = cfg.Pool.WSReadLimitMB
	remote := ws.NewRemoteManager()
	// 浏览器事件推送 Hub（d1: WebSocket 替代轮询；OnEvent 闭包引用，须先声明）
	browserHub := ws.NewBrowserHub()
	// P2-1 监控墙 Hub（缩略图帧广播）
	wallHub := ws.NewWallHub()
	hub.OnEvent = func(f ws.Frame, agentID string) {
		switch f.Type {
		case "register":
			handleRegister(repo, f, agentID)
			// d1: Agent 上线 → 浏览器事件推送
			browserHub.Broadcast("agent_status", gin.H{"agent_id": agentID, "online": true})
		case "heartbeat":
			repo.SetAgentOnline(agentID, true)
		case "event":
			log.Printf("[event] agent=%s payload=%s", agentID, string(f.Payload))
		}
	}
	// 教师端输入 → 包装为 cmd 帧转发给 Agent
	remote.SetInputHandler(func(agentID string, data []byte) {
		var inputParams json.RawMessage
		if err := json.Unmarshal(data, &inputParams); err != nil {
			return
		}
		payload, _ := json.Marshal(map[string]interface{}{
			"action": "remote_input",
			"params": inputParams,
		})
		f := ws.Frame{
			Type:    "cmd",
			MsgID:   "remote_input",
			Payload: payload,
		}
		hub.SendToAgent(agentID, f)
	})
	// Agent 二进制帧（屏幕帧）→ 中继给教师查看器；缩略图帧 → 监控墙广播
	hub.OnBinary = func(data []byte, agentID string) {
		if ws.IsThumbFrame(data) {
			wallHub.Relay(agentID, data)
			return
		}
		remote.RelayFrame(agentID, data)
	}
	go hub.Run()

	// 启动浏览器事件推送 Hub（d1）
	go browserHub.Run()

	// 心跳超时监控（60秒无心跳→标记离线+审计日志+浏览器事件推送）
	hbStop := make(chan struct{})
	go middleware.StartHeartbeatMonitor(repo, hbStop)
	// 离线事件钩子：心跳监控标记离线后推送 agent_status 事件
	middleware.SetOfflineCallback(func(agentID string) {
		browserHub.Broadcast("agent_status", gin.H{"agent_id": agentID, "online": false})
	})

	// Gin 路由
	gin.SetMode(cfg.Server.Mode)
	r := gin.Default()
	r.Use(cors.Default(), middleware.CORSMiddleware(), middleware.RateLimiter(), middleware.MetricsCollector())

	// 初始化安全策略
	middleware.SetLoginPolicy(cfg.Security.LoginMaxFail, cfg.Security.LoginLockMin)
	middleware.SetRateLimit(cfg.Security.RateLimitRPS)

	r.GET("/health", middleware.HealthDetailHandler(func() bool {
		return sqlDB.Ping() == nil
	}))
	r.GET("/metrics", middleware.MetricsHandler)

	h := api.New(repo, hub, remote, broadcast.NewManager(), cfg.JWT.Secret, cfg.JWT.ExpireHours, cfg)
	// P0-1: 指令 ack 实时推送通道 + ack 帧处理
	h.BrowserHub = browserHub
	// P2-3: 定时电源任务调度器（进程存活期间常驻）
	go h.RunDuePowerTasks(make(chan struct{}))
	prevOnEvent := hub.OnEvent
	hub.OnEvent = func(f ws.Frame, agentID string) {
		if f.Type == "ack" {
			h.HandleCmdAck(f, agentID)
			return
		}
		prevOnEvent(f, agentID)
	}
	r.GET("/api/v1/auth/captcha", h.GetCaptcha)
	// P3-2: OpenAPI 文档端点
	r.GET("/api/v1/openapi.json", func(c *gin.Context) {
		c.JSON(200, api.OpenAPISpec())
	})

	// d1: 周期指标推送（每 15s 向所有浏览器订阅者推送 metrics 快照）
	metricsStop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-metricsStop:
				return
			case <-ticker.C:
				if browserHub.Count() > 0 {
					browserHub.Broadcast("metrics", h.AlertsPayload())
				}
			}
		}
	}()

	// Agent WebSocket 端点
	r.GET("/agent/ws", func(c *gin.Context) {
		client, err := hub.Upgrade(c.Writer, c.Request)
		if err != nil {
			return
		}
		go client.WritePump()
		client.ReadPump()
	})

	// 教师端远程桌面查看器 WebSocket
	r.GET("/remote/ws", func(c *gin.Context) {
		agentID := c.Query("agent_id")
		if agentID == "" {
			c.JSON(400, gin.H{"error": "agent_id required"})
			return
		}
		remote.HandleViewerWS(c.Writer, c.Request, agentID)
	})

	// P2-1: 监控墙 WebSocket（JWT query 鉴权）
	r.GET("/ws/wall", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(401, gin.H{"error": "token required"})
			return
		}
		if _, err := auth.ParseToken(token); err != nil {
			c.JSON(401, gin.H{"error": "invalid token"})
			return
		}
		wallHub.HandleWS(c.Writer, c.Request)
	})

	// d1: 浏览器事件推送 WebSocket（JWT query 鉴权，浏览器 WS 无法设 Header）
	r.GET("/ws/events", func(c *gin.Context) {
		token := c.Query("token")
		if token == "" {
			c.JSON(401, gin.H{"error": "token required"})
			return
		}
		claims, err := auth.ParseToken(token)
		if err != nil {
			c.JSON(401, gin.H{"error": "invalid token"})
			return
		}
		conn, err := browserHub.Upgrade(c.Writer, c.Request)
		if err != nil {
			return
		}
		browserHub.Register(conn)
		// 连接后立即推送一次初始快照，避免前端首屏空白
		go func() {
			browserHub.Broadcast("metrics", h.AlertsPayload())
		}()
		_ = claims
		go conn.WritePump()
		conn.ReadPump()
	})

	v1 := r.Group("/api/v1")
	{
		v1.POST("/auth/login", h.Login)

		// V1.x-21 系统设置（公开读，登录页也需要标题/Logo/主题）
		v1.GET("/settings", h.GetSystemSettings)
		// V1.x-21 Logo 静态文件（公开读）
		v1.GET("/static/logo/:file", h.ServeStaticLogo)

		// Agent 文件下载（无需认证，Agent直接拉取）
		v1.GET("/files/:id/download", h.DownloadFile)
		// Agent 进度上报（无需认证）
		v1.POST("/files/progress", h.ReportFileProgress)

		// P1: Agent 免登录端点（对齐 exam/violation 模式）
		v1.POST("/msg/read", h.MessageReadReport)
		v1.POST("/screenshots/upload", h.UploadScreenshot)
		v1.GET("/static/screenshots/:file", h.ServeScreenshot)
		// P2-5: Agent 作业收集上传（免登录）
		v1.POST("/collect/upload", h.HandleCollectUploadNoAuth)
		// P2-5: 作业文件下载（浏览器直链，对齐 /files/:id/download 免登录模式）
		v1.GET("/collect/download/:file_id", h.DownloadCollectFile)

		// V1.0-13 启动策略查询（无需认证，PXE 引导链中使用）
		v1.GET("/boot/policy", h.QueryBootPolicy)
		v1.GET("/boot/ipxe", h.IPXEBootScript)
		v1.GET("/boot/report", h.ReportBootResult)

		// V1.x-16 教务系统推送（X-Sync-Key 头鉴权，免登录）
		v1.POST("/edu/push/students", h.SyncStudentsPush)
		v1.POST("/edu/push/courses", h.SyncCoursesPush)
		v1.POST("/edu/push/classes", h.SyncClassesPush)
		v1.POST("/edu/push/schedules", h.SyncSchedulesPush)

		// V1.x-17 一卡通/门禁事件推送（X-Sync-Key 头鉴权，免登录）
		v1.POST("/access/card-event", h.PushCardEvent)

		authed := v1.Group("")
		authed.Use(middleware.Auth(), middleware.DenyViewerWrites(), middleware.AuditLog(repo))
		{
			authed.GET("/auth/profile", h.Profile)
			authed.PUT("/auth/password", h.ChangePassword)
			authed.GET("/stats/dashboard", h.Dashboard)

			// V1.x-16 教务对接（管理端 JWT）
			authed.GET("/edu/scores", h.QueryExamScores)
		authed.GET("/edu/scores/export", h.ExportExamScoresCSV)
		authed.POST("/edu/sync-configs", h.CreateSyncConfig)
		authed.GET("/edu/sync-configs", h.ListSyncConfigs)
		authed.GET("/edu/sync-logs", h.ListSyncLogs)

		// V1.x-17 一卡通/门禁（管理端 JWT）
		authed.GET("/access/card-events", h.ListCardEvents)
		authed.POST("/access/rules", h.CreateAccessRule)
		authed.GET("/access/rules", h.ListAccessRules)
		authed.DELETE("/access/rules/:id", h.DeleteAccessRule)
		authed.PUT("/access/bind", h.BindCard)

		// V1.x-18 多校区管理
		authed.POST("/campuses", h.CreateCampus)
		authed.GET("/campuses", h.ListCampuses)
		authed.PUT("/campuses/:id", h.UpdateCampus)
		authed.DELETE("/campuses/:id", h.DeleteCampus)
		authed.GET("/campuses/:id/rooms", h.ListCampusRooms)
		authed.PUT("/campuses/:id/rooms/:room_id", h.AssignRoomCampus)
		authed.DELETE("/campuses/rooms/:room_id", h.UnassignRoomCampus)
		authed.GET("/campuses/:id/dashboard", h.CampusDashboard)
		authed.GET("/dashboard/global", h.GlobalDashboard)

			// V1.x-19 授权 License
			authed.GET("/license", h.GetLicenseStatus)
			authed.POST("/license", h.UploadLicense)
			authed.POST("/license/issue", h.IssueLicense)

			// V1.x-21 系统设置（管理员）
			authed.PUT("/settings", middleware.RequireRoles("admin"), h.UpdateSystemSettings)
			authed.POST("/settings/logo", middleware.RequireRoles("admin"), h.UploadLogo)
			// V1.x-21 用户分组（管理员）
			authed.GET("/groups", h.ListGroups)
			authed.POST("/groups", middleware.RequireRoles("admin"), h.CreateGroup)
			authed.PUT("/groups/:id", middleware.RequireRoles("admin"), h.UpdateGroup)
			authed.DELETE("/groups/:id", middleware.RequireRoles("admin"), h.DeleteGroup)
			// V1.x-21 用户管理（管理员）
			authed.GET("/users", middleware.RequireRoles("admin"), h.ListUsers)
			authed.POST("/users", middleware.RequireRoles("admin"), h.CreateUser)
			authed.PUT("/users/:id", middleware.RequireRoles("admin"), h.UpdateUser)
			authed.DELETE("/users/:id", middleware.RequireRoles("admin"), h.DeleteUser)

		// V1.x-20 运维工具：Agent 批量升级
		authed.POST("/ops/upgrade", h.CreateUpgradeTask)
		authed.GET("/ops/upgrade", h.ListUpgradeTasks)
		authed.GET("/ops/upgrade/:id", h.GetUpgradeTask)
		authed.POST("/ops/upgrade/:id/start", h.StartUpgradeTask)
		authed.POST("/ops/upgrade/node/report", h.ReportUpgradeNode)
		authed.POST("/ops/upgrade/:id/rollback", h.RollbackUpgradeTask)
		authed.POST("/ops/verify-password", h.VerifyPassword)

		// V1.x-22 监控告警（聚合健康+告警规则）
		authed.GET("/alerts", h.Alerts)

		authed.GET("/rooms", h.ListRooms)
			authed.POST("/rooms", h.CreateRoom)
			authed.PUT("/rooms/:id", h.UpdateRoom)
			authed.DELETE("/rooms/:id", h.DeleteRoom)
			authed.GET("/rooms/:id/seats", h.ListSeats)
			authed.PUT("/rooms/:id/boot-mode", h.SetRoomBootMode)
			authed.PUT("/seats/:id/boot-mode", h.SetSeatBootMode)
			authed.POST("/boot/batch", h.BatchSetBootMode)

			authed.GET("/devices", h.ListDevices)

			authed.GET("/agents/online", h.AgentOnline)

			authed.POST("/ctrl/broadcast", h.StartBroadcast)
			authed.POST("/ctrl/broadcast/:id/stop", h.StopBroadcast)
			authed.GET("/ctrl/broadcast/status", h.BroadcastStatus)
			authed.GET("/ctrl/broadcast", h.ListBroadcasts)
			authed.POST("/ctrl/command", h.SendCommandV2)
			authed.GET("/ctrl/results", h.ListCmdResults)

			// P1: 消息广播 / 屏幕截图 / 桌面服务
			authed.POST("/msg/send", h.SendMessage)
			authed.GET("/msg/messages", h.ListMessages)
			authed.GET("/screenshots", h.ListScreenshots)
			authed.GET("/desktop-services", h.ListDesktopServices)
			authed.POST("/desktop-services", middleware.RequireRoles("admin"), h.CreateDesktopService)
			authed.PUT("/desktop-services/:id", middleware.RequireRoles("admin"), h.UpdateDesktopService)
			authed.DELETE("/desktop-services/:id", middleware.RequireRoles("admin"), h.DeleteDesktopService)
			authed.POST("/desktop-services/:id/launch", h.LaunchDesktopService)

			// P2: 监控墙 / WOL / 定时电源任务 / 作业收集
			authed.GET("/wall/seats", h.WallSeats)
			authed.POST("/wall/start", h.WallStart)
			authed.POST("/wall/stop", h.WallStop)
			authed.POST("/wol", h.WakeOnLan)
			authed.GET("/power-tasks", h.ListPowerTasks)
			authed.POST("/power-tasks", middleware.RequireRoles("admin"), h.CreatePowerTask)
			authed.DELETE("/power-tasks/:id", middleware.RequireRoles("admin"), h.DeletePowerTask)
			authed.GET("/collect/tasks", h.ListCollectTasks)
			authed.POST("/collect/tasks", h.CreateCollectTask)
			authed.GET("/collect/tasks/:id", h.GetCollectTask)

			// 文件收发
			authed.POST("/files/upload", h.UploadFile)
			authed.POST("/files/distribute", h.DistributeFile)
			authed.GET("/files/tasks", h.ListFileTasks)
			authed.GET("/files/tasks/:id/progress", h.FileTaskProgress)

			// 远程控制
			authed.POST("/remote/start", h.StartRemote)
			authed.POST("/remote/:id/stop", h.StopRemote)
			authed.GET("/remote/sessions", h.ListRemoteSessions)
			authed.GET("/remote/status", h.RemoteStatus)

			authed.GET("/images", h.ListImages)
			authed.GET("/courses", h.ListCourses)
			authed.POST("/courses", h.CreateCourse)
			authed.PUT("/courses/:id", h.UpdateCourse)
			authed.DELETE("/courses/:id", h.DeleteCourse)
			authed.GET("/clazzes", h.ListClazzes)
			authed.POST("/clazzes", h.CreateClazz)
			authed.PUT("/clazzes/:id", h.UpdateClazz)
			authed.DELETE("/clazzes/:id", h.DeleteClazz)
			authed.GET("/students", h.SearchStudents)
			authed.POST("/students", h.CreateStudent)
			authed.PUT("/students/:id", h.UpdateStudent)
			authed.DELETE("/students/:id", h.DeleteStudent)
			authed.GET("/schedules", h.ListSchedules)
			authed.POST("/schedules", h.CreateSchedule)
			authed.PUT("/schedules/:id", h.UpdateSchedule)
			authed.DELETE("/schedules/:id", h.DeleteSchedule)
			authed.GET("/audit/logs", h.ListAudits)
			authed.GET("/audit/logs/export", h.ExportAudits)
			authed.GET("/system/resources", h.SystemResources)
			authed.GET("/system/alert-rules", h.GetAlertRules)
			authed.PUT("/system/alert-rules", middleware.RequireRoles("admin"), h.UpdateAlertRules)
			authed.GET("/system/cmd-whitelist", h.GetCmdWhitelist)
			authed.PUT("/system/cmd-whitelist", middleware.RequireRoles("admin"), h.UpdateCmdWhitelist)
			authed.POST("/system/backup", middleware.RequireRoles("admin"), h.TriggerBackup)
			authed.GET("/agent/logs", h.ListAgentLogs)
			authed.POST("/agent/logs", h.UploadAgentLog)

			// 考试域
			authed.POST("/questions", h.CreateQuestion)
			authed.GET("/questions", h.ListQuestions)
			authed.GET("/questions/:id", h.GetQuestion)
			authed.PUT("/questions/:id", h.UpdateQuestion)
			authed.DELETE("/questions/:id", h.DeleteQuestion)

			authed.POST("/papers", h.CreatePaper)
			authed.GET("/papers", h.ListPapers)
			authed.GET("/papers/:id", h.GetPaperDetail)
			authed.PUT("/papers/:id", h.UpdatePaper)
			authed.DELETE("/papers/:id", h.DeletePaper)

			authed.POST("/exams", h.CreateExam)
			authed.GET("/exams", h.ListExams)
			authed.GET("/exams/:id", h.GetExamDetail)
			authed.DELETE("/exams/:id", h.DeleteExam)
			authed.POST("/exams/:id/start", h.StartExam)
			authed.POST("/exams/:id/end", h.EndExam)
			authed.GET("/exams/:id/results", h.GetExamResults)
			authed.GET("/exams/:id/violations", h.ListViolations)

			// === V0.9 增强路由 ===
			// 多机房广播
			authed.POST("/broadcast/multiroom/start", h.MultiRoomBroadcastStart)
			authed.POST("/broadcast/multiroom/:id/stop", h.MultiRoomBroadcastStop)
			authed.GET("/broadcast/multiroom", h.MultiRoomBroadcastList)

			// 远程桌面增强
			authed.POST("/remote/patrol/start", h.PatrolStart)
			authed.POST("/remote/patrol/:id/stop", h.PatrolStop)
			authed.GET("/remote/patrols", h.PatrolList)
			authed.POST("/remote/input/lock", h.InputLock)
			authed.POST("/remote/input/unlock", h.InputUnlock)
			authed.POST("/remote/screen/lock", h.ScreenLock)
			authed.POST("/remote/screen/unlock", h.ScreenUnlock)
			authed.POST("/remote/usb", h.USBControl)
			authed.GET("/remote/locks", h.GetLocks)

			// 文件收发增强（分片上传）
			authed.POST("/files/chunk/init", h.InitChunkUpload)
			authed.POST("/files/chunk/:file_id/:chunk_index", h.UploadChunk)
			authed.POST("/files/chunk/:file_id/merge", h.MergeChunks)
			authed.GET("/files/chunk/:file_id/progress", h.UploadProgress)

			// 作业归档
			authed.POST("/homework/submit", h.SubmitHomework)
			authed.GET("/homework", h.ListHomework)

			// 课堂管理（签到）
			authed.POST("/classroom/checkin", h.StudentCheckIn)
			authed.GET("/classroom/checkins", h.ListCheckins)

			// 考试增强（V0.9-9）
			authed.POST("/papers/random", h.RandomGeneratePaper)
			authed.GET("/exams/:id/timer", h.GetExamTimer)
			authed.POST("/exams/:id/extend", h.ExtendExam)
			authed.POST("/exams/:id/regrade", h.ReGradeExam)
			authed.GET("/exams/:id/export", h.ExportExamScores)
			authed.GET("/exams/:id/analysis", h.GetExamAnalysis)
		}

		// 学生端/Agent 端（无需教师认证）
		r.POST("/api/v1/exam/submit", h.SubmitAnswers)
		r.POST("/api/v1/exam/violation", h.ReportViolation)

		// 学生端考试接口
		r.POST("/api/v1/student/join", h.StudentJoinExam)
		r.GET("/api/v1/student/exam/:exam_id/questions", h.StudentGetQuestions)
		r.POST("/api/v1/student/submit", h.StudentSubmit)
		r.GET("/api/v1/student/exam/:exam_id/result", h.StudentGetResult)
	}

	// 优雅关闭（支持 SIGTERM/SIGINT 信号）
	srv := &http.Server{
		Addr:    cfg.Server.Port,
		Handler: r,
	}
	go func() {
		log.Printf("服务启动于 %s", cfg.Server.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("服务启动失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("收到关闭信号，正在优雅退出...")
	close(hbStop)
	close(metricsStop)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("服务关闭失败: %v", err)
	}
	sqlDB.Close()
	log.Println("服务已安全关闭")
}

// seedAdmin 自动创建默认管理员 admin（初始密码由 LAB_ADMIN_INIT_PASSWORD 指定，默认 ChangeMe@123）
func seedAdmin(repo *repository.Repo) {
	if repo.CountUsers() > 0 {
		return
	}
	initPwd := os.Getenv("LAB_ADMIN_INIT_PASSWORD")
	if initPwd == "" {
		initPwd = "ChangeMe@123"
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte(initPwd), bcrypt.DefaultCost)
	repo.CreateUser(&model.User{
		Username:     "admin",
		PasswordHash: string(hash),
		RealName:     "系统管理员",
		Role:         "admin",
		Status:       "active",
	})
	log.Println("已创建默认管理员 admin（初始密码见 LAB_ADMIN_INIT_PASSWORD，默认 ChangeMe@123，请尽快修改）")
}

// handleRegister 处理 Agent 注册帧，落库
func handleRegister(repo *repository.Repo, f ws.Frame, agentID string) {
	var p struct {
		AgentID  string `json:"agent_id"`
		OS       string `json:"os"`
		Arch     string `json:"arch"`
		MAC      string `json:"mac"`
		Version  string `json:"version"`
		BootMode string `json:"boot_mode"`
	}
	json.Unmarshal(f.Payload, &p)
	now := time.Now()
	repo.UpsertAgentSession(&model.AgentSession{
		AgentID:       agentID,
		OS:            p.OS,
		Arch:          p.Arch,
		MAC:           p.MAC,
		Version:       p.Version,
		BootMode:      p.BootMode,
		Online:        true,
		LastHeartbeat: &now,
	})
}
