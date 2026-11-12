package api

import (
	"fmt"
	"math/rand"
	"sync"
	"time"

	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

type captchaEntry struct {
	Answer   string
	ExpireAt time.Time
}

var (
	captchaStore = map[string]captchaEntry{}
	captchaMu    sync.Mutex
)

func init() {
	go func() {
		for range time.Tick(5 * time.Minute) {
			captchaMu.Lock()
			now := time.Now()
			for k, v := range captchaStore {
				if now.After(v.ExpireAt) {
					delete(captchaStore, k)
				}
			}
			captchaMu.Unlock()
		}
	}()
}

// GetCaptcha 生成数学验证码
func (h *Handler) GetCaptcha(c *gin.Context) {
	a := rand.Intn(20) + 1
	b := rand.Intn(20) + 1
	op := "+"
	result := fmt.Sprintf("%d", a+b)
	if rand.Intn(2) == 0 {
		op = "-"
		if a < b { a, b = b, a }
		result = fmt.Sprintf("%d", a-b)
	}
	id := fmt.Sprintf("%d%d", time.Now().UnixNano(), rand.Intn(9999))
	captchaMu.Lock()
	captchaStore[id] = captchaEntry{Answer: result, ExpireAt: time.Now().Add(5 * time.Minute)}
	captchaMu.Unlock()
	response.Success(c, gin.H{
		"id":       id,
		"question": fmt.Sprintf("%d %s %d = ?", a, op, b),
	})
}

// VerifyCaptcha 验证验证码
func verifyCaptcha(id, answer string) bool {
	captchaMu.Lock()
	defer captchaMu.Unlock()
	e, ok := captchaStore[id]
	if !ok {
		return false
	}
	if time.Now().After(e.ExpireAt) {
		delete(captchaStore, id)
		return false
	}
	delete(captchaStore, id) // 一次性
	return e.Answer == answer
}