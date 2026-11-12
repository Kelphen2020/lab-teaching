package api

// openapi.go — P3-2 OpenAPI 3.0 规范生成器
// 从端点定义表驱动生成 spec，供 GET /api/v1/openapi.json 与前端接口文档页使用。
// 新增接口时只需在 endpointDefs 中追加一行即可同步出现在文档里。

import "time"

type endpointDef struct {
	Method      string
	Path        string // OpenAPI 路径（:param 转 {param}）
	Summary     string
	Tags        []string
	Auth        bool // 是否需要 Bearer 令牌
	SummaryOnly bool // 仅列出，不写详细 schema
}

// bearerRef 公共安全方案引用
func bearerRef() []map[string][]string { return []map[string][]string{{"bearerAuth": {}}} }

// endpointDefs 全量接口清单（与 cmd/gateway-api/main.go 路由保持一致）
var endpointDefs = []endpointDef{
	// ===== 认证 =====
	{Method: "POST", Path: "/auth/login", Summary: "登录获取令牌", Tags: []string{"认证"}, Auth: false},
	{Method: "GET", Path: "/auth/captcha", Summary: "获取图形验证码（登录失败≥3次后要求）", Tags: []string{"认证"}, Auth: false},
	{Method: "GET", Path: "/auth/profile", Summary: "当前用户信息", Tags: []string{"认证"}, Auth: true},
	{Method: "PUT", Path: "/auth/password", Summary: "修改当前用户密码", Tags: []string{"认证"}, Auth: true},

	// ===== 用户管理 =====
	{Method: "GET", Path: "/users", Summary: "用户列表", Tags: []string{"用户"}, Auth: true},
	{Method: "POST", Path: "/users", Summary: "创建用户", Tags: []string{"用户"}, Auth: true},
	{Method: "PUT", Path: "/users/{id}", Summary: "更新用户", Tags: []string{"用户"}, Auth: true},
	{Method: "DELETE", Path: "/users/{id}", Summary: "删除用户", Tags: []string{"用户"}, Auth: true},

	// ===== 基础数据 =====
	{Method: "GET", Path: "/rooms", Summary: "机房列表", Tags: []string{"基础数据"}, Auth: true},
	{Method: "POST", Path: "/rooms", Summary: "创建机房", Tags: []string{"基础数据"}, Auth: true},
	{Method: "PUT", Path: "/rooms/{id}", Summary: "更新机房", Tags: []string{"基础数据"}, Auth: true},
	{Method: "DELETE", Path: "/rooms/{id}", Summary: "删除机房", Tags: []string{"基础数据"}, Auth: true},
	{Method: "GET", Path: "/rooms/{id}/seats", Summary: "机房座位列表", Tags: []string{"基础数据"}, Auth: true},
	{Method: "PUT", Path: "/rooms/{id}/boot-mode", Summary: "设置机房启动模式", Tags: []string{"基础数据"}, Auth: true},
	{Method: "PUT", Path: "/seats/{id}/boot-mode", Summary: "设置座位启动模式", Tags: []string{"基础数据"}, Auth: true},
	{Method: "GET", Path: "/clazzes", Summary: "班级列表", Tags: []string{"基础数据"}, Auth: true},
	{Method: "POST", Path: "/clazzes", Summary: "创建班级", Tags: []string{"基础数据"}, Auth: true},
	{Method: "PUT", Path: "/clazzes/{id}", Summary: "更新班级", Tags: []string{"基础数据"}, Auth: true},
	{Method: "DELETE", Path: "/clazzes/{id}", Summary: "删除班级", Tags: []string{"基础数据"}, Auth: true},
	{Method: "GET", Path: "/courses", Summary: "课程列表", Tags: []string{"基础数据"}, Auth: true},
	{Method: "POST", Path: "/courses", Summary: "创建课程", Tags: []string{"基础数据"}, Auth: true},
	{Method: "PUT", Path: "/courses/{id}", Summary: "更新课程", Tags: []string{"基础数据"}, Auth: true},
	{Method: "DELETE", Path: "/courses/{id}", Summary: "删除课程", Tags: []string{"基础数据"}, Auth: true},
	{Method: "GET", Path: "/students", Summary: "学生列表", Tags: []string{"基础数据"}, Auth: true},
	{Method: "POST", Path: "/students", Summary: "创建学生", Tags: []string{"基础数据"}, Auth: true},
	{Method: "PUT", Path: "/students/{id}", Summary: "更新学生", Tags: []string{"基础数据"}, Auth: true},
	{Method: "DELETE", Path: "/students/{id}", Summary: "删除学生", Tags: []string{"基础数据"}, Auth: true},
	{Method: "GET", Path: "/groups", Summary: "分组列表", Tags: []string{"基础数据"}, Auth: true},
	{Method: "POST", Path: "/groups", Summary: "创建分组", Tags: []string{"基础数据"}, Auth: true},
	{Method: "PUT", Path: "/groups/{id}", Summary: "更新分组", Tags: []string{"基础数据"}, Auth: true},
	{Method: "DELETE", Path: "/groups/{id}", Summary: "删除分组", Tags: []string{"基础数据"}, Auth: true},
	{Method: "GET", Path: "/devices", Summary: "设备列表", Tags: []string{"基础数据"}, Auth: true},
	{Method: "GET", Path: "/schedules", Summary: "课表列表", Tags: []string{"基础数据"}, Auth: true},
	{Method: "POST", Path: "/schedules", Summary: "创建课表", Tags: []string{"基础数据"}, Auth: true},
	{Method: "PUT", Path: "/schedules/{id}", Summary: "更新课表", Tags: []string{"基础数据"}, Auth: true},
	{Method: "DELETE", Path: "/schedules/{id}", Summary: "删除课表", Tags: []string{"基础数据"}, Auth: true},

	// ===== 多校区 =====
	{Method: "GET", Path: "/campuses", Summary: "校区列表", Tags: []string{"多校区"}, Auth: true},
	{Method: "POST", Path: "/campuses", Summary: "创建校区", Tags: []string{"多校区"}, Auth: true},
	{Method: "PUT", Path: "/campuses/{id}", Summary: "更新校区", Tags: []string{"多校区"}, Auth: true},
	{Method: "DELETE", Path: "/campuses/{id}", Summary: "删除校区", Tags: []string{"多校区"}, Auth: true},
	{Method: "GET", Path: "/campuses/{id}/rooms", Summary: "校区下机房列表", Tags: []string{"多校区"}, Auth: true},
	{Method: "PUT", Path: "/campuses/{id}/rooms/{room_id}", Summary: "调整校区机房", Tags: []string{"多校区"}, Auth: true},
	{Method: "DELETE", Path: "/campuses/rooms/{room_id}", Summary: "从校区移除机房", Tags: []string{"多校区"}, Auth: true},
	{Method: "GET", Path: "/campuses/{id}/dashboard", Summary: "校区看板数据", Tags: []string{"多校区"}, Auth: true},

	// ===== Agent 与远程控制 =====
	{Method: "GET", Path: "/agents/online", Summary: "Agent 在线状态（含 online_count/sessions）", Tags: []string{"Agent"}, Auth: true},
	{Method: "GET", Path: "/agent/logs", Summary: "查询 Agent 日志", Tags: []string{"Agent"}, Auth: true},
	{Method: "POST", Path: "/agent/logs", Summary: "Agent 上报日志（免登录）", Tags: []string{"Agent"}, Auth: false},
	{Method: "POST", Path: "/ctrl/command", Summary: "下发控制指令（lock/shutdown/reboot/snapshot/message 等）", Tags: []string{"控制"}, Auth: true},
	{Method: "GET", Path: "/ctrl/results", Summary: "指令 ack 结果列表", Tags: []string{"控制"}, Auth: true},
	{Method: "GET", Path: "/ctrl/broadcast", Summary: "查询广播任务", Tags: []string{"控制"}, Auth: true},
	{Method: "POST", Path: "/ctrl/broadcast", Summary: "发起广播", Tags: []string{"控制"}, Auth: true},
	{Method: "GET", Path: "/ctrl/broadcast/status", Summary: "广播状态", Tags: []string{"控制"}, Auth: true},
	{Method: "POST", Path: "/ctrl/broadcast/{id}/stop", Summary: "停止广播", Tags: []string{"控制"}, Auth: true},
	{Method: "GET", Path: "/remote/status", Summary: "远程会话状态", Tags: []string{"远程控制"}, Auth: true},
	{Method: "GET", Path: "/remote/sessions", Summary: "远程会话列表", Tags: []string{"远程控制"}, Auth: true},
	{Method: "GET", Path: "/remote/locks", Summary: "锁屏状态", Tags: []string{"远程控制"}, Auth: true},
	{Method: "GET", Path: "/remote/patrols", Summary: "巡屏任务列表", Tags: []string{"远程控制"}, Auth: true},
	{Method: "POST", Path: "/remote/start", Summary: "发起远程控制会话", Tags: []string{"远程控制"}, Auth: true},
	{Method: "POST", Path: "/remote/{id}/stop", Summary: "停止远程会话", Tags: []string{"远程控制"}, Auth: true},
	{Method: "POST", Path: "/remote/patrol/start", Summary: "发起巡屏", Tags: []string{"远程控制"}, Auth: true},
	{Method: "POST", Path: "/remote/patrol/{id}/stop", Summary: "停止巡屏", Tags: []string{"远程控制"}, Auth: true},
	{Method: "POST", Path: "/remote/screen/lock", Summary: "锁屏", Tags: []string{"远程控制"}, Auth: true},
	{Method: "POST", Path: "/remote/screen/unlock", Summary: "解锁屏幕", Tags: []string{"远程控制"}, Auth: true},
	{Method: "POST", Path: "/remote/input/lock", Summary: "锁定输入", Tags: []string{"远程控制"}, Auth: true},
	{Method: "POST", Path: "/remote/input/unlock", Summary: "解锁输入", Tags: []string{"远程控制"}, Auth: true},
	{Method: "POST", Path: "/remote/usb", Summary: "USB 管控", Tags: []string{"远程控制"}, Auth: true},

	// ===== 消息与截图 =====
	{Method: "POST", Path: "/msg/send", Summary: "发送文本消息/弹窗", Tags: []string{"消息"}, Auth: true},
	{Method: "GET", Path: "/msg/messages", Summary: "消息历史", Tags: []string{"消息"}, Auth: true},
	{Method: "POST", Path: "/msg/read", Summary: "消息已读上报（免登录）", Tags: []string{"消息"}, Auth: false},
	{Method: "GET", Path: "/screenshots", Summary: "截图库列表", Tags: []string{"截图"}, Auth: true},
	{Method: "POST", Path: "/screenshots/upload", Summary: "截图上传（免登录）", Tags: []string{"截图"}, Auth: false},

	// ===== 桌面服务 =====
	{Method: "GET", Path: "/desktop-services", Summary: "桌面服务/应用库列表", Tags: []string{"桌面服务"}, Auth: true},
	{Method: "POST", Path: "/desktop-services", Summary: "创建桌面服务", Tags: []string{"桌面服务"}, Auth: true},
	{Method: "PUT", Path: "/desktop-services/{id}", Summary: "更新桌面服务", Tags: []string{"桌面服务"}, Auth: true},
	{Method: "DELETE", Path: "/desktop-services/{id}", Summary: "删除桌面服务", Tags: []string{"桌面服务"}, Auth: true},
	{Method: "POST", Path: "/desktop-services/{id}/launch", Summary: "一键启动桌面服务", Tags: []string{"桌面服务"}, Auth: true},

	// ===== 电源与开机 =====
	{Method: "POST", Path: "/wol", Summary: "WOL 远程开机（magic packet）", Tags: []string{"电源"}, Auth: true},
	{Method: "GET", Path: "/power-tasks", Summary: "定时电源任务列表", Tags: []string{"电源"}, Auth: true},
	{Method: "POST", Path: "/power-tasks", Summary: "创建定时电源任务", Tags: []string{"电源"}, Auth: true},
	{Method: "DELETE", Path: "/power-tasks/{id}", Summary: "删除定时电源任务", Tags: []string{"电源"}, Auth: true},
	{Method: "POST", Path: "/boot/batch", Summary: "批量开机", Tags: []string{"电源"}, Auth: true},

	// ===== 监控墙 =====
	{Method: "GET", Path: "/wall/seats", Summary: "监控墙座位布局", Tags: []string{"监控墙"}, Auth: true},
	{Method: "POST", Path: "/wall/start", Summary: "启动监控墙缩略图采集", Tags: []string{"监控墙"}, Auth: true},
	{Method: "POST", Path: "/wall/stop", Summary: "停止监控墙", Tags: []string{"监控墙"}, Auth: true},

	// ===== 作业收集 =====
	{Method: "GET", Path: "/collect/tasks", Summary: "作业收集任务列表", Tags: []string{"作业收集"}, Auth: true},
	{Method: "GET", Path: "/collect/tasks/{id}", Summary: "任务详情与文件列表", Tags: []string{"作业收集"}, Auth: true},
	{Method: "POST", Path: "/collect/tasks", Summary: "发起作业收集", Tags: []string{"作业收集"}, Auth: true},
	{Method: "POST", Path: "/collect/upload", Summary: "Agent 上传作业文件（免登录）", Tags: []string{"作业收集"}, Auth: false},
	{Method: "GET", Path: "/collect/download/{file_id}", Summary: "作业文件下载（免登录直链）", Tags: []string{"作业收集"}, Auth: false},

	// ===== 文件分发 =====
	{Method: "POST", Path: "/files/upload", Summary: "文件上传", Tags: []string{"文件"}, Auth: true},
	{Method: "GET", Path: "/files/{id}/download", Summary: "文件下载（免登录直链）", Tags: []string{"文件"}, Auth: false},
	{Method: "POST", Path: "/files/distribute", Summary: "下发文件到学生机", Tags: []string{"文件"}, Auth: true},
	{Method: "GET", Path: "/files/tasks", Summary: "分发任务列表", Tags: []string{"文件"}, Auth: true},
	{Method: "GET", Path: "/files/tasks/{id}/progress", Summary: "分发进度", Tags: []string{"文件"}, Auth: true},
	{Method: "POST", Path: "/files/chunk/init", Summary: "分片上传初始化", Tags: []string{"文件"}, Auth: true},
	{Method: "POST", Path: "/files/chunk/{file_id}/{chunk_index}", Summary: "上传分片", Tags: []string{"文件"}, Auth: true},
	{Method: "POST", Path: "/files/chunk/{file_id}/merge", Summary: "合并分片", Tags: []string{"文件"}, Auth: true},
	{Method: "GET", Path: "/files/chunk/{file_id}/progress", Summary: "分片上传进度", Tags: []string{"文件"}, Auth: true},
	{Method: "POST", Path: "/files/progress", Summary: "上报文件传输进度（免登录）", Tags: []string{"文件"}, Auth: false},

	// ===== 考试系统 =====
	{Method: "GET", Path: "/exams", Summary: "考试列表", Tags: []string{"考试"}, Auth: true},
	{Method: "GET", Path: "/exams/{id}", Summary: "考试详情", Tags: []string{"考试"}, Auth: true},
	{Method: "POST", Path: "/exams", Summary: "创建考试", Tags: []string{"考试"}, Auth: true},
	{Method: "DELETE", Path: "/exams/{id}", Summary: "删除考试", Tags: []string{"考试"}, Auth: true},
	{Method: "POST", Path: "/exams/{id}/start", Summary: "开始考试", Tags: []string{"考试"}, Auth: true},
	{Method: "POST", Path: "/exams/{id}/end", Summary: "结束考试", Tags: []string{"考试"}, Auth: true},
	{Method: "POST", Path: "/exams/{id}/extend", Summary: "延长考试", Tags: []string{"考试"}, Auth: true},
	{Method: "POST", Path: "/exams/{id}/regrade", Summary: "重新阅卷", Tags: []string{"考试"}, Auth: true},
	{Method: "GET", Path: "/exams/{id}/analysis", Summary: "成绩分析", Tags: []string{"考试"}, Auth: true},
	{Method: "GET", Path: "/exams/{id}/export", Summary: "导出成绩", Tags: []string{"考试"}, Auth: true},
	{Method: "GET", Path: "/exams/{id}/results", Summary: "考试结果列表", Tags: []string{"考试"}, Auth: true},
	{Method: "GET", Path: "/exams/{id}/timer", Summary: "考试计时状态", Tags: []string{"考试"}, Auth: true},
	{Method: "GET", Path: "/exams/{id}/violations", Summary: "违纪记录", Tags: []string{"考试"}, Auth: true},
	{Method: "GET", Path: "/papers", Summary: "试卷列表", Tags: []string{"考试"}, Auth: true},
	{Method: "GET", Path: "/papers/{id}", Summary: "试卷详情", Tags: []string{"考试"}, Auth: true},
	{Method: "POST", Path: "/papers", Summary: "创建试卷", Tags: []string{"考试"}, Auth: true},
	{Method: "PUT", Path: "/papers/{id}", Summary: "更新试卷", Tags: []string{"考试"}, Auth: true},
	{Method: "DELETE", Path: "/papers/{id}", Summary: "删除试卷", Tags: []string{"考试"}, Auth: true},
	{Method: "POST", Path: "/papers/random", Summary: "随机组卷", Tags: []string{"考试"}, Auth: true},
	{Method: "GET", Path: "/questions", Summary: "题库列表", Tags: []string{"考试"}, Auth: true},
	{Method: "GET", Path: "/questions/{id}", Summary: "题目详情", Tags: []string{"考试"}, Auth: true},
	{Method: "POST", Path: "/questions", Summary: "创建题目", Tags: []string{"考试"}, Auth: true},
	{Method: "PUT", Path: "/questions/{id}", Summary: "更新题目", Tags: []string{"考试"}, Auth: true},
	{Method: "DELETE", Path: "/questions/{id}", Summary: "删除题目", Tags: []string{"考试"}, Auth: true},
	{Method: "POST", Path: "/exam/submit", Summary: "学生交卷（免登录）", Tags: []string{"考试"}, Auth: false},
	{Method: "POST", Path: "/exam/violation", Summary: "违纪上报（免登录）", Tags: []string{"考试"}, Auth: false},
	{Method: "GET", Path: "/student/exam/{exam_id}/questions", Summary: "学生获取试题（免登录）", Tags: []string{"考试"}, Auth: false},
	{Method: "GET", Path: "/student/exam/{exam_id}/result", Summary: "学生查询成绩（免登录）", Tags: []string{"考试"}, Auth: false},
	{Method: "POST", Path: "/student/join", Summary: "学生加入考试（免登录）", Tags: []string{"考试"}, Auth: false},
	{Method: "POST", Path: "/student/submit", Summary: "学生提交（免登录）", Tags: []string{"考试"}, Auth: false},

	// ===== 考勤 =====
	{Method: "GET", Path: "/classroom/checkins", Summary: "课堂签到记录", Tags: []string{"考勤"}, Auth: true},
	{Method: "POST", Path: "/classroom/checkin", Summary: "课堂签到", Tags: []string{"考勤"}, Auth: true},
	{Method: "GET", Path: "/homework", Summary: "作业列表", Tags: []string{"考勤"}, Auth: true},
	{Method: "POST", Path: "/homework/submit", Summary: "提交作业", Tags: []string{"考勤"}, Auth: true},

	// ===== 门禁 =====
	{Method: "GET", Path: "/access/rules", Summary: "门禁规则列表", Tags: []string{"门禁"}, Auth: true},
	{Method: "POST", Path: "/access/rules", Summary: "创建门禁规则", Tags: []string{"门禁"}, Auth: true},
	{Method: "PUT", Path: "/access/bind", Summary: "绑定门禁", Tags: []string{"门禁"}, Auth: true},
	{Method: "DELETE", Path: "/access/rules/{id}", Summary: "删除门禁规则", Tags: []string{"门禁"}, Auth: true},
	{Method: "GET", Path: "/access/card-events", Summary: "刷卡事件", Tags: []string{"门禁"}, Auth: true},
	{Method: "POST", Path: "/access/card-event", Summary: "刷卡上报（免登录）", Tags: []string{"门禁"}, Auth: false},

	// ===== EduSync =====
	{Method: "GET", Path: "/edu/scores", Summary: "教学成绩", Tags: []string{"EduSync"}, Auth: true},
	{Method: "GET", Path: "/edu/scores/export", Summary: "导出教学成绩", Tags: []string{"EduSync"}, Auth: true},
	{Method: "GET", Path: "/edu/sync-configs", Summary: "同步配置列表", Tags: []string{"EduSync"}, Auth: true},
	{Method: "POST", Path: "/edu/sync-configs", Summary: "创建同步配置", Tags: []string{"EduSync"}, Auth: true},
	{Method: "GET", Path: "/edu/sync-logs", Summary: "同步日志", Tags: []string{"EduSync"}, Auth: true},
	{Method: "POST", Path: "/edu/push/classes", Summary: "推送班级（免登录）", Tags: []string{"EduSync"}, Auth: false},
	{Method: "POST", Path: "/edu/push/courses", Summary: "推送课程（免登录）", Tags: []string{"EduSync"}, Auth: false},
	{Method: "POST", Path: "/edu/push/schedules", Summary: "推送课表（免登录）", Tags: []string{"EduSync"}, Auth: false},
	{Method: "POST", Path: "/edu/push/students", Summary: "推送学生（免登录）", Tags: []string{"EduSync"}, Auth: false},

	// ===== 运维与系统 =====
	{Method: "GET", Path: "/audit/logs", Summary: "审计日志（data.list）", Tags: []string{"系统"}, Auth: true},
	{Method: "GET", Path: "/audit/logs/export", Summary: "导出审计日志", Tags: []string{"系统"}, Auth: true},
	{Method: "GET", Path: "/alerts", Summary: "告警列表", Tags: []string{"系统"}, Auth: true},
	{Method: "GET", Path: "/system/resources", Summary: "系统资源（CPU/内存/磁盘/DB）", Tags: []string{"系统"}, Auth: true},
	{Method: "GET", Path: "/system/alert-rules", Summary: "告警阈值", Tags: []string{"系统"}, Auth: true},
	{Method: "PUT", Path: "/system/alert-rules", Summary: "更新告警阈值（管理员）", Tags: []string{"系统"}, Auth: true},
	{Method: "GET", Path: "/system/cmd-whitelist", Summary: "exec 白名单", Tags: []string{"系统"}, Auth: true},
	{Method: "PUT", Path: "/system/cmd-whitelist", Summary: "更新 exec 白名单（管理员）", Tags: []string{"系统"}, Auth: true},
	{Method: "POST", Path: "/system/backup", Summary: "手动触发备份（管理员）", Tags: []string{"系统"}, Auth: true},
	{Method: "GET", Path: "/settings", Summary: "系统设置", Tags: []string{"系统"}, Auth: true, SummaryOnly: true},
	{Method: "PUT", Path: "/settings", Summary: "更新系统设置", Tags: []string{"系统"}, Auth: true},
	{Method: "POST", Path: "/settings/logo", Summary: "上传 Logo", Tags: []string{"系统"}, Auth: true},
	{Method: "GET", Path: "/images", Summary: "图片列表", Tags: []string{"系统"}, Auth: true},
	{Method: "GET", Path: "/license", Summary: "许可证信息", Tags: []string{"系统"}, Auth: true},
	{Method: "POST", Path: "/license", Summary: "导入许可证", Tags: []string{"系统"}, Auth: true},
	{Method: "POST", Path: "/license/issue", Summary: "签发许可证", Tags: []string{"系统"}, Auth: true},
	{Method: "GET", Path: "/ops/upgrade", Summary: "升级任务列表", Tags: []string{"系统"}, Auth: true},
	{Method: "GET", Path: "/ops/upgrade/{id}", Summary: "升级任务详情", Tags: []string{"系统"}, Auth: true},
	{Method: "POST", Path: "/ops/upgrade", Summary: "创建升级任务", Tags: []string{"系统"}, Auth: true},
	{Method: "POST", Path: "/ops/upgrade/{id}/start", Summary: "启动升级", Tags: []string{"系统"}, Auth: true},
	{Method: "POST", Path: "/ops/upgrade/{id}/rollback", Summary: "回滚升级", Tags: []string{"系统"}, Auth: true},
	{Method: "POST", Path: "/ops/upgrade/node/report", Summary: "升级节点上报（免登录）", Tags: []string{"系统"}, Auth: false},
	{Method: "POST", Path: "/ops/verify-password", Summary: "危险操作密码校验", Tags: []string{"系统"}, Auth: true},

	// ===== 看板与统计 =====
	{Method: "GET", Path: "/dashboard/global", Summary: "全局看板数据", Tags: []string{"看板"}, Auth: true},
	{Method: "GET", Path: "/stats/dashboard", Summary: "统计数据", Tags: []string{"看板"}, Auth: true},
	{Method: "GET", Path: "/broadcast/multiroom", Summary: "多机房广播列表", Tags: []string{"看板"}, Auth: true},
	{Method: "POST", Path: "/broadcast/multiroom/start", Summary: "发起多机房广播", Tags: []string{"看板"}, Auth: true},
	{Method: "POST", Path: "/broadcast/multiroom/{id}/stop", Summary: "停止多机房广播", Tags: []string{"看板"}, Auth: true},
}

