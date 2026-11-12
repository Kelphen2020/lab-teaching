package api

import (
	"os"
	"strconv"
	"strings"

	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
)

// ============ V1.0-13 无盘终端PXE+本地盘混合模式 ============
//
// 启动模式：
//   - 无盘: 终端从 PXE/iSCSI/NFS 网络引导，无本地系统
//   - 本地: 终端从本地硬盘引导（已装系统 + 本地 Agent）
//   - 混合: 优先本地盘引导（速度快、抗网络故障），本地盘异常时
//     Agent/iPXE 自动回退到 PXE 网络引导；管理端可一键切换整个机房
//
// 策略优先级: 座位级覆盖 (Seat.BootMode) > 机房级策略 (Room.BootMode)

// BootPolicy 终端启动策略（Agent 与 iPXE 引导脚本共用）
type BootPolicy struct {
	MAC        string `json:"mac"`
	RoomID     uint   `json:"room_id,omitempty"`
	RoomName   string `json:"room_name,omitempty"`
	SeatNo     string `json:"seat_no,omitempty"`
	BootMode   string `json:"boot_mode"`             // 无盘/本地/混合
	Effective  string `json:"effective"`             // 实际生效的引导方式: pxe / local
	ImageName  string `json:"image_name,omitempty"`  // 无盘镜像名
	FallbackOn string `json:"fallback_on,omitempty"` // 混合模式回退触发条件
	KernelArgs string `json:"kernel_args,omitempty"` // PXE 引导附加内核参数
}

// resolveBootPolicy 根据 MAC 解析启动策略（座位覆盖 > 机房默认）
func (h *Handler) resolveBootPolicy(mac string) BootPolicy {
	mac = strings.ToLower(strings.TrimSpace(mac))
	policy := BootPolicy{MAC: mac, BootMode: "混合", Effective: "pxe"}

	// 1. 按 MAC 查找座位级策略
	seat, err := h.Repo.GetSeatByMAC(mac)
	if err == nil && seat != nil {
		policy.SeatNo = seat.SeatNo
		policy.RoomID = seat.RoomID
		policy.BootMode = seat.BootMode
	}

	// 2. 机房级策略（座位未覆盖或座位为空时）
	if policy.RoomID > 0 {
		if room, err := h.Repo.GetRoom(policy.RoomID); err == nil {
			policy.RoomName = room.Name
			if policy.SeatNo == "" || seat == nil || seat.BootMode == "" {
				policy.BootMode = room.BootMode
			}
		}
	}

	// 3. 计算实际生效的引导方式
	switch policy.BootMode {
	case "无盘":
		policy.Effective = "pxe"
	case "本地":
		policy.Effective = "local"
	default: // 混合
		// 混合模式: iPXE 引导阶段仍从网络加载最小引导环境，
		// 由引导环境探测本地盘有效性后决定从本地盘还是网络根启动
		policy.Effective = "hybrid"
		policy.FallbackOn = "本地盘校验失败或3次启动超时"
		policy.KernelArgs = "lab_hybrid=1 lab_fallback=pxe"
	}
	if policy.BootMode == "无盘" || policy.BootMode == "" {
		policy.ImageName = "default"
	}
	return policy
}

// QueryBootPolicy 终端/Agent 查询启动策略（无需认证，PXE 引导链中使用）
// GET /api/v1/boot/policy?mac=AA:BB:CC:00:00:01
func (h *Handler) QueryBootPolicy(c *gin.Context) {
	mac := c.Query("mac")
	if mac == "" {
		response.BadRequest(c, "缺少 mac 参数")
		return
	}
	response.Success(c, h.resolveBootPolicy(mac))
}

