package api

import (
	"strconv"

	"lab-teaching/backend/internal/model"
	"lab-teaching/backend/pkg/response"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// ============ V1.x-18 多校区管理 ============
// 模式：主中心 + 分校区。每个校区独立部署 gateway-api+PostgreSQL（本地自治，断网可用），
// 通过校区编码(code)做数据汇总上报；中心端提供统一看板与跨校区查询。

// ---- 校区 CRUD ----

// CreateCampus POST /api/v1/campuses（JWT）
func (h *Handler) CreateCampus(c *gin.Context) {
	var campus model.Campus
	if err := c.ShouldBindJSON(&campus); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if campus.Code == "" || campus.Name == "" {
		response.BadRequest(c, "code 与 name 必填")
		return
	}
	if e := h.Repo.DB.Create(&campus).Error; e != nil {
		response.ServerError(c, e.Error())
		return
	}
	response.Created(c, campus)
}

// ListCampuses GET /api/v1/campuses（JWT）
func (h *Handler) ListCampuses(c *gin.Context) {
	var campuses []model.Campus
	h.Repo.DB.Order("id").Find(&campuses)
	response.Success(c, campuses)
}

// UpdateCampus PUT /api/v1/campuses/:id（JWT）
func (h *Handler) UpdateCampus(c *gin.Context) {
	id := c.Param("id")
	var campus model.Campus
	if e := h.Repo.DB.First(&campus, id).Error; e != nil {
		if e == gorm.ErrRecordNotFound {
			response.NotFound(c, "校区不存在")
			return
		}
		response.ServerError(c, e.Error())
		return
	}
	var req model.Campus
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}
	if req.Name != "" {
		campus.Name = req.Name
	}
	if req.Address != "" {
		campus.Address = req.Address
	}
	if req.Contact != "" {
		campus.Contact = req.Contact
	}
	if req.Phone != "" {
		campus.Phone = req.Phone
	}
	if req.Status != "" {
		campus.Status = req.Status
	}
	if e := h.Repo.DB.Save(&campus).Error; e != nil {
		response.ServerError(c, e.Error())
		return
	}
	response.Success(c, campus)
}

// DeleteCampus DELETE /api/v1/campuses/:id（JWT）
func (h *Handler) DeleteCampus(c *gin.Context) {
	id := c.Param("id")
	// 有归属机房时禁止删除
	var cnt int64
	if e := h.Repo.DB.Model(&model.Room{}).Where("campus_id = ?", id).Count(&cnt).Error; e != nil {
		response.ServerError(c, e.Error())
		return
	}
	if cnt > 0 {
		response.BadRequest(c, "该校区下仍有 "+strconv.FormatInt(cnt, 10)+" 个机房，请先迁移")
		return
	}
	if e := h.Repo.DB.Delete(&model.Campus{}, id).Error; e != nil {
		if e == gorm.ErrRecordNotFound {
			response.NotFound(c, "校区不存在")
			return
		}
		response.ServerError(c, e.Error())
		return
	}
	response.Success(c, gin.H{"deleted": id})
}

// ---- 机房归属校区 ----

// AssignRoomCampus PUT /api/v1/campuses/:id/rooms/:room_id（JWT）
func (h *Handler) AssignRoomCampus(c *gin.Context) {
	campusID := c.Param("id")
	roomID := c.Param("room_id")
	var campus model.Campus
	if e := h.Repo.DB.First(&campus, campusID).Error; e != nil {
		response.NotFound(c, "校区不存在")
		return
	}
	var room model.Room
	if e := h.Repo.DB.First(&room, roomID).Error; e != nil {
		response.NotFound(c, "机房不存在")
		return
	}
	room.CampusID = &campus.ID
	if e := h.Repo.DB.Save(&room).Error; e != nil {
		response.ServerError(c, e.Error())
		return
	}
	response.Success(c, gin.H{"room_id": room.ID, "campus_id": campus.ID, "campus": campus.Name})
}

// UnassignRoomCampus DELETE /api/v1/campuses/rooms/:room_id（JWT）：移出校区
func (h *Handler) UnassignRoomCampus(c *gin.Context) {
	roomID := c.Param("room_id")
	var room model.Room
	if e := h.Repo.DB.First(&room, roomID).Error; e != nil {
		response.NotFound(c, "机房不存在")
		return
	}
	room.CampusID = nil
	if e := h.Repo.DB.Save(&room).Error; e != nil {
		response.ServerError(c, e.Error())
		return
	}
	response.Success(c, gin.H{"room_id": room.ID, "campus_id": nil})
}

