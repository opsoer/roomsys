package handlers

import (
	"net/http"
	"strings"
	"time"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BuildingHandler struct {
	DB *gorm.DB
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
	tx := h.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if err := tx.Create(&building).Error; err != nil {
		tx.Rollback()
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
		if err := tx.Create(&ll).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("创建房东信息失败")
			utils.Error(c, http.StatusInternalServerError, "创建公寓失败")
			return
		}
	}
	tx.Commit()
	logger.Log.Info().Uint("building_id", building.ID).Str("name", building.Name).Uint("created_by", uid).Msg("公寓创建成功")
	utils.Created(c, "公寓创建成功", gin.H{"building": building})
}

func (h *BuildingHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var building models.Building
	if err := h.DB.First(&building, id).Error; err != nil {
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
	tx := h.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if len(updates) > 0 {
		if err := tx.Model(&building).Updates(updates).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("更新公寓数据库失败")
			if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "UNIQUE") {
				utils.Error(c, http.StatusConflict, "公寓名称已存在")
				return
			}
			utils.Error(c, http.StatusConflict, "更新失败，请检查名称是否重复")
			return
		}
	}
	if req.Landlords != nil {
		if err := tx.Where("building_id = ?", building.ID).Delete(&models.BuildingLandlord{}).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("删除旧房东信息失败")
			utils.Error(c, http.StatusInternalServerError, "更新失败")
			return
		}
		for _, l := range req.Landlords {
			ll := models.BuildingLandlord{
				BuildingID: building.ID,
				Name:       l.Name,
				Phone:      l.Phone,
			}
			if err := tx.Create(&ll).Error; err != nil {
				tx.Rollback()
				logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("创建房东信息失败")
				utils.Error(c, http.StatusInternalServerError, "更新失败")
				return
			}
		}
	}
	tx.Commit()
	logger.Log.Info().Uint("building_id", building.ID).Str("name", building.Name).Msg("公寓信息更新成功")
	utils.SuccessWithMsg(c, "更新成功", nil)
}

