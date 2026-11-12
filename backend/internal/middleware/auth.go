package middleware

import (
	"net/http"
	"strings"

	"lab-teaching/backend/pkg/auth"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// Auth JWT 鉴权中间件
func Auth() gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		if header == "" || !strings.HasPrefix(header, "Bearer ") {
			response.Unauthorized(c, "缺少或无效的 Authorization 头")
			c.Abort()
			return
		}
		tokenStr := strings.TrimPrefix(header, "Bearer ")
		claims, err := auth.ParseToken(tokenStr)
		if err != nil {
			response.Unauthorized(c, "token 无效或已过期")
			c.Abort()
			return
		}
		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Next()
	}
}

// RequireRoles 角色校验中间件
func RequireRoles(roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		roleStr, _ := role.(string)
		for _, r := range roles {
			if roleStr == r {
				c.Next()
				return
			}
		}
		response.Forbidden(c, "权限不足")
		c.Abort()
	}
}

// DenyViewerWrites 只读角色守卫：viewer 仅允许 GET/OPTIONS/HEAD，
// 其余写操作一律 403（登录、心跳等免鉴权路由不受影响）
func DenyViewerWrites() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, _ := c.Get("role")
		if role == "viewer" {
			m := c.Request.Method
			if m != http.MethodGet && m != http.MethodOptions && m != http.MethodHead {
				response.Forbidden(c, "只读角色无权执行此操作")
				c.Abort()
				return
			}
		}
		c.Next()
	}
}

// CORSMiddleware 跨域
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Authorization")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
