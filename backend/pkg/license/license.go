package license

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ============ V1.x-19 授权 License ============
// 授权文件格式：JSON 载荷 + HMAC-SHA256 签名（密钥=安装密钥），
// 字段：客户名称、最大终端数、到期时间、启用模块列表、签发时间。
// 校验失败或过期时系统降级为只读演示模式。

// License 授权信息
type License struct {
	Customer   string   `json:"customer"`              // 客户名称
	IssuedAt   string   `json:"issued_at"`             // 签发时间 RFC3339
	ExpiresAt  string   `json:"expires_at"`            // 到期时间 RFC3339
	MaxDevices int      `json:"max_devices"`           // 最大终端数
	Modules    []string `json:"modules"`               // 启用模块：teaching/exam/access/campus 等
	Campus     int      `json:"campus,omitempty"`      // 校区数上限（0=不限）
	Remark     string   `json:"remark,omitempty"`
	Signature  string   `json:"signature"`             // HMAC 签名（对规范化 JSON）
}

var (
	ErrNotFound = errors.New("license 文件不存在")
	ErrInvalid  = errors.New("license 签名无效或格式错误")
	ErrExpired  = errors.New("license 已过期")
)

// payload 生成规范化签名载荷（排除 signature 字段）
func payload(lic *License) ([]byte, error) {
	tmp := *lic
	tmp.Signature = ""
	// 固定字段顺序
	m := map[string]interface{}{
		"customer":    tmp.Customer,
		"issued_at":   tmp.IssuedAt,
		"expires_at":  tmp.ExpiresAt,
		"max_devices": tmp.MaxDevices,
		"modules":     tmp.Modules,
		"campus":      tmp.Campus,
		"remark":      tmp.Remark,
	}
	return json.Marshal(m)
}

// Sign 用密钥签名 license
func Sign(lic *License, secret string) error {
	data, err := payload(lic)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(data)
	lic.Signature = hex.EncodeToString(mac.Sum(nil))
	return nil
}

// Verify 校验签名
func Verify(lic *License, secret string) error {
	data, err := payload(lic)
	if err != nil {
		return ErrInvalid
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(data)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(strings.ToLower(lic.Signature))) {
		return ErrInvalid
	}
	return nil
}

// Load 从文件加载并校验（secret 为空时用环境变量 LICENSE_SECRET）
func Load(path string, secret string) (*License, error) {
	if secret == "" {
		secret = os.Getenv("LICENSE_SECRET")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, ErrNotFound
	}
	var lic License
	if e := json.Unmarshal(raw, &lic); e != nil {
		return nil, ErrInvalid
	}
	if secret != "" {
		if e := Verify(&lic, secret); e != nil {
			return nil, e
		}
	}
	exp, e := time.Parse(time.RFC3339, lic.ExpiresAt)
	if e != nil {
		return nil, ErrInvalid
	}
	if time.Now().After(exp) {
		return &lic, ErrExpired
	}
	return &lic, nil
}

// Save 写入 license 文件
func Save(path string, lic *License) error {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if e := os.MkdirAll(dir, 0o755); e != nil {
			return e
		}
	}
	data, err := json.MarshalIndent(lic, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// HasModule 检查模块是否启用
func (l *License) HasModule(name string) bool {
	if l == nil {
		return false
	}
	for _, m := range l.Modules {
		if strings.EqualFold(m, name) {
			return true
		}
	}
	return false
}

// DaysLeft 剩余天数（负数=已过期）
func (l *License) DaysLeft() int {
	exp, e := time.Parse(time.RFC3339, l.ExpiresAt)
	if e != nil {
		return 0
	}
	return int(time.Until(exp).Hours() / 24)
}

// Status 汇总状态
func (l *License) Status() string {
	if l == nil {
		return "无授权（演示模式）"
	}
	if l.DaysLeft() < 0 {
		return fmt.Sprintf("已过期 %d 天", -l.DaysLeft())
	}
	if l.DaysLeft() <= 30 {
		return fmt.Sprintf("即将到期（剩余 %d 天）", l.DaysLeft())
	}
	return "有效"
}
