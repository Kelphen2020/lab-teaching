package api

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"time"

	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/pkg/license"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ============ V1.x-19 授权 License API ============

// licenseSecret 签发/校验密钥（生产环境务必通过 LICENSE_SECRET 环境变量注入）
func licenseSecret() string {
	if s := os.Getenv("LICENSE_SECRET"); s != "" {
		return s
	}
	return "dqinfos-lab-license-2026"
}

// licensePath license 文件位置
func licensePath() string {
	if p := os.Getenv("LICENSE_PATH"); p != "" {
		return p
	}
	return "data/license.json"
}

// GetLicenseStatus GET /api/v1/license（JWT）
func (h *Handler) GetLicenseStatus(c *gin.Context) {
	lic, err := license.Load(licensePath(), licenseSecret())
	if err != nil && lic == nil {
		msg := "无授权文件（演示模式）"
		if err == license.ErrExpired {
			msg = "授权已过期"
		} else if err == license.ErrInvalid {
			msg = "授权文件无效"
		}
		response.Success(c, gin.H{
			"licensed": false, "status": msg,
			"device_count": countSeats(h),
		})
		return
	}
	var status string
	if err == license.ErrExpired {
		status = lic.Status()
	} else {
		status = lic.Status()
	}
	response.Success(c, gin.H{
		"licensed":     true,
		"customer":     lic.Customer,
		"expires_at":   lic.ExpiresAt,
		"days_left":    lic.DaysLeft(),
		"max_devices":  lic.MaxDevices,
		"modules":      lic.Modules,
		"status":       status,
		"device_count": countSeats(h),
	})
}

// countSeats 统计当前终端（座位）数
func countSeats(h *Handler) int64 {
	var cnt int64
	h.Repo.DB.Model(&model.Seat{}).Count(&cnt)
	return cnt
}

// UploadLicense POST /api/v1/license（JWT）：上传 license 文件内容
func (h *Handler) UploadLicense(c *gin.Context) {
	var req struct {
		License json.RawMessage `json:"license" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: license 字段必填")
		return
	}
	var lic license.License
	if e := json.Unmarshal(req.License, &lic); e != nil {
		response.BadRequest(c, "license JSON 解析失败")
		return
	}
	if e := license.Verify(&lic, licenseSecret()); e != nil {
		response.BadRequest(c, "license 签名校验失败，请使用授权方签发的文件")
		return
	}
	if e := license.Save(licensePath(), &lic); e != nil {
		response.ServerError(c, e.Error())
		return
	}
	response.Success(c, gin.H{
		"saved": licensePath(), "customer": lic.Customer,
		"expires_at": lic.ExpiresAt, "days_left": lic.DaysLeft(),
	})
}

// IssueLicense POST /api/v1/license/issue（JWT）：签发 license（授权方使用）
func (h *Handler) IssueLicense(c *gin.Context) {
	var req struct {
		Customer   string   `json:"customer" binding:"required"`
		MaxDevices int      `json:"max_devices"`
		ExpiresAt  string   `json:"expires_at"` // RFC3339，空=1年后
		Modules    []string `json:"modules"`
		Campus     int      `json:"campus"`
		Remark     string   `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "customer 必填")
		return
	}
	if req.ExpiresAt == "" {
		req.ExpiresAt = time.Now().AddDate(1, 0, 0).Format(time.RFC3339)
	}
	if _, e := time.Parse(time.RFC3339, req.ExpiresAt); e != nil {
		response.BadRequest(c, "expires_at 需为 RFC3339 格式")
		return
	}
	if req.MaxDevices <= 0 {
		req.MaxDevices = 200
	}
	if len(req.Modules) == 0 {
		req.Modules = []string{"teaching", "exam", "access", "campus", "ops"}
	}
	lic := license.License{
		Customer:   req.Customer,
		IssuedAt:   time.Now().Format(time.RFC3339),
		ExpiresAt:  req.ExpiresAt,
		MaxDevices: req.MaxDevices,
		Modules:    req.Modules,
		Campus:     req.Campus,
		Remark:     req.Remark,
	}
	if e := license.Sign(&lic, licenseSecret()); e != nil {
		response.ServerError(c, e.Error())
		return
	}
	response.Success(c, lic)
}

// CheckLicenseCapacity 终端容量校验（新增座位前调用）
// 返回 nil=允许；超出授权容量返回错误
func CheckLicenseCapacity(h *Handler) error {
	lic, err := license.Load(licensePath(), licenseSecret())
	if err != nil || lic == nil {
		return nil // 演示模式不限（或按部署策略限制）
	}
	if countSeats(h) >= int64(lic.MaxDevices) {
		return errors.New("终端数量已达授权上限（" + strconv.Itoa(lic.MaxDevices) + "），请升级授权")
	}
	return nil
}
