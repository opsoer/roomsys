// Package handlers 处理公寓楼栋相关接口，包括公寓的增删改查、套餐升级等
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BuildingHandler 公寓处理器，依赖数据库连接和公寓服务
type BuildingHandler struct {
	DB             *gorm.DB
	BuildingService *services.BuildingService
}

// defaultContractDurationYears 合同默认期限为 1 年
const defaultContractDurationYears = 1

// CreateBuildingReq 创建公寓请求参数
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

// UpdateBuildingReq 更新公寓信息请求参数
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

// Create 创建新公寓，包含名称查重和房东信息录入
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
	exists, err := h.BuildingService.ExistsByName(req.Name)
	if err != nil {
		logger.Log.Error().Err(err).Msg("检查公寓名称失败")
		utils.Error(c, http.StatusInternalServerError, "创建公寓失败")
		return
	}
	if exists {
		suggestedName, err := h.BuildingService.GenerateSuggestedName(req.Name)
		if err != nil {
			logger.Log.Warn().Str("name", req.Name).Msg("生成建议名称失败")
			utils.Error(c, http.StatusConflict, "公寓名称已存在")
			return
		}
		logger.Log.Warn().Str("name", req.Name).Str("suggested", suggestedName).Msg("创建公寓失败: 名称已存在")
		c.JSON(http.StatusConflict, utils.APIResponse{
			Code:    utils.CodeNameConflict,
			Message: "公寓名称已存在",
			Data:    gin.H{"suggested_name": suggestedName},
		})
		return
	}
	if err := h.BuildingService.Create(&building); err != nil {
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

// Update 更新公寓信息，支持修改名称、地址、封面等
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
	if req.CoverImage != "" {
		updates["cover_image"] = req.CoverImage
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	if req.Name != "" && req.Name != building.Name {
		exists, err := h.BuildingService.ExistsByName(req.Name)
		if err != nil {
			logger.Log.Error().Err(err).Msg("检查公寓名称失败")
			utils.Error(c, http.StatusInternalServerError, "更新失败")
			return
		}
		if exists {
			suggestedName, err := h.BuildingService.GenerateSuggestedName(req.Name)
			if err != nil {
				utils.Error(c, http.StatusConflict, "公寓名称已存在")
				return
			}
			c.JSON(http.StatusConflict, utils.APIResponse{
				Code:    utils.CodeNameConflict,
				Message: "公寓名称已存在",
				Data:    gin.H{"suggested_name": suggestedName},
			})
			return
		}
	}
	if len(updates) > 0 {
		if err := h.BuildingService.Update(building.ID, updates); err != nil {
			logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("更新公寓数据库失败")
			utils.Error(c, http.StatusConflict, "更新失败")
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

// Delete 删除指定公寓（存在活跃合同时禁止删除）
func (h *BuildingHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	if err := h.BuildingService.Delete(uint(buildingID)); err != nil {
		if err.Error() == "active_contracts_exist" {
			logger.Log.Warn().Str("id", id).Msg("删除公寓失败: 存在活跃合同")
			utils.ErrorWithCode(c, http.StatusConflict, utils.CodeActiveContract, "该公寓存在活跃合同，无法删除")
			return
		}
		logger.Log.Error().Err(err).Str("id", id).Msg("删除公寓失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	logger.Log.Info().Str("id", id).Msg("公寓已删除")
	utils.SuccessWithMsg(c, "删除成功", nil)
}

// UpgradePackage 升级公寓套餐（basic/full）
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

// List 获取公寓列表，支持状态和关键词筛选（管理端）
func (h *BuildingHandler) List(c *gin.Context) {
	status := c.Query("status")
	keyword := c.Query("keyword")
	page, size := utils.ParsePage(c)
	buildings, total, err := h.BuildingService.List(status, keyword, "", "", "", page, size)
	if err != nil {
		logger.Log.Error().Err(err).Msg("查询公寓列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"buildings": buildings, "total": total, "page": page, "size": size})
}

// ListPublic 获取公寓列表（公开端），按地区筛选并记录访问
func (h *BuildingHandler) ListPublic(c *gin.Context) {
	page, size := utils.ParsePage(c)
	district := c.Query("district")
	street := c.Query("street")
	village := c.Query("village")
	buildings, total, err := h.BuildingService.List("", "", district, street, village, page, size)
	if err != nil {
		logger.Log.Error().Err(err).Msg("查询公寓列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	go utils.RecordPageView(h.DB, "building_list", 0, 0, utils.GetRealIP(c))
	utils.Success(c, gin.H{"buildings": buildings, "total": total, "page": page, "size": size})
}

// GetPublic 获取公寓详情（公开端），含统计信息并记录访问
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
	go utils.RecordPageView(h.DB, "building_detail", uint(buildingID), uint(buildingID), utils.GetRealIP(c))
	utils.Success(c, gin.H{"building": building})
}

// GetRooms 获取公寓的房间列表，支持楼层、户型、状态筛选
func (h *BuildingHandler) GetRooms(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	requestedStatus := c.Query("status")
	page, size := utils.ParsePage(c)
	query := h.DB.Where("building_id = ? AND status != ?", buildingID, "reserved")
	if floor := c.Query("floor"); floor != "" {
		query = query.Where("floor = ?", floor)
	}
	if layout := c.Query("layout"); layout != "" {
		query = query.Where("layout = ?", layout)
	}

	var total int64
	if err := query.Model(&models.Room{}).Count(&total).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", uint(buildingID)).Msg("查询房间总数失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}

	var rooms []models.Room
	if err := query.Preload("Media").Offset((page - 1) * size).Limit(size).Find(&rooms).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", uint(buildingID)).Msg("查询房间列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}

	roomIDs := make([]uint, len(rooms))
	for i, r := range rooms {
		roomIDs[i] = r.ID
	}
	var contracts []models.RentalContract
	if len(roomIDs) > 0 {
		h.DB.Where("room_id IN ? AND status = ?", roomIDs, "active").Find(&contracts)
	}
	contractMap := make(map[uint]string)
	for _, ct := range contracts {
		contractMap[ct.RoomID] = ct.EndDate
	}

	type RoomWithThumbnail struct {
		models.Room
		Thumbnail string `json:"thumbnail"`
		EndDate   string `json:"end_date"`
	}
	var result []RoomWithThumbnail
	for _, r := range rooms {
		thumb := ""
		for _, m := range r.Media {
			if m.Category == "cover" && m.Type == "image" {
				if m.ThumbnailPath != "" {
					thumb = m.ThumbnailPath
				} else {
					thumb = m.FilePath
				}
				break
			}
		}
		if thumb == "" {
			for _, m := range r.Media {
				if m.Type == "image" {
					if m.ThumbnailPath != "" {
						thumb = m.ThumbnailPath
					} else {
						thumb = m.FilePath
					}
					break
				}
			}
		}
		endDate := contractMap[r.ID]
		dynStatus := utils.DynamicRoomStatus(r.Status, endDate)
		r.Status = dynStatus
		if requestedStatus != "" && dynStatus != requestedStatus {
			continue
		}
		result = append(result, RoomWithThumbnail{Room: r, Thumbnail: thumb, EndDate: endDate})
	}
	utils.Success(c, gin.H{"rooms": result, "total": total, "page": page, "size": size})
}

// MyBuilding 获取当前管理员所属的公寓信息
func (h *BuildingHandler) MyBuilding(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
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

// UpdateMyBuilding 更新当前管理员所属的公寓信息
func (h *BuildingHandler) UpdateMyBuilding(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
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
	if req.CoverImage != "" {
		updates["cover_image"] = req.CoverImage
	}
	if err := h.BuildingService.Update(bid, updates); err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("更新公寓信息失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	utils.SuccessWithMsg(c, "更新成功", nil)
}

// MyStats 获取当前管理员所属公寓的统计数据
func (h *BuildingHandler) MyStats(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
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