func (h *BuildingHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	var building models.Building
	if err := h.DB.First(&building, id).Error; err != nil {
		logger.Log.Warn().Str("id", id).Msg("删除公寓失败: 公寓不存在")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	tx := h.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()
	if err := tx.Where("building_id = ?", building.ID).Delete(&models.Dividend{}).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("删除公寓分红记录失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	if err := tx.Where("building_id = ?", building.ID).Delete(&models.Shareholder{}).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("删除公寓股东信息失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	if err := tx.Where("building_id = ?", building.ID).Delete(&models.Bill{}).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("删除公寓账单失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	if err := tx.Where("building_id = ?", building.ID).Delete(&models.Task{}).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("删除公寓任务失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	var roomIDs []uint
	tx.Model(&models.Room{}).Where("building_id = ?", building.ID).Pluck("id", &roomIDs)
	if len(roomIDs) > 0 {
		if err := tx.Where("room_id IN ?", roomIDs).Delete(&models.RentalContract{}).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("删除公寓合同失败")
			utils.Error(c, http.StatusInternalServerError, "删除失败")
			return
		}
		if err := tx.Where("room_id IN ?", roomIDs).Delete(&models.RoomMedia{}).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("删除公寓房间媒体失败")
			utils.Error(c, http.StatusInternalServerError, "删除失败")
			return
		}
	}
	if err := tx.Where("building_id = ?", building.ID).Delete(&models.Room{}).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("删除公寓房间失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	if err := tx.Where("building_id = ?", building.ID).Delete(&models.BuildingLandlord{}).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("删除公寓房东信息失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	if err := tx.Delete(&building).Error; err != nil {
		tx.Rollback()
		logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("删除公寓失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	tx.Commit()
	logger.Log.Info().Uint("building_id", building.ID).Str("name", building.Name).Msg("公寓已删除")
	utils.SuccessWithMsg(c, "删除成功", nil)
}

func (h *BuildingHandler) List(c *gin.Context) {
	var buildings []models.Building
	now := utils.Now()
	query := h.DB.Order("created_at desc")

	statusFilter := c.Query("status")
	keyword := c.Query("keyword")

	if statusFilter == "normal" {
		query = query.Where("status = ? AND (expired_at IS NULL OR expired_at = '' OR expired_at > ?)", "active", now.AddDate(0, 0, 30).Format("2006-01-02"))
	} else if statusFilter == "expiring" {
		query = query.Where("status = ? AND expired_at IS NOT NULL AND expired_at != '' AND expired_at <= ? AND expired_at > ?", "active", now.AddDate(0, 0, 30).Format("2006-01-02"), now.Format("2006-01-02"))
	} else if statusFilter == "expired" {
		query = query.Where("status = ? OR (expired_at IS NOT NULL AND expired_at != '' AND expired_at <= ?)", "expired", now.Format("2006-01-02"))
	} else if statusFilter != "" {
		query = query.Where("status = ?", statusFilter)
	}

	if district := c.Query("district"); district != "" {
		query = query.Where("district = ?", district)
	}

	if err := query.Find(&buildings).Error; err != nil {
		logger.Log.Error().Err(err).Msg("查询公寓列表失败")
	}

	if keyword != "" {
		var nameMatchIDs []uint
		if err := h.DB.Model(&models.Building{}).Select("id").Where("name LIKE ?", "%"+keyword+"%").Find(&nameMatchIDs).Error; err != nil {
			logger.Log.Error().Err(err).Msg("搜索公寓名称失败")
		}
		var phoneMatchIDs []uint
		if err := h.DB.Model(&models.BuildingLandlord{}).Select("building_id").Where("phone LIKE ?", "%"+keyword+"%").Find(&phoneMatchIDs).Error; err != nil {
			logger.Log.Error().Err(err).Msg("搜索公寓电话失败")
		}
		matchIDs := make(map[uint]bool)
		for _, id := range nameMatchIDs {
			matchIDs[id] = true
		}
		for _, id := range phoneMatchIDs {
			matchIDs[id] = true
		}
		filtered := make([]models.Building, 0)
		for _, b := range buildings {
			if matchIDs[b.ID] {
				filtered = append(filtered, b)
				continue
			}
		}
		buildings = filtered
	}

	type BuildingVO struct {
		models.Building
		BuildingStatus string                     `json:"building_status"`
		Landlords      []models.BuildingLandlord `json:"landlords"`
		RoomCount      int64                      `json:"room_count"`
		VacantCount    int64                      `json:"vacant_count"`
		RentedCount    int64                      `json:"rented_count"`
		ExpiringCount  int64                      `json:"expiring_count"`
	}
	result := make([]BuildingVO, 0)
	for _, b := range buildings {
		var landlords []models.BuildingLandlord
		if err := h.DB.Where("building_id = ?", b.ID).Find(&landlords).Error; err != nil {
			logger.Log.Error().Err(err).Uint("building_id", b.ID).Msg("查询房东列表失败")
		}
		var roomCount, vacantCount, rentedCount, expiringCount int64
		h.DB.Model(&models.Room{}).Where("building_id = ?", b.ID).Count(&roomCount)
		h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", b.ID, "vacant").Count(&vacantCount)
		h.DB.Model(&models.Room{}).Where("building_id = ? AND status IN ?", b.ID, []string{"rented", "expiring"}).Count(&rentedCount)
		h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", b.ID, "expiring").Count(&expiringCount)

		buildingStatus := "normal"
		if b.Status == "expired" {
			buildingStatus = "expired"
		} else if b.ExpiredAt != "" {
			if expDate, err := time.Parse("2006-01-02", b.ExpiredAt); err == nil {
				if now.After(expDate) {
					buildingStatus = "expired"
				} else if expDate.Sub(now) <= 30*24*time.Hour {
					buildingStatus = "expiring"
				}
			}
		}
		result = append(result, BuildingVO{
			Building:       b,
			BuildingStatus: buildingStatus,
			Landlords:      landlords,
			RoomCount:      roomCount,
			VacantCount:    vacantCount,
			RentedCount:    rentedCount,
			ExpiringCount:  expiringCount,
		})
	}
	logger.Log.Debug().Int("count", len(result)).Str("status", statusFilter).Msg("查询公寓列表")
	utils.Success(c, gin.H{"buildings": result})
}

func (h *BuildingHandler) ListPublic(c *gin.Context) {
	logger.Log.Debug().Str("district", c.Query("district")).Msg("公开查询公寓列表")
	var buildings []models.Building
	query := h.DB.Where("status = ?", "active").Order("created_at desc")
	if district := c.Query("district"); district != "" {
		query = query.Where("district = ?", district)
	}
	if err := query.Find(&buildings).Error; err != nil {
		logger.Log.Error().Err(err).Msg("公开查询公寓列表失败")
	}
	now := utils.Now()
	validBuildings := make([]models.Building, 0)
	for _, b := range buildings {
		if b.ExpiredAt != "" {
			if expDate, err := time.Parse("2006-01-02", b.ExpiredAt); err == nil && now.After(expDate) {
				continue
			}
		}
		validBuildings = append(validBuildings, b)
	}
	buildings = validBuildings
	type BuildingPublicVO struct {
		ID            uint                      `json:"id"`
		Name          string                    `json:"name"`
		District      string                    `json:"district"`
		Street        string                    `json:"street"`
		Village       string                    `json:"village"`
		BuildingNo    string                    `json:"building_no"`
		CoverImage    string                    `json:"cover_image"`
		Description   string                    `json:"description"`
		Landlords     []models.BuildingLandlord `json:"landlords"`
		RoomCount     int64                     `json:"room_count"`
		VacantCount   int64                     `json:"vacant_count"`
		RentedCount   int64                     `json:"rented_count"`
		ExpiringCount int64                     `json:"expiring_count"`
	}
	result := make([]BuildingPublicVO, 0)
	for _, b := range buildings {
		var landlords []models.BuildingLandlord
		if err := h.DB.Where("building_id = ?", b.ID).Find(&landlords).Error; err != nil {
			logger.Log.Error().Err(err).Uint("building_id", b.ID).Msg("公开查询房东列表失败")
		}
		var roomCount, vacantCount, rentedCount, expiringCount int64
		h.DB.Model(&models.Room{}).Where("building_id = ?", b.ID).Count(&roomCount)
		h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", b.ID, "vacant").Count(&vacantCount)
		h.DB.Model(&models.Room{}).Where("building_id = ? AND status IN ?", b.ID, []string{"rented", "expiring"}).Count(&rentedCount)
		h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", b.ID, "expiring").Count(&expiringCount)
		result = append(result, BuildingPublicVO{
			ID:            b.ID,
			Name:          b.Name,
			District:      b.District,
			Street:        b.Street,
			Village:       b.Village,
			BuildingNo:    b.BuildingNo,
			CoverImage:    b.CoverImage,
			Description:   b.Description,
			Landlords:     landlords,
			RoomCount:     roomCount,
			VacantCount:   vacantCount,
			RentedCount:   rentedCount,
			ExpiringCount: expiringCount,
		})
	}
	utils.Success(c, gin.H{"buildings": result})
}

func (h *BuildingHandler) GetPublic(c *gin.Context) {
	id := c.Param("id")
	var building models.Building
	if err := h.DB.First(&building, id).Error; err != nil {
		logger.Log.Warn().Str("id", id).Msg("公开查询公寓详情失败: 不存在")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	logger.Log.Debug().Uint("building_id", building.ID).Msg("公开查询公寓详情")
	var landlords []models.BuildingLandlord
	if err := h.DB.Where("building_id = ?", building.ID).Find(&landlords).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("公开查询房东信息失败")
	}
	var roomCount, vacantCount, rentedCount, expiringCount int64
	h.DB.Model(&models.Room{}).Where("building_id = ?", building.ID).Count(&roomCount)
	h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", building.ID, "vacant").Count(&vacantCount)
	h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", building.ID, "expiring").Count(&expiringCount)
	h.DB.Model(&models.Room{}).Where("building_id = ? AND status IN ?", building.ID, []string{"rented", "expiring"}).Count(&rentedCount)
	utils.Success(c, gin.H{
		"building": gin.H{
			"id":             building.ID,
			"name":           building.Name,
			"district":       building.District,
			"street":         building.Street,
			"village":        building.Village,
			"building_no":    building.BuildingNo,
			"cover_image":    building.CoverImage,
			"description":    building.Description,
			"landlords":      landlords,
			"room_count":     roomCount,
			"vacant_count":   vacantCount,
			"rented_count":   rentedCount,
			"expiring_count": expiringCount,
		},
	})
}

func (h *BuildingHandler) GetRooms(c *gin.Context) {
	id := c.Param("id")
	var building models.Building
	if err := h.DB.First(&building, id).Error; err != nil {
		logger.Log.Warn().Str("id", id).Msg("公开查询公寓房间失败: 公寓不存在")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	logger.Log.Debug().Uint("building_id", building.ID).Msg("公开查询公寓房间列表")
	var rooms []models.Room
	query := h.DB.Preload("Media", func(db *gorm.DB) *gorm.DB {
		return db.Where("type = ?", "image").Order("FIELD(category,'cover','gallery'), sort_order asc")
	}).Where("building_id = ?", id)
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if floor := c.Query("floor"); floor != "" {
		query = query.Where("floor = ?", floor)
	}
	if layout := c.Query("layout"); layout != "" {
		query = query.Where("layout = ?", layout)
	}
	if err := query.Find(&rooms).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("查询公寓房间失败")
	}
	type RoomVO struct {
		ID          uint   `json:"id"`
		RoomNumber  string `json:"room_number"`
		Floor       string `json:"floor"`
		Layout      string `json:"layout"`
		Status      string `json:"status"`
		Description string `json:"description"`
		Thumbnail   string `json:"thumbnail"`
		EndDate     string `json:"end_date"`
	}
	result := make([]RoomVO, 0)
	for _, r := range rooms {
		vo := RoomVO{
			ID:          r.ID,
			RoomNumber:  r.RoomNumber,
			Floor:       r.Floor,
			Layout:      r.Layout,
			Status:      r.Status,
			Description: r.Description,
		}
		for _, m := range r.Media {
			if m.Category == "cover" {
				vo.Thumbnail = m.FilePath
				break
			}
		}
		if vo.Thumbnail == "" && len(r.Media) > 0 {
			vo.Thumbnail = r.Media[0].FilePath
		}
		var contract models.RentalContract
		if r.Status == "rented" || r.Status == "expiring" {
			if err := h.DB.Where("room_id = ? AND status = ?", r.ID, "active").Select("end_date").First(&contract).Error; err != nil {
				logger.Log.Error().Err(err).Uint("room_id", r.ID).Msg("查询房间合同失败")
			}
			vo.EndDate = contract.EndDate
		}
		result = append(result, vo)
	}
	utils.Success(c, gin.H{"rooms": result})
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
	var building models.Building
	if err := h.DB.First(&building, bid).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("获取当前楼栋信息失败")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	var landlords []models.BuildingLandlord
	if err := h.DB.Where("building_id = ?", bid).Find(&landlords).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询房东信息失败")
	}
	logger.Log.Debug().Uint("building_id", bid).Msg("获取当前楼栋信息")
	utils.Success(c, gin.H{
		"building": building,
		"landlords": landlords,
	})
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
	var building models.Building
	if err := h.DB.First(&building, bid).Error; err != nil {
		logger.Log.Warn().Uint("building_id", bid).Msg("更新楼栋信息失败: 公寓不存在")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	var req UpdateBuildingReq
	if err := c.ShouldBindJSON(&req); err != nil {
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
	tx := h.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	if len(updates) > 0 {
		if err := tx.Model(&building).Updates(updates).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("building_id", bid).Msg("更新楼栋信息数据库失败")
			if strings.Contains(err.Error(), "Duplicate") || strings.Contains(err.Error(), "UNIQUE") {
				utils.Error(c, http.StatusConflict, "公寓名称已存在")
				return
			}
			utils.Error(c, http.StatusConflict, "更新失败，请检查名称是否重复")
			return
		}
	}
	if req.Landlords != nil {
		if err := tx.Where("building_id = ?", bid).Delete(&models.BuildingLandlord{}).Error; err != nil {
			tx.Rollback()
			logger.Log.Error().Err(err).Uint("building_id", bid).Msg("删除旧房东信息失败")
			utils.Error(c, http.StatusInternalServerError, "更新失败")
			return
		}
		for _, l := range req.Landlords {
			ll := models.BuildingLandlord{
				BuildingID: bid,
				Name:       l.Name,
				Phone:      l.Phone,
			}
			if err := tx.Create(&ll).Error; err != nil {
				tx.Rollback()
				logger.Log.Error().Err(err).Uint("building_id", bid).Msg("创建房东信息失败")
				utils.Error(c, http.StatusInternalServerError, "更新失败")
				return
			}
		}
	}
	tx.Commit()
	logger.Log.Info().Uint("building_id", bid).Msg("楼栋信息更新成功")
	utils.SuccessWithMsg(c, "更新成功", nil)
}

type UpgradePackageReq struct {
	Package string `json:"package" binding:"required"`
}

func (h *BuildingHandler) UpgradePackage(c *gin.Context) {
	id := c.Param("id")
	var building models.Building
	if err := h.DB.First(&building, id).Error; err != nil {
		logger.Log.Warn().Str("id", id).Msg("升级套餐失败: 公寓不存在")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	var req UpgradePackageReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("building_id", building.ID).Msg("升级套餐请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Package != "basic" && req.Package != "full" {
		utils.Error(c, http.StatusBadRequest, "套餐类型无效")
		return
	}
	if building.Package == req.Package {
		utils.SuccessWithMsg(c, "套餐未变更", nil)
		return
	}
	if err := h.DB.Model(&building).Update("package", req.Package).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("升级套餐失败")
		utils.Error(c, http.StatusInternalServerError, "升级失败")
		return
	}
	logger.Log.Info().Uint("building_id", building.ID).Str("old", building.Package).Str("new", req.Package).Msg("套餐变更成功")
	utils.SuccessWithMsg(c, "套餐变更成功", nil)
}

// 返回公寓列表中城市区域供筛选
func (h *BuildingHandler) Districts(c *gin.Context) {
	type DistrictRow struct {
		District string `json:"district"`
	}
	var rows []DistrictRow
	if err := h.DB.Model(&models.Building{}).Select("DISTINCT district").Where("district != '' AND status = ?", "active").Find(&rows).Error; err != nil {
		logger.Log.Error().Err(err).Msg("查询区域列表失败")
	}
	districts := make([]string, 0)
	for _, r := range rows {
		if r.District != "" {
			districts = append(districts, r.District)
		}
	}
	utils.Success(c, gin.H{"districts": districts})
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

	var roomCount, vacantCount, rentedCount, expiringCount int64
	h.DB.Model(&models.Room{}).Where("building_id = ?", bid).Count(&roomCount)
	h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", bid, "vacant").Count(&vacantCount)
	h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", bid, "rented").Count(&rentedCount)
	h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", bid, "expiring").Count(&expiringCount)

	var taskCount int64
	h.DB.Model(&models.Task{}).Where("building_id = ? AND status = ?", bid, "pending").Count(&taskCount)

	month := c.Query("month")
	if month == "" {
		month = utils.MonthStr(utils.Now())
	}

	summary := utils.QueryMonthlyFinance(h.DB, bid, month)

	logger.Log.Debug().
		Uint("building_id", bid).
		Str("month", month).
		Int64("room_count", roomCount).
		Float64("income", summary.TotalIncome).
		Float64("expense", summary.TotalExpense).
		Msg("查询楼栋统计")
	utils.Success(c, gin.H{
		"building_id":    bid,
		"room_count":     roomCount,
		"vacant_count":   vacantCount,
		"rented_count":   rentedCount,
		"expiring_count": expiringCount,
		"task_count":     taskCount,
		"month":          month,
		"total_income":   summary.TotalIncome,
		"total_expense":  summary.TotalExpense,
		"net_profit":     summary.NetProfit,
	})
}