// OpenAPISpec 生成完整 OpenAPI 3.0.3 文档
func OpenAPISpec() map[string]interface{} {
	paths := map[string]interface{}{}
	tagSet := map[string]bool{}

	for _, e := range endpointDefs {
		for _, t := range e.Tags {
			tagSet[t] = true
		}
		op := map[string]interface{}{
			"summary": e.Summary,
			"tags":    e.Tags,
			"responses": map[string]interface{}{
				"200": map[string]interface{}{
					"description": "统一响应 {code, msg, data}",
				},
			},
		}
		if e.Auth {
			op["security"] = bearerRef()
		}
		// 关键接口补充请求体示例
		if body := exampleRequestBody(e); body != nil {
			op["requestBody"] = map[string]interface{}{
				"required": true,
				"content": map[string]interface{}{
					"application/json": map[string]interface{}{"schema": body},
				},
			}
		}
		p := paths[e.Path]
		if p == nil {
			p = map[string]interface{}{}
		}
		pm := p.(map[string]interface{})
		pm[lower(e.Method)] = op
		paths[e.Path] = pm
	}

	tags := []map[string]interface{}{}
	for t := range tagSet {
		tags = append(tags, map[string]interface{}{"name": t})
	}

	return map[string]interface{}{
		"openapi": "3.0.3",
		"info": map[string]interface{}{
			"title":       "机房教学管理系统 API",
			"description": "对标 Veyon 的机房教学管理系统 REST API。统一响应结构：{code:0, msg:\"ok\", data:...}。认证方式：Authorization: Bearer <token>。WebSocket 端点：/ws/events、/ws/wall、/remote/ws、/agent/ws（token 走 query 参数）。",
			"version":     "3.0.0",
			"contact":     map[string]interface{}{"name": "北京东青互联科技"},
		},
		"servers": []map[string]interface{}{
			{"url": "/api/v1", "description": "API 前缀"},
		},
		"tags":  tags,
		"paths": paths,
		"components": map[string]interface{}{
			"securitySchemes": map[string]interface{}{
				"bearerAuth": map[string]interface{}{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "JWT",
				},
			},
		},
	}
}

