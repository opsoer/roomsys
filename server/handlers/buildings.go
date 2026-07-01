package handlers

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BuildingHandler struct {
	DB             *gorm.DB
	BuildingService *services.BuildingService
}

const defaultContractDurationYears = 1

type CreateBuildingReq struct {
	Name         string `json:"name" binding:"required"`
	Package      string `json:"package"`
	ContractDate string `json:"contract_date"`
	District     string `json:"district"`
	Street       string `json:"street"`
	Village      string `json:"village"`
	BuildingNo   string `json:"building_no"`
	Description  string `json:"description"`
	Landlords    []struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	} `json:"landlords"`
}

type UpdateBuildingReq struct {
	Name         string `json:"name"`
	Package      string `json:"package"`
	ContractDate string `json:"contract_date"`
	District     string `json:"district"`
	Street       string `json:"street"`
	Village      string `json:"village"`
	BuildingNo   string `json:"building_no"`
	CoverImage   string `json:"cover_image"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Landlords    []struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	} `json:"landlords"`
}

func (h *BuildingHandler) Create(c *gin.Context) {
	var req CreateBuildingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Msg("创建公寓请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	uid, ok := userID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}

	pkg := req.Package
	if pkg != "basic" && pkg != "full" {
		pkg = "basic"
	}
	building := models.Building{
		Name:         req.Name,
		Package:      pkg,
		ContractDate: req.ContractDate,
		District:     req.District,
		Street:       req.Street,
		Village:      req.Village,
		BuildingNo:   req.BuildingNo,
		Description:  req.Description,
		Status:       "active",
		CreatedBy:    uid,
	}
	if req.ContractDate != "" {
		if cd, err := time.Parse("2006-01-02", req.ContractDate); err == nil {
			building.ExpiredAt = cd.AddDate(defaultContractDurationYears, 0, 0).Format("2006-01-02")
		}
	}
	if err := h.BuildingService.Create(&building); err != nil {
		if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "Duplicate entry") {
			logger.Log.Warn().Str("name", req.Name).Msg("创建公寓失败: 名称已存在")
			utils.Error(c, http.StatusConflict, "公寓名称已存在")
			return
		}
		logger.Log.Error().Err(err).Str("name", req.Name).Msg("创建公寓数据库失败")
		utils.Error(c, http.StatusInternalServerError, "创建公寓失败")
		return
	}
	for _, l := range req.Landlords {
		ll := models.BuildingLandlord{
			BuildingID: building.ID,
			Name:       l.Name,
			Phone:      l.Phone,
		}
		if err := h.BuildingService.CreateLandlord(&ll); err != nil {
			logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("创建房东信息失败")
			utils.Error(c, http.StatusInternalServerError, "创建公寓失败")
			return
		}
	}
	logger.Log.Info().Uint("building_id", building.ID).Str("name", building.Name).Uint("created_by", uid).Msg("公寓创建成功")
	utils.Created(c, "公寓创建成功", gin.H{"building": building})
}

func (h *BuildingHandler) Update(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	building, err := h.BuildingService.GetByID(uint(buildingID))
	if err != nil {
		logger.Log.Warn().Str("id", id).Msg("更新公寓失败: 公寓不存在")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	var req UpdateBuildingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("building_id", building.ID).Msg("更新公寓请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Package != "" {
		if req.Package == "basic" || req.Package == "full" {
			updates["package"] = req.Package
		}
	}
	if req.ContractDate != "" {
		updates["contract_date"] = req.ContractDate
		if cd, err := time.Parse("2006-01-02", req.ContractDate); err == nil {
			updates["expired_at"] = cd.AddDate(defaultContractDurationYears, 0, 0).Format("2006-01-02")
		}
	}
	if req.District != "" {
		updates["district"] = req.District
	}
	if req.Street != "" {
		updates["street"] = req.Street
	}
	if req.Village != "" {
		updates["village"] = req.Village
	}
	if req.BuildingNo != "" {
		updates["building_no"] = req.BuildingNo
	}
	updates["cover_image"] = req.CoverImage
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	if len(updates) > 0 {
		if err := h.BuildingService.Update(building.ID, updates); err != nil {
			logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("更新公寓数据库失败")
			if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "UNIQUE") {
				utils.Error(c, http.StatusConflict, "公寓名称已存在")
				return
			}
			utils.Error(c, http.StatusConflict, "更新失败，请检查名称是否重复")
			return
		}
	}

	if len(req.Landlords) > 0 {
		h.BuildingService.DeleteLandlords(building.ID)
		for _, l := range req.Landlords {
			ll := models.BuildingLandlord{
				BuildingID: building.ID,
				Name:       l.Name,
				Phone:      l.Phone,
			}
			h.BuildingService.CreateLandlord(&ll)
		}
	}

	logger.Log.Info().Uint("building_id", building.ID).Msg("公寓更新成功")
	utils.SuccessWithMsg(c, "更新成功", nil)
}

func (h *BuildingHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	if err := h.BuildingService.Delete(uint(buildingID)); err != nil {
		logger.Log.Error().Err(err).Str("id", id).Msg("删除公寓失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	logger.Log.Info().Str("id", id).Msg("公寓已删除")
	utils.SuccessWithMsg(c, "删除成功", nil)
}

func (h *BuildingHandler) UpgradePackage(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	var req struct {
		Package string `json:"package" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Package != "basic" && req.Package != "full" {
		utils.Error(c, http.StatusBadRequest, "无效的套餐类型")
		return
	}
	if err := h.BuildingService.UpgradePackage(uint(buildingID), req.Package); err != nil {
		logger.Log.Error().Err(err).Str("id", id).Msg("升级套餐失败")
		utils.Error(c, http.StatusInternalServerError, "升级失败")
		return
	}
	logger.Log.Info().Str("id", id).Str("package", req.Package).Msg("套餐升级成功")
	utils.SuccessWithMsg(c, "升级成功", nil)
}

