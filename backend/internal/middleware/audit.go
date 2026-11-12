package middleware

import (
	"time"

	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/internal/repository"

	"github.com/gin-gonic/gin"
)

// AuditLog 操作审计中间件，记录 POST/PUT/DELETE 操作
func AuditLog(repo *repository.Repo) gin.HandlerFunc {
	return func(c *gin.Context) {
		method := c.Request.Method
		// 只记录写操作
		if method != "POST" && method != "PUT" && method != "DELETE" {
			c.Next()
			return
		}

		c.Next()

		// 只记录成功的写操作
		if c.Writer.Status() >= 400 {
			return
		}

		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")

		uid, _ := userID.(uint)
		uname, _ := username.(string)
		if uname == "" {
			uname = "system"
		}

		repo.CreateAudit(&model.Audit{
			UserID: uid,
			Action: method + " " + c.FullPath(),
			Target: c.Request.URL.Path,
			IP:     c.ClientIP(),
			Detail: "operator=" + uname,
		})
	}
}

// offlineCallback Agent 被心跳监控标记离线时的回调（d1: 浏览器事件推送）
var offlineCallback func(agentID string)

// SetOfflineCallback 注册离线事件回调（main 中注入，推送 agent_status 事件）
func SetOfflineCallback(fn func(agentID string)) { offlineCallback = fn }

// StartHeartbeatMonitor 后台心跳超时检测，每30秒扫描一次
// 超过60秒无心跳的在线Agent标记为离线并写入审计日志
func StartHeartbeatMonitor(repo *repository.Repo, stop <-chan struct{}) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			staleAgents := repo.FindStaleOnlineAgents(60 * time.Second)
			for _, agentID := range staleAgents {
				repo.SetAgentOnline(agentID, false)
				repo.CreateAudit(&model.Audit{
					UserID: 0,
					Action: "AGENT_OFFLINE",
					Target: agentID,
					IP:     "",
					Detail: "心跳超时自动标记离线",
				})
				if offlineCallback != nil {
					offlineCallback(agentID)
				}
			}
		}
	}
}
