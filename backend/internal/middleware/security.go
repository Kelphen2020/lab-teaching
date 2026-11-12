package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// === 登录失败锁定 ===

type loginAttempt struct {
	Count    int
	LockedAt time.Time
}

var (
	loginAttempts = make(map[string]*loginAttempt)
	loginMu       sync.Mutex
	maxFail       = 5
	lockDuration  = 15 * time.Minute
)

// SetLoginPolicy 配置登录锁定策略
func SetLoginPolicy(maxFails int, lockMin int) {
	loginMu.Lock()
	defer loginMu.Unlock()
	if maxFails > 0 {
		maxFail = maxFails
	}
	if lockMin > 0 {
		lockDuration = time.Duration(lockMin) * time.Minute
	}
}

// CheckLoginLock 检查IP是否被锁定
func CheckLoginLock(ip string) bool {
	loginMu.Lock()
	defer loginMu.Unlock()
	a, ok := loginAttempts[ip]
	if !ok {
		return false
	}
	if a.LockedAt.IsZero() {
		return false
	}
	if time.Since(a.LockedAt) > lockDuration {
		// 锁定已过期，重置
		delete(loginAttempts, ip)
		return false
	}
	return true
}

// RecordLoginFail 记录登录失败
func RecordLoginFail(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	a, ok := loginAttempts[ip]
	if !ok {
		a = &loginAttempt{}
		loginAttempts[ip] = a
	}
	a.Count++
	if a.Count >= maxFail {
		a.LockedAt = time.Now()
	}
}

// RecordLoginSuccess 登录成功重置计数
func RecordLoginSuccess(ip string) {
	loginMu.Lock()
	defer loginMu.Unlock()
	delete(loginAttempts, ip)
}

// GetLoginFails 返回当前失败次数
func GetLoginFails(ip string) int {
	loginMu.Lock()
	defer loginMu.Unlock()
	if a, ok := loginAttempts[ip]; ok {
		return a.Count
	}
	return 0
}

// MaxLoginFails 返回最大失败次数
func MaxLoginFails() int { return maxFail }

// === API 限流（令牌桶，per-IP） ===

type bucket struct {
	tokens   float64
	lastTime time.Time
}

var (
	buckets  = make(map[string]*bucket)
	bucketMu sync.Mutex
	rateRPS  = 100.0
)

// SetRateLimit 配置限流参数
func SetRateLimit(rps int) {
	bucketMu.Lock()
	defer bucketMu.Unlock()
	if rps > 0 {
		rateRPS = float64(rps)
	}
}

// RateLimiter 限流中间件
func RateLimiter() gin.HandlerFunc {
	// 定期清理过期桶
	go func() {
		for range time.Tick(5 * time.Minute) {
			bucketMu.Lock()
			now := time.Now()
			for ip, b := range buckets {
				if now.Sub(b.lastTime) > 10*time.Minute {
					delete(buckets, ip)
				}
			}
			bucketMu.Unlock()
		}
	}()

	return func(c *gin.Context) {
		ip := c.ClientIP()
		bucketMu.Lock()
		b, ok := buckets[ip]
		now := time.Now()
		if !ok {
			b = &bucket{tokens: rateRPS, lastTime: now}
			buckets[ip] = b
		}
		// 补充令牌
		elapsed := now.Sub(b.lastTime).Seconds()
		b.tokens += elapsed * rateRPS
		if b.tokens > rateRPS {
			b.tokens = rateRPS
		}
		b.lastTime = now

		if b.tokens < 1 {
			bucketMu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{
				"code": 429,
				"msg":  "请求过于频繁，请稍后重试",
			})
			c.Abort()
			return
		}
		b.tokens--
		bucketMu.Unlock()
		c.Next()
	}
}
