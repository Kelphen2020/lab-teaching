package api

import (
	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/pkg/response"
	"strconv"

	"github.com/gin-gonic/gin"
)

// UploadAgentLog POST /api/v1/agent/logs（Agent 端上传日志）
func (h *Handler) UploadAgentLog(c *gin.Context) {
	var req struct {
		AgentID string `json:"agent_id"`
		Level   string `json:"level"`
		Content string `json:"content"`
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.AgentID == "" {
		response.BadRequest(c, "缺少 agent_id")
		return
	}
	if req.Level == "" { req.Level = "info" }
	log := model.AgentLog{AgentID: req.AgentID, Level: req.Level, Content: req.Content}
	if err := h.Repo.CreateAgentLog(&log); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Created(c, log)
}

// ListAgentLogs GET /api/v1/agent/logs?agent_id=xxx&limit=50
func (h *Handler) ListAgentLogs(c *gin.Context) {
	agentID := c.Query("agent_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if limit < 1 || limit > 500 { limit = 50 }
	list, err := h.Repo.ListAgentLogs(agentID, limit)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, list)
}