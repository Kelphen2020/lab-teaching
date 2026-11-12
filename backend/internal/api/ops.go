package api

import (
	"strconv"
	"strings"
	"time"

	"lab-teaching/backend/internal/middleware"
	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/internal/repository"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ============ V1.x-20 运维工具：Agent 批量升级 ============
// 模式：主中心下发升级任务 -> 各机房 Agent 拉取新版本 -> 灰度滚动 -> 失败回滚。
// 本实现负责任务编排与状态管理；实际二进制下发通过 WS 通道推送版本号触发。

// CreateUpgradeTask POST /api/v1/ops/upgrade（JWT）
func (h *Handler) CreateUpgradeTask(c *gin.Context) {
	var req struct {
		Name    string `json:"name" binding:"required"`
		Version string `json:"version" binding:"required"`
		Arch    string `json:"arch"`
		RoomIDs string `json:"room_ids"` // 逗号分隔；空=全部机房
		GrayPct int    `json:"gray_pct"` // 0-100
		Remark  string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name 与 version 必填")
		return
	}
	if req.GrayPct < 0 || req.GrayPct > 100 {
		response.BadRequest(c, "gray_pct 需在 0-100")
		return
	}
	arch := req.Arch
	if arch == "" {
		arch = "amd64"
	}
	// 目标终端：按机房筛选座位
	q := h.Repo.DB.Model(&model.Seat{})
	if req.RoomIDs != "" {
		ids := strings.Split(req.RoomIDs, ",")
		q = q.Where("room_id IN ?", ids)
	}
	var total int64
	q.Count(&total)
	if total == 0 {
		response.BadRequest(c, "目标范围内无终端")
		return
	}
	uid, _ := c.Get("uid")
	var uidNum uint
	if v, ok := uid.(uint); ok {
		uidNum = v
	}
	task := model.UpgradeTask{
		Name: req.Name, Version: req.Version, Arch: arch,
		RoomIDs: req.RoomIDs, GrayPct: req.GrayPct,
		Status: "pending", Total: int(total), Remark: req.Remark,
		CreatedBy: uidNum, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if e := h.Repo.DB.Create(&task).Error; e != nil {
		response.ServerError(c, e.Error())
		return
	}
	response.Success(c, gin.H{"task": task, "target_count": total})
}

// ListUpgradeTasks GET /api/v1/ops/upgrade（JWT）
func (h *Handler) ListUpgradeTasks(c *gin.Context) {
	var tasks []model.UpgradeTask
	h.Repo.DB.Order("id desc").Limit(100).Find(&tasks)
	response.Success(c, tasks)
}

// GetUpgradeTask GET /api/v1/ops/upgrade/:id（JWT）含节点明细
func (h *Handler) GetUpgradeTask(c *gin.Context) {
	id := c.Param("id")
	var task model.UpgradeTask
	if e := h.Repo.DB.First(&task, "id = ?", id).Error; e != nil {
		response.NotFound(c, "任务不存在")
		return
	}
	var nodes []model.UpgradeNode
	h.Repo.DB.Where("task_id = ?", task.ID).Find(&nodes)
	response.Success(c, gin.H{"task": task, "nodes": nodes})
}

// StartUpgradeTask POST /api/v1/ops/upgrade/:id/start（JWT）
// 生成节点升级记录，按灰度比例标记首批
func (h *Handler) StartUpgradeTask(c *gin.Context) {
	id := c.Param("id")
	var task model.UpgradeTask
	if e := h.Repo.DB.First(&task, "id = ?", id).Error; e != nil {
		response.NotFound(c, "任务不存在")
		return
	}
	if task.Status != "pending" {
		response.BadRequest(c, "仅 pending 状态可启动")
		return
	}
	// 重建节点记录
	h.Repo.DB.Where("task_id = ?", task.ID).Delete(&model.UpgradeNode{})
	q := h.Repo.DB.Model(&model.Seat{})
	if task.RoomIDs != "" {
		q = q.Where("room_id IN ?", strings.Split(task.RoomIDs, ","))
	}
	var seats []model.Seat
	q.Find(&seats)
	batch := make([]model.UpgradeNode, 0, len(seats))
	for _, s := range seats {
		batch = append(batch, model.UpgradeNode{
			TaskID: task.ID, SeatID: s.ID,
			AgentID: strconv.FormatUint(uint64(s.ID), 10),
			Status:  "pending", UpdatedAt: time.Now(),
		})
	}
	if len(batch) > 0 {
		h.Repo.DB.Create(&batch)
	}
	// 灰度：首批按 gray_pct 置 upgrading，其余保持 pending
	if task.GrayPct > 0 && task.GrayPct < 100 {
		n := len(batch) * task.GrayPct / 100
		if n < 1 {
			n = 1
		}
		h.Repo.DB.Model(&model.UpgradeNode{}).Where("task_id = ? AND id IN ?", task.ID, nodeIDs(batch[:n])).Update("status", "upgrading")
	} else {
		h.Repo.DB.Model(&model.UpgradeNode{}).Where("task_id = ?", task.ID).Update("status", "upgrading")
	}
	task.Status = "rolling"
	task.UpdatedAt = time.Now()
	h.Repo.DB.Save(&task)
	response.Success(c, gin.H{"task_id": task.ID, "status": "rolling", "nodes": len(batch)})
}

func nodeIDs(ns []model.UpgradeNode) []uint {
	ids := make([]uint, 0, len(ns))
	for _, n := range ns {
		ids = append(ids, n.ID)
	}
	return ids
}

// ReportUpgradeNode POST /api/v1/ops/upgrade/node/report（JWT/Agent）
// 节点上报升级结果：success/failed，携带新版本号或错误
func (h *Handler) ReportUpgradeNode(c *gin.Context) {
	var req struct {
		TaskID uint   `json:"task_id" binding:"required"`
		SeatID uint   `json:"seat_id" binding:"required"`
		Status string `json:"status" binding:"required"` // success/failed
		NewVer string `json:"new_ver"`
		Error  string `json:"error"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "task_id/seat_id/status 必填")
		return
	}
	var node model.UpgradeNode
	if e := h.Repo.DB.Where("task_id = ? AND seat_id = ?", req.TaskID, req.SeatID).First(&node).Error; e != nil {
		response.NotFound(c, "节点记录不存在")
		return
	}
	node.Status = req.Status
	node.NewVer = req.NewVer
	node.Error = req.Error
	node.UpdatedAt = time.Now()
	h.Repo.DB.Save(&node)
	// 聚合任务进度
	refreshUpgradeTaskProgress(h, req.TaskID)
	response.Success(c, gin.H{"reported": true})
}

func refreshUpgradeTaskProgress(h *Handler, taskID uint) {
	var task model.UpgradeTask
	if e := h.Repo.DB.First(&task, "id = ?", taskID).Error; e != nil {
		return
	}
	var succ, fail int64
	h.Repo.DB.Model(&model.UpgradeNode{}).Where("task_id = ? AND status = ?", taskID, "success").Count(&succ)
	h.Repo.DB.Model(&model.UpgradeNode{}).Where("task_id = ? AND status = ?", taskID, "failed").Count(&fail)
	task.Succeeded = int(succ)
	task.Failed = int(fail)
	if succ+fail >= int64(task.Total) {
		if fail == 0 {
			task.Status = "success"
		} else {
			task.Status = "failed"
		}
	}
	task.UpdatedAt = time.Now()
	h.Repo.DB.Save(&task)
}

// RollbackUpgradeTask POST /api/v1/ops/upgrade/:id/rollback（JWT）
func (h *Handler) RollbackUpgradeTask(c *gin.Context) {
	id := c.Param("id")
	var task model.UpgradeTask
	if e := h.Repo.DB.First(&task, "id = ?", id).Error; e != nil {
		response.NotFound(c, "任务不存在")
		return
	}
	if task.Status == "rolled_back" {
		response.BadRequest(c, "已回滚")
		return
	}
	// 将已升级/升级中的节点标记回滚
	h.Repo.DB.Model(&model.UpgradeNode{}).
		Where("task_id = ? AND status IN ?", task.ID, []string{"upgrading", "success"}).
		Update("status", "rolled_back")
	task.Status = "rolled_back"
	task.UpdatedAt = time.Now()
	h.Repo.DB.Save(&task)
	response.Success(c, gin.H{"task_id": task.ID, "status": "rolled_back"})
}

// ============ V1.x-22 监控告警：聚合健康 + 告警规则 ============

// alertItem 告警条目
type alertItem struct {
	Level   string `json:"level"` // warning/critical
	Rule    string `json:"rule"`
	Message string `json:"message"`
	Value   string `json:"value"`
}

func settingFloat(repo *repository.Repo, key string, def float64) float64 {
	v, ok := repo.GetSetting(key)
	if !ok {
		return def
	}
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return def
	}
	return f
}

// GetAlertRules 返回当前告警阈值设置
func (h *Handler) GetAlertRules(c *gin.Context) {
	response.Success(c, gin.H{
		"agent_online_warn":     settingFloat(h.Repo, "alert_agent_online_warn", 80),
		"agent_online_critical": settingFloat(h.Repo, "alert_agent_online_critical", 50),
		"http_error_warn":       settingFloat(h.Repo, "alert_http_error_warn", 5),
		"http_error_critical":   settingFloat(h.Repo, "alert_http_error_critical", 20),
	})
}

// UpdateAlertRules 更新告警阈值
func (h *Handler) UpdateAlertRules(c *gin.Context) {
	var req map[string]float64
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	keys := map[string]string{
		"agent_online_warn":     "alert_agent_online_warn",
		"agent_online_critical": "alert_agent_online_critical",
		"http_error_warn":       "alert_http_error_warn",
		"http_error_critical":   "alert_http_error_critical",
	}
	for k, dbKey := range keys {
		if v, ok := req[k]; ok {
			h.Repo.UpsertSetting(dbKey, strconv.FormatFloat(v, 'f', 1, 64))
		}
	}
	response.Success(c, nil)
}

// Alerts GET /api/v1/alerts（JWT）
// 规则：Agent 离线告警 / 升级任务失败告警 / HTTP 错误率告警。
func (h *Handler) Alerts(c *gin.Context) {
	response.Success(c, h.AlertsPayload())
}

// AlertsPayload 聚合告警+指标快照，供 /alerts 接口与 WebSocket 推送复用
func (h *Handler) AlertsPayload() gin.H {
	alerts := []alertItem{}

	// 读取告警阈值（可配置）
	agentWarn := settingFloat(h.Repo, "alert_agent_online_warn", 80)
	agentCrit := settingFloat(h.Repo, "alert_agent_online_critical", 50)
	httpWarn := settingFloat(h.Repo, "alert_http_error_warn", 5)
	httpCrit := settingFloat(h.Repo, "alert_http_error_critical", 20)

	// 1) Agent 离线告警
	var totalAgents int64
	h.Repo.DB.Model(&model.AgentSession{}).Count(&totalAgents)
	online := h.Repo.CountOnlineAgents()
	if totalAgents > 0 {
		rate := float64(online) / float64(totalAgents) * 100
		msg := "终端在线率 " + strconv.FormatFloat(rate, 'f', 1, 64) + "%（" + strconv.FormatInt(online, 10) + "/" + strconv.FormatInt(totalAgents, 10) + "）"
		if rate < agentCrit {
			alerts = append(alerts, alertItem{"critical", "agent_offline", msg, strconv.FormatFloat(rate, 'f', 1, 64) + "%"})
		} else if rate < agentWarn {
			alerts = append(alerts, alertItem{"warning", "agent_offline", msg, strconv.FormatFloat(rate, 'f', 1, 64) + "%"})
		}
	}

	// 2) 升级失败告警：最近一次非 pending 升级任务的节点失败数
	var lastTask model.UpgradeTask
	if err := h.Repo.DB.Order("id desc").First(&lastTask).Error; err == nil && lastTask.Status != "pending" {
		if lastTask.Status == "failed" || lastTask.Failed > 0 {
			lvl := "warning"
			if lastTask.Status == "failed" || (lastTask.Total > 0 && float64(lastTask.Failed)/float64(lastTask.Total) >= 0.5) {
				lvl = "critical"
			}
			alerts = append(alerts, alertItem{lvl, "upgrade_failed",
				"升级任务「" + lastTask.Name + "」状态 " + lastTask.Status + "，失败节点 " + strconv.Itoa(lastTask.Failed) + "/" + strconv.Itoa(lastTask.Total),
				lastTask.Status})
		}
	}

	// 3) HTTP 错误率告警（进程内指标，样本量 >100 才评估）
	reqs, errs, avgLat, uptime := middleware.MetricsSnapshot()
	if reqs > 100 {
		errRate := float64(errs) / float64(reqs) * 100
		if errRate >= httpWarn {
			lvl := "warning"
			if errRate >= httpCrit {
				lvl = "critical"
			}
			alerts = append(alerts, alertItem{lvl, "http_error_rate",
				"HTTP 错误率 " + strconv.FormatFloat(errRate, 'f', 2, 64) + "%（" + strconv.FormatInt(errs, 10) + "/" + strconv.FormatInt(reqs, 10) + "）",
				strconv.FormatFloat(errRate, 'f', 2, 64) + "%"})
		}
	}

	// 汇总严重级别：任一 critical 即 critical，其次 warning，否则 ok
	severity := "ok"
	for _, a := range alerts {
		if a.Level == "critical" {
			severity = "critical"
			break
		}
		if a.Level == "warning" {
			severity = "warning"
		}
	}

	return gin.H{
		"alerts":   alerts,
		"severity": severity,
		"metrics": gin.H{
			"uptime_seconds":      int(uptime.Seconds()),
			"http_requests_total": reqs,
			"http_errors_total":   errs,
			"avg_latency_micros":  avgLat,
			"agents_online":       online,
			"agents_total":        totalAgents,
		},
	}
}
