package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type RoomHandler struct {
	DB          *gorm.DB
	Cfg         *config.Config
	RoomService *services.RoomService
}

func (h *RoomHandler) hasAuth(c *gin.Context) bool {
	tokenStr := c.GetHeader("Authorization")
	if tokenStr == "" || !strings.HasPrefix(tokenStr, "Bearer ") {
		return false
	}
	tokenStr = strings.TrimPrefix(tokenStr, "Bearer ")
	_, err := utils.ParseToken(tokenStr, h.Cfg.JWTSecret)
	return err == nil
}

type CreateRoomReq struct {
	RoomNumber  string `json:"room_number" binding:"required"`
	Floor       string `json:"floor" binding:"required"`
	Layout      string `json:"layout" binding:"required"`
	Description string `json:"description"`
}

type UpdateRoomReq struct {
	RoomNumber  string `json:"room_number"`
	Floor       string `json:"floor"`
	Layout      string `json:"layout"`
	Description string `json:"description"`
}

type UpdateRoomStatusReq struct {
	Status          string   `json:"status" binding:"required"`
	TenantName      string   `json:"tenant_name"`
	TenantPhone     string   `json:"tenant_phone"`
	RentPrice       float64  `json:"rent_price"`
	Deposit         float64  `json:"deposit"`
	EarnestMoney    float64  `json:"earnest_money"`
	StartDate       string   `json:"start_date"`
	EndDate         string   `json:"end_date"`
	RefundedDeposit *float64 `json:"refunded_deposit"`
}

type UpdateContractReq struct {
	EndDate   string  `json:"end_date" binding:"required"`
	RentPrice float64 `json:"rent_price"`
}

func (h *RoomHandler) GetPublic(c *gin.Context) {
	roomID := c.Param("rid")
	buildingID := c.Param("id")
	rid, _ := strconv.ParseUint(roomID, 10, 32)
	bid, _ := strconv.ParseUint(buildingID, 10, 32)

	room, contract, err := h.RoomService.GetWithContract(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}
	if room.BuildingID != uint(bid) {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}

	type RoomDetail struct {
		models.Room
		CurrentContract *models.RentalContract `json:"current_contract,omitempty"`
		EndDate         string                  `json:"end_date"`
	}
	detail := RoomDetail{Room: *room}
	if h.hasAuth(c) {
		tokenStr := strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
		claims, err := utils.ParseToken(tokenStr, h.Cfg.JWTSecret)
		if err == nil && claims.BuildingID > 0 && claims.BuildingID == room.BuildingID {
			if contract != nil {
				detail.CurrentContract = contract
				detail.EndDate = contract.EndDate
			}
		}
	}
	utils.Success(c, gin.H{"room": detail})
}

func (h *RoomHandler) GetActiveContractPublic(c *gin.Context) {
	roomID := c.Param("rid")
	bid, _ := strconv.ParseUint(c.Param("id"), 10, 32)
	rid, _ := strconv.ParseUint(roomID, 10, 32)

	room, err := h.RoomService.GetByID(uint(rid))
	if err != nil || room.BuildingID != uint(bid) {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}

	contract, err := h.RoomService.GetActiveContract(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "无有效合同")
		return
	}
	utils.Success(c, gin.H{"contract": contract})
}

func (h *RoomHandler) List(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}

	rooms, err := h.RoomService.List(bid)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询房间列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"rooms": rooms})
}

func (h *RoomHandler) Get(c *gin.Context) {
	roomID := c.Param("id")
	rid, _ := strconv.ParseUint(roomID, 10, 32)

	room, contract, err := h.RoomService.GetWithContract(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}

	type RoomDetail struct {
		models.Room
		CurrentContract *models.RentalContract `json:"current_contract,omitempty"`
	}
	detail := RoomDetail{Room: *room, CurrentContract: contract}
	utils.Success(c, gin.H{"room": detail})
}