// ---- 跨校区汇总看板 ----

// CampusDashboard GET /api/v1/campuses/:id/dashboard（JWT）
func (h *Handler) CampusDashboard(c *gin.Context) {
	id := c.Param("id")
	var campus model.Campus
	if e := h.Repo.DB.First(&campus, id).Error; e != nil {
		response.NotFound(c, "校区不存在")
		return
	}
	var rooms []model.Room
	h.Repo.DB.Where("campus_id = ?", id).Find(&rooms)
	roomIDs := make([]uint, 0, len(rooms))
	for _, r := range rooms {
		roomIDs = append(roomIDs, r.ID)
	}
	summary := gin.H{
		"campus":      campus,
		"room_count":  len(rooms),
		"device_count": 0,
		"online_count": 0,
	}
	if len(roomIDs) > 0 {
		var seatTotal, onlineTotal int64
		h.Repo.DB.Model(&model.Seat{}).Where("room_id IN ?", roomIDs).Count(&seatTotal)
		h.Repo.DB.Model(&model.Seat{}).Where("room_id IN ? AND status = ?", roomIDs, "使用中").Count(&onlineTotal)
		summary["device_count"] = seatTotal
		summary["online_count"] = onlineTotal
		// 每机房明细
		details := make([]gin.H, 0, len(rooms))
		for _, r := range rooms {
			var dev, on int64
			h.Repo.DB.Model(&model.Seat{}).Where("room_id = ?", r.ID).Count(&dev)
			h.Repo.DB.Model(&model.Seat{}).Where("room_id = ? AND status = ?", r.ID, "使用中").Count(&on)
			details = append(details, gin.H{
				"room_id": r.ID, "room_name": r.Name,
				"device_count": dev, "online_count": on,
			})
		}
		summary["rooms"] = details
	} else {
		summary["rooms"] = []gin.H{}
	}
	response.Success(c, summary)
}

// GlobalDashboard GET /api/v1/dashboard/global（JWT）：全部校区总览
func (h *Handler) GlobalDashboard(c *gin.Context) {
	var campuses []model.Campus
	h.Repo.DB.Order("id").Find(&campuses)
	items := make([]gin.H, 0, len(campuses)+1)
	var totalRooms, totalDevices, totalOnline int64
	for _, cp := range campuses {
		var rooms int64
		h.Repo.DB.Model(&model.Room{}).Where("campus_id = ?", cp.ID).Count(&rooms)
		var devs, ons int64
		h.Repo.DB.Model(&model.Seat{}).Joins("JOIN t_room ON t_room.id = t_seat.room_id").
			Where("t_room.campus_id = ?", cp.ID).Count(&devs)
		h.Repo.DB.Model(&model.Seat{}).Joins("JOIN t_room ON t_room.id = t_seat.room_id").
			Where("t_room.campus_id = ? AND t_seat.status = ?", cp.ID, "使用中").Count(&ons)
		totalRooms += rooms
		totalDevices += devs
		totalOnline += ons
		items = append(items, gin.H{
			"campus_id": cp.ID, "campus_name": cp.Name, "code": cp.Code,
			"room_count": rooms, "device_count": devs, "online_count": ons,
		})
	}
	// 未分配机房的机房
	var unRooms, unDevs int64
	h.Repo.DB.Model(&model.Room{}).Where("campus_id IS NULL").Count(&unRooms)
	h.Repo.DB.Model(&model.Seat{}).Joins("JOIN t_room ON t_room.id = t_seat.room_id").
		Where("t_room.campus_id IS NULL").Count(&unDevs)
	totalRooms += unRooms
	totalDevices += unDevs
	items = append(items, gin.H{
		"campus_id": 0, "campus_name": "未分配", "code": "-",
		"room_count": unRooms, "device_count": unDevs, "online_count": 0,
	})
	response.Success(c, gin.H{
		"campus_count":  len(campuses),
		"room_count":    totalRooms,
		"device_count":  totalDevices,
		"online_count":  totalOnline,
		"campuses":      items,
	})
}

// ListCampusRooms GET /api/v1/campuses/:id/rooms（JWT）
func (h *Handler) ListCampusRooms(c *gin.Context) {
	id := c.Param("id")
	var rooms []model.Room
	h.Repo.DB.Where("campus_id = ?", id).Order("id").Find(&rooms)
	response.Success(c, rooms)
}
