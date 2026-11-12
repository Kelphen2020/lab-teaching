package api

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

// ============ V1.x-21 系统设置 ============

// defaultSettings 系统设置默认值（未配置时返回）
var defaultSettings = map[string]string{
	"site_title": "机房教学管理系统",
	"site_logo":  "",
	"theme":      "tech-blue",
}

// GetSystemSettings GET /settings  公开返回站点配置（登录页也需要）
func (h *Handler) GetSystemSettings(c *gin.Context) {
	out := make(map[string]string)
	for k, v := range defaultSettings {
		out[k] = v
	}
	list, err := h.Repo.ListSettings()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	for _, s := range list {
		out[s.Key] = s.Value
	}
	response.Success(c, out)
}

// UpdateSystemSettings PUT /settings （管理员）
func (h *Handler) UpdateSystemSettings(c *gin.Context) {
	var req map[string]string
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	for k, v := range req {
		if len(k) > 64 {
			response.BadRequest(c, "配置项名称过长")
			return
		}
		if err := h.Repo.UpsertSetting(k, v); err != nil {
			response.ServerError(c, err.Error())
			return
		}
	}
	response.Success(c, req)
}

// UploadLogo POST /settings/logo （管理员）→ 保存 base64 图片并返回访问 URL
func (h *Handler) UploadLogo(c *gin.Context) {
	var req struct {
		Data string `json:"data"` // data:image/png;base64,xxxx 或纯 base64
	}
	if err := c.ShouldBindJSON(&req); err != nil || req.Data == "" {
		response.BadRequest(c, "缺少图片数据")
		return
	}
	data := req.Data
	ext := ".png"
	if strings.HasPrefix(data, "data:image/") {
		parts := strings.SplitN(data, ",", 2)
		if len(parts) != 2 {
			response.BadRequest(c, "dataURL 格式错误")
			return
		}
		mime := strings.TrimPrefix(strings.Split(parts[0], ";")[0], "data:image/")
		switch mime {
		case "jpeg", "jpg":
			ext = ".jpg"
		case "svg+xml":
			ext = ".svg"
		case "png", "":
			ext = ".png"
		default:
			response.BadRequest(c, "仅支持 png/jpg/svg 格式")
			return
		}
		data = parts[1]
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		response.BadRequest(c, "base64 解码失败")
		return
	}
	if len(raw) > 1<<20 { // 1MB 上限
		response.BadRequest(c, "Logo 文件过大(限1MB)")
		return
	}
	dir := "static/logo"
	if err := os.MkdirAll(dir, 0755); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	fname := fmt.Sprintf("logo_%d%s", time.Now().Unix(), ext)
	fpath := filepath.Join(dir, fname)
	if err := os.WriteFile(fpath, raw, 0644); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	url := "/api/v1/static/logo/" + fname
	_ = h.Repo.UpsertSetting("site_logo", url)
	response.Success(c, gin.H{"url": url})
}

// ServeStaticLogo GET /static/logo/:file （公开）
func (h *Handler) ServeStaticLogo(c *gin.Context) {
	fname := c.Param("file")
	// 防目录穿越：拒绝路径分隔符与 .. 序列（允许扩展名中的单个点）
	if strings.Contains(fname, "..") || strings.Contains(fname, "/") || strings.Contains(fname, "\\") {
		response.BadRequest(c, "非法文件名")
		return
	}
	fpath := filepath.Join("static/logo", fname)
	if _, err := os.Stat(fpath); err != nil {
		response.NotFound(c, "文件不存在")
		return
	}
	c.File(fpath)
}

// ============ V1.x-21 用户分组 ============

func (h *Handler) ListGroups(c *gin.Context) {
	groups, err := h.Repo.ListGroups()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, groups)
}

func (h *Handler) CreateGroup(c *gin.Context) {
	var g model.UserGroup
	if err := c.ShouldBindJSON(&g); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if g.Name == "" || g.Code == "" {
		response.BadRequest(c, "分组名称和编码必填")
		return
	}
	// 权限数组序列化
	if g.Permissions == "" {
		g.Permissions = "[]"
	} else if !json.Valid([]byte(g.Permissions)) {
		response.BadRequest(c, "权限配置格式错误")
		return
	}
	if err := h.Repo.CreateGroup(&g); err != nil {
		response.ServerError(c, "创建失败(编码可能重复)")
		return
	}
	response.Created(c, g)
}