func (h *RoomHandler) Create(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}

	var req CreateRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	room := models.Room{
		BuildingID:  bid,
		RoomNumber:  req.RoomNumber,
		Floor:       req.Floor,
		Layout:      req.Layout,
		Description: req.Description,
		Status:      "vacant",
	}

	if err := h.RoomService.Create(&room); err != nil {
		if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "UNIQUE") {
			utils.Error(c, http.StatusConflict, "房间号已存在")
			return
		}
		logger.Log.Error().Err(err).Msg("创建房间失败")
		utils.Error(c, http.StatusInternalServerError, "创建失败")
		return
	}

	utils.Created(c, "创建成功", gin.H{"room": room})
}

func (h *RoomHandler) Update(c *gin.Context) {
	roomID := c.Param("id")
	rid, _ := strconv.ParseUint(roomID, 10, 32)

	var req UpdateRoomReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	updates := map[string]interface{}{}
	if req.RoomNumber != "" {
		updates["room_number"] = req.RoomNumber
	}
	if req.Floor != "" {
		updates["floor"] = req.Floor
	}
	if req.Layout != "" {
		updates["layout"] = req.Layout
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}

	if err := h.RoomService.Update(uint(rid), updates); err != nil {
		if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "UNIQUE") {
			utils.Error(c, http.StatusConflict, "房间号已存在")
			return
		}
		logger.Log.Error().Err(err).Msg("更新房间失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}

	utils.SuccessWithMsg(c, "更新成功", nil)
}

func (h *RoomHandler) Delete(c *gin.Context) {
	roomID := c.Param("id")
	rid, _ := strconv.ParseUint(roomID, 10, 32)

	if err := h.RoomService.Delete(uint(rid)); err != nil {
		logger.Log.Error().Err(err).Msg("删除房间失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}

	utils.SuccessWithMsg(c, "删除成功", nil)
}

func (h *RoomHandler) UpdateStatus(c *gin.Context) {
	roomID := c.Param("id")
	rid, _ := strconv.ParseUint(roomID, 10, 32)

	var req UpdateRoomStatusReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	room, err := h.RoomService.GetByID(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}

	if req.Status == "rented" {
		tenant := models.Tenant{
			Name:  req.TenantName,
			Phone: req.TenantPhone,
		}
		if err := h.RoomService.CreateTenant(&tenant); err != nil {
			logger.Log.Error().Err(err).Msg("创建租客失败")
			utils.Error(c, http.StatusInternalServerError, "创建失败")
			return
		}

		contract := models.RentalContract{
			RoomID:     room.ID,
			BuildingID: room.BuildingID,
			TenantID:   tenant.ID,
			RentPrice:  req.RentPrice,
			Deposit:    req.Deposit,
			StartDate:  req.StartDate,
			EndDate:    req.EndDate,
			Status:     "active",
		}
		if err := h.RoomService.CreateContract(&contract); err != nil {
			logger.Log.Error().Err(err).Msg("创建合同失败")
			utils.Error(c, http.StatusInternalServerError, "创建失败")
			return
		}

		h.RoomService.UpdateStatus(room.ID, "rented")
	} else if req.Status == "vacant" {
		h.RoomService.UpdateStatus(room.ID, "vacant")
	}

	utils.SuccessWithMsg(c, "状态更新成功", nil)
}

func (h *RoomHandler) GetActiveContract(c *gin.Context) {
	roomID := c.Param("id")
	rid, _ := strconv.ParseUint(roomID, 10, 32)

	contract, err := h.RoomService.GetActiveContract(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "无有效合同")
		return
	}
	utils.Success(c, gin.H{"contract": contract})
}

func (h *RoomHandler) RenewContract(c *gin.Context) {
	roomID := c.Param("id")
	rid, _ := strconv.ParseUint(roomID, 10, 32)

	var req UpdateContractReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	contract, err := h.RoomService.GetActiveContract(uint(rid))
	if err != nil {
		utils.Error(c, http.StatusNotFound, "无有效合同")
		return
	}

	updates := map[string]interface{}{
		"end_date": req.EndDate,
	}
	if req.RentPrice > 0 {
		updates["rent_price"] = req.RentPrice
	}

	if err := h.RoomService.UpdateContract(contract.ID, updates); err != nil {
		logger.Log.Error().Err(err).Msg("续租失败")
		utils.Error(c, http.StatusInternalServerError, "续租失败")
		return
	}

	utils.SuccessWithMsg(c, "续租成功", nil)
}