// exampleRequestBody 为高频写接口提供示例 schema
func exampleRequestBody(e endpointDef) map[string]interface{} {
	switch e.Path + "|" + e.Method {
	case "/auth/login|POST":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"username":       map[string]interface{}{"type": "string", "example": "admin"},
				"password":       map[string]interface{}{"type": "string", "example": "ChangeMe@123"},
				"captcha_id":     map[string]interface{}{"type": "string"},
				"captcha_answer": map[string]interface{}{"type": "string"},
			},
			"required": []string{"username", "password"},
		}
	case "/ctrl/command|POST":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"agent_ids": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"action":    map[string]interface{}{"type": "string", "example": "lock|unlock|shutdown|reboot|snapshot|message|session_logoff|exec"},
				"params":    map[string]interface{}{"type": "object"},
				"password":  map[string]interface{}{"type": "string", "description": "危险操作（shutdown/reboot/exec/agent_upgrade）需提供管理员密码"},
			},
			"required": []string{"agent_ids", "action"},
		}
	case "/wol|POST":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"agent_ids": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"agent_id":  map[string]interface{}{"type": "string"},
				"mac":       map[string]interface{}{"type": "string", "example": "aa:bb:cc:dd:ee:ff"},
				"device_id": map[string]interface{}{"type": "integer"},
			},
		}
	case "/collect/tasks|POST":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"title":        map[string]interface{}{"type": "string", "example": "Python 作业"},
				"room_id":      map[string]interface{}{"type": "integer"},
				"path_pattern": map[string]interface{}{"type": "string", "example": "~/Desktop/*.docx"},
				"agent_ids":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			},
			"required": []string{"title", "room_id", "path_pattern"},
		}
	case "/wall/start|POST", "/wall/stop|POST":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"room_id": map[string]interface{}{"type": "integer"},
			},
			"required": []string{"room_id"},
		}
	case "/remote/start|POST":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"agent_id": map[string]interface{}{"type": "string"},
				"fps":      map[string]interface{}{"type": "integer"},
				"quality":  map[string]interface{}{"type": "integer"},
			},
			"required": []string{"agent_id"},
		}
	case "/msg/send|POST":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"agent_ids": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
				"text":      map[string]interface{}{"type": "string"},
				"confirm":   map[string]interface{}{"type": "boolean"},
				"duration":  map[string]interface{}{"type": "integer"},
			},
			"required": []string{"agent_ids", "text"},
		}
	case "/power-tasks|POST":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":       map[string]interface{}{"type": "string"},
				"action":     map[string]interface{}{"type": "string", "example": "shutdown|reboot|wol"},
				"execute_at": map[string]interface{}{"type": "string", "format": "date-time"},
				"room_id":    map[string]interface{}{"type": "integer"},
				"agent_ids":  map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
			},
			"required": []string{"name", "action", "execute_at"},
		}
	}
	return nil
}

func lower(s string) string {
	b := []byte(s)
	for i := range b {
		if b[i] >= 'A' && b[i] <= 'Z' {
			b[i] += 32
		}
	}
	return string(b)
}

// OpenAPIGeneratedAt 供文档页显示生成时间
var OpenAPIGeneratedAt = time.Now()