func (h *Handler) UpdateGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效ID")
		return
	}
	g, err := h.Repo.GetGroup(uint(id))
	if err != nil {
		response.NotFound(c, "分组不存在")
		return
	}
	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		Permissions *string `json:"permissions"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Name != nil && *req.Name != "" {
		g.Name = *req.Name
	}
	if req.Description != nil {
		g.Description = *req.Description
	}
	if req.Permissions != nil {
		if !json.Valid([]byte(*req.Permissions)) {
			response.BadRequest(c, "权限配置格式错误")
			return
		}
		g.Permissions = *req.Permissions
	}
	if err := h.Repo.UpdateGroup(g); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, g)
}

func (h *Handler) DeleteGroup(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效ID")
		return
	}
	if err := h.Repo.DeleteGroup(uint(id)); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// ============ V1.x-21 用户管理 ============

type userResp struct {
	model.User
	GroupName string `json:"group_name"`
}

func (h *Handler) ListUsers(c *gin.Context) {
	users, err := h.Repo.ListUsers()
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	groups, _ := h.Repo.ListGroups()
	gname := map[uint]string{}
	for _, g := range groups {
		gname[g.ID] = g.Name
	}
	out := make([]userResp, 0, len(users))
	for _, u := range users {
		out = append(out, userResp{User: u, GroupName: gname[u.GroupID]})
	}
	response.Success(c, out)
}

type createUserReq struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	RealName string `json:"real_name"`
	Role     string `json:"role"`
	GroupID  uint   `json:"group_id"`
}

func (h *Handler) CreateUser(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if len(req.Password) < 8 {
		response.BadRequest(c, "密码至少8位")
		return
	}
	if req.Role == "" {
		req.Role = "teacher"
	}
	if req.Role != "admin" && req.Role != "teacher" && req.Role != "viewer" {
		response.BadRequest(c, "角色仅支持 admin/teacher/viewer")
		return
	}
	if _, err := h.Repo.FindUserByUsername(req.Username); err == nil {
		response.BadRequest(c, "用户名已存在")
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	u := &model.User{
		Username:     req.Username,
		PasswordHash: string(hash),
		RealName:     req.RealName,
		Role:         req.Role,
		Status:       "active",
		GroupID:      req.GroupID,
	}
	if err := h.Repo.CreateUser(u); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	u.PasswordHash = ""
	response.Created(c, u)
}

type updateUserReq struct {
	RealName *string `json:"real_name"`
	Role     *string `json:"role"`
	Status   *string `json:"status"`
	GroupID  *uint   `json:"group_id"`
	Password string  `json:"password"` // 可选：重置密码
}

func (h *Handler) UpdateUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效ID")
		return
	}
	u, err := h.Repo.GetUser(uint(id))
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	// 禁止删除最后一个管理员
	operator, _ := c.Get("user_id")
	if opID, ok := operator.(uint); ok && opID == u.ID && u.Role == "admin" {
		// 允许改自己，但下面有降级保护
	}
	var req updateUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.RealName != nil {
		u.RealName = *req.RealName
	}
	if req.Role != nil {
		if *req.Role != "admin" && *req.Role != "teacher" && *req.Role != "viewer" {
			response.BadRequest(c, "角色仅支持 admin/teacher")
			return
		}
		// 降级保护：检查是否还有其它管理员
		if u.Role == "admin" && *req.Role != "admin" {
			var n int64
			h.Repo.DB.Model(&model.User{}).Where("role = 'admin' AND id != ? AND status = 'active'", u.ID).Count(&n)
			if n == 0 {
				response.BadRequest(c, "不能降级最后一个管理员")
				return
			}
		}
		u.Role = *req.Role
	}
	if req.Status != nil {
		if *req.Status != "active" && *req.Status != "disabled" {
			response.BadRequest(c, "状态仅支持 active/disabled")
			return
		}
		u.Status = *req.Status
	}
	if req.GroupID != nil {
		u.GroupID = *req.GroupID
	}
	if req.Password != "" {
		if len(req.Password) < 8 {
			response.BadRequest(c, "密码至少8位")
			return
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			response.ServerError(c, err.Error())
			return
		}
		u.PasswordHash = string(hash)
	}
	if err := h.Repo.UpdateUser(u); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	u.PasswordHash = ""
	response.Success(c, u)
}

func (h *Handler) DeleteUser(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效ID")
		return
	}
	u, err := h.Repo.GetUser(uint(id))
	if err != nil {
		response.NotFound(c, "用户不存在")
		return
	}
	// 不能删除自己 / 最后一个管理员
	opID, _ := c.Get("user_id")
	if op, ok := opID.(uint); ok && op == u.ID {
		response.BadRequest(c, "不能删除当前登录用户")
		return
	}
	if u.Role == "admin" {
		var n int64
		h.Repo.DB.Model(&model.User{}).Where("role = 'admin' AND id != ? AND status = 'active'", u.ID).Count(&n)
		if n == 0 {
			response.BadRequest(c, "不能删除最后一个管理员")
			return
		}
	}
	if err := h.Repo.DeleteUser(uint(id)); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, nil)
}

// CheckDisabledUser 登录时校验 disabled 状态（由 Login 调用）
func (h *Handler) isUserDisabled(u *model.User) bool { return u.Status == "disabled" }

// permissionsForUser 计算用户权限列表：admin → ["*"]；viewer → 内置只读集；其他按所属分组 permissions JSON
func (h *Handler) permissionsForUser(u *model.User) []string {
	if u.Role == "admin" {
		return []string{"*"}
	}
	if u.Role == "viewer" {
		// 只读角色：仪表盘 + 监控告警 + 审计日志 + 终端在线（只看不改）
		return []string{"dashboard", "monitor", "audits", "agents"}
	}
	if u.GroupID == 0 {
		return []string{}
	}
	g, err := h.Repo.GetGroup(u.GroupID)
	if err != nil {
		return []string{}
	}
	var perms []string
	if err := json.Unmarshal([]byte(g.Permissions), &perms); err != nil {
		return []string{}
	}
	return perms
}

// 保留 http import 使用（ServeStaticLogo 通过 c.File 已用 net/http 间接）
var _ = http.StatusOK