// IPXEBootScript 动态生成 iPXE 引导脚本（支持混合模式回退）
// GET /api/v1/boot/ipxe?mac=AA:BB:CC:00:00:01
func (h *Handler) IPXEBootScript(c *gin.Context) {
	mac := c.Query("mac")
	if mac == "" {
		response.BadRequest(c, "缺少 mac 参数")
		return
	}
	p := h.resolveBootPolicy(mac)
	server := c.DefaultQuery("server", "")
	if server == "" {
		server = os.Getenv("LAB_BOOT_SERVER")
	}
	if server == "" {
		server = "127.0.0.1"
	}
	img := p.ImageName
	if img == "" {
		img = "default"
	}

	var sb strings.Builder
	sb.WriteString("#!ipxe\n")
	sb.WriteString("# 机房教学管理系统 - 动态引导脚本 (V1.0 混合模式)\n")
	sb.WriteString("# 终端 MAC: " + p.MAC + "\n")
	sb.WriteString("set server " + server + "\n")
	sb.WriteString("set mirror http://${server}/images\n\n")

	switch p.Effective {
	case "local":
		// 纯本地盘模式: iPXE 直接退出交还本地引导
		sb.WriteString("echo 策略=本地盘引导，退出网络引导\n")
		sb.WriteString("exit\n")
	case "hybrid":
		// 混合模式: 加载最小探测系统，检测本地盘后决定
		sb.WriteString("echo 策略=混合模式，探测本地盘...\n")
		sb.WriteString("kernel ${mirror}/" + img + "/vmlinuz ip=dhcp ro quiet " + p.KernelArgs + "\n")
		sb.WriteString("initrd ${mirror}/" + img + "/initrd.img\n")
		sb.WriteString("boot || goto netboot\n\n")
		sb.WriteString(":netboot\n")
		sb.WriteString("echo 混合探测失败，回退纯无盘模式\n")
		sb.WriteString("kernel ${mirror}/" + img + "/vmlinuz ip=dhcp ro quiet\n")
		sb.WriteString("initrd ${mirror}/" + img + "/initrd.img\n")
		sb.WriteString("boot\n")
	default: // pxe 纯无盘
		sb.WriteString("echo 策略=无盘模式\n")
		sb.WriteString("kernel ${mirror}/" + img + "/vmlinuz ip=dhcp ro quiet\n")
		sb.WriteString("initrd ${mirror}/" + img + "/initrd.img\n")
		sb.WriteString("boot\n")
	}

	c.Data(200, "text/plain; charset=utf-8", []byte(sb.String()))
}

// SetRoomBootMode 设置机房级启动策略
// PUT /api/v1/rooms/:id/boot-mode  {"boot_mode":"混合"}
func (h *Handler) SetRoomBootMode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的机房ID")
		return
	}
	var req struct {
		BootMode string `json:"boot_mode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if !isValidBootMode(req.BootMode) {
		response.BadRequest(c, "boot_mode 必须为 无盘/本地/混合")
		return
	}
	if err := h.Repo.UpdateRoomBootMode(uint(id), req.BootMode); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"room_id": id, "boot_mode": req.BootMode})
}

// SetSeatBootMode 设置座位级启动策略覆盖
// PUT /api/v1/seats/:id/boot-mode  {"boot_mode":"本地"}
func (h *Handler) SetSeatBootMode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的座位ID")
		return
	}
	var req struct {
		BootMode string `json:"boot_mode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if req.BootMode != "" && !isValidBootMode(req.BootMode) {
		response.BadRequest(c, "boot_mode 必须为 无盘/本地/混合 (空串=跟随机房)")
		return
	}
	if err := h.Repo.UpdateSeatBootMode(uint(id), req.BootMode); err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"seat_id": id, "boot_mode": req.BootMode})
}

// BatchSetBootMode 批量设置机房启动策略
// POST /api/v1/boot/batch  {"room_id":1, "seat_nos":["01","02"], "boot_mode":"本地"}
func (h *Handler) BatchSetBootMode(c *gin.Context) {
	var req struct {
		RoomID   uint     `json:"room_id" binding:"required"`
		SeatNos  []string `json:"seat_nos"`
		BootMode string   `json:"boot_mode" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if !isValidBootMode(req.BootMode) {
		response.BadRequest(c, "boot_mode 必须为 无盘/本地/混合")
		return
	}
	count, err := h.Repo.BatchUpdateSeatBootMode(req.RoomID, req.SeatNos, req.BootMode)
	if err != nil {
		response.ServerError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"updated": count, "boot_mode": req.BootMode})
}

func isValidBootMode(m string) bool {
	return m == "无盘" || m == "本地" || m == "混合"
}

// ReportBootResult 终端上报实际启动结果（混合模式探测脚本调用，无需认证）
// GET /api/v1/boot/report?mac=AA:BB:CC:00:00:01&mode=local&ver=1.0
func (h *Handler) ReportBootResult(c *gin.Context) {
	mac := c.Query("mac")
	if mac == "" {
		response.BadRequest(c, "缺少 mac 参数")
		return
	}
	mode := c.DefaultQuery("mode", "unknown")
	ver := c.Query("ver")
	// 记录到审计日志，便于管理端排查启动问题
	h.Repo.DB.Exec(
		"INSERT INTO t_audit (user_id, action, target, detail, ip, created_at) VALUES (0, 'boot_report', ?, ?, ?, NOW())",
		"terminal:"+mac, "mode="+mode+" ver="+ver, c.ClientIP(),
	)
	response.Success(c, gin.H{"mac": mac, "mode": mode, "version": ver, "received": true})
}

var _ = model.Room{} // 保持 model 导入