func (h *BuildingHandler) List(c *gin.Context) {
	status := c.Query("status")
	keyword := c.Query("keyword")
	buildings, err := h.BuildingService.List(status, keyword)
	if err != nil {
		logger.Log.Error().Err(err).Msg("查询公寓列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"buildings": buildings})
}

func (h *BuildingHandler) ListPublic(c *gin.Context) {
	buildings, err := h.BuildingService.List("", "")
	if err != nil {
		logger.Log.Error().Err(err).Msg("查询公寓列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"buildings": buildings})
}

func (h *BuildingHandler) GetPublic(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	building, err := h.BuildingService.GetWithStats(uint(buildingID))
	if err != nil {
		logger.Log.Warn().Str("id", id).Msg("获取公寓详情失败: 公寓不存在")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	utils.Success(c, gin.H{"building": building})
}

func (h *BuildingHandler) GetRooms(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	var rooms []models.Room
	if err := h.DB.Where("building_id = ?", buildingID).Find(&rooms).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", uint(buildingID)).Msg("查询房间列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"rooms": rooms})
}

func (h *BuildingHandler) MyBuilding(c *gin.Context) {
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
	building, err := h.BuildingService.GetWithStats(bid)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("获取公寓信息失败")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	utils.Success(c, gin.H{"building": building})
}

func (h *BuildingHandler) UpdateMyBuilding(c *gin.Context) {
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
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		CoverImage  string `json:"cover_image"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	updates["cover_image"] = req.CoverImage
	if err := h.BuildingService.Update(bid, updates); err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("更新公寓信息失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	utils.SuccessWithMsg(c, "更新成功", nil)
}

func (h *BuildingHandler) MyStats(c *gin.Context) {
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
	stats, err := h.BuildingService.GetWithStats(bid)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("获取公寓统计失败")
		utils.Error(c, http.StatusInternalServerError, "获取统计失败")
		return
	}
	utils.Success(c, gin.H{"stats": stats})
}

func (h *BuildingHandler) Districts(c *gin.Context) {
	utils.Success(c, gin.H{"districts": []interface{}{}})
}
