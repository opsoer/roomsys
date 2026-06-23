package handlers

import (
	"net/http"

	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type BuildingHandler struct {
	DB *gorm.DB
}

type CreateBuildingReq struct {
	Name        string `json:"name" binding:"required"`
	District    string `json:"district"`
	Street      string `json:"street"`
	Village     string `json:"village"`
	BuildingNo  string `json:"building_no"`
	Description string `json:"description"`
	Landlords   []struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	} `json:"landlords"`
}

type UpdateBuildingReq struct {
	Name        string `json:"name"`
	District    string `json:"district"`
	Street      string `json:"street"`
	Village     string `json:"village"`
	BuildingNo  string `json:"building_no"`
	CoverImage  string `json:"cover_image"`
	Description string `json:"description"`
	Status      string `json:"status"`
	Landlords   []struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	} `json:"landlords"`
}

func (h *BuildingHandler) Create(c *gin.Context) {
	var req CreateBuildingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	userID, _ := c.Get("user_id")
	building := models.Building{
		Name:        req.Name,
		District:    req.District,
		Street:      req.Street,
		Village:     req.Village,
		BuildingNo:  req.BuildingNo,
		Description: req.Description,
		Status:      "active",
		CreatedBy:   userID.(uint),
	}
	if err := h.DB.Create(&building).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建公寓失败"})
		return
	}
	for _, l := range req.Landlords {
		ll := models.BuildingLandlord{
			BuildingID: building.ID,
			Name:       l.Name,
			Phone:      l.Phone,
		}
		h.DB.Create(&ll)
	}
	c.JSON(http.StatusCreated, gin.H{"building": building, "message": "公寓创建成功"})
}

func (h *BuildingHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var building models.Building
	if err := h.DB.First(&building, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "公寓不存在"})
		return
	}
	var req UpdateBuildingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
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
	if len(updates) > 0 {
		h.DB.Model(&building).Updates(updates)
	}
	if req.Landlords != nil {
		h.DB.Where("building_id = ?", building.ID).Delete(&models.BuildingLandlord{})
		for _, l := range req.Landlords {
			ll := models.BuildingLandlord{
				BuildingID: building.ID,
				Name:       l.Name,
				Phone:      l.Phone,
			}
			h.DB.Create(&ll)
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

func (h *BuildingHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	var building models.Building
	if err := h.DB.First(&building, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "公寓不存在"})
		return
	}
	h.DB.Delete(&building)
	h.DB.Where("building_id = ?", building.ID).Delete(&models.BuildingLandlord{})
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (h *BuildingHandler) List(c *gin.Context) {
	var buildings []models.Building
	query := h.DB.Order("created_at desc")
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	if district := c.Query("district"); district != "" {
		query = query.Where("district = ?", district)
	}
	query.Find(&buildings)
	type BuildingVO struct {
		models.Building
		Landlords     []models.BuildingLandlord `json:"landlords"`
		RoomCount     int64                      `json:"room_count"`
		VacantCount   int64                      `json:"vacant_count"`
		RentedCount   int64                      `json:"rented_count"`
		ExpiringCount int64                      `json:"expiring_count"`
	}
	result := make([]BuildingVO, 0)
	for _, b := range buildings {
		var landlords []models.BuildingLandlord
		h.DB.Where("building_id = ?", b.ID).Find(&landlords)
		var roomCount, vacantCount, rentedCount, expiringCount int64
		h.DB.Model(&models.Room{}).Where("building_id = ?", b.ID).Count(&roomCount)
		h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", b.ID, "vacant").Count(&vacantCount)
		h.DB.Model(&models.Room{}).Where("building_id = ? AND status IN ?", b.ID, []string{"rented", "expiring"}).Count(&rentedCount)
		h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", b.ID, "expiring").Count(&expiringCount)
		result = append(result, BuildingVO{
			Building:      b,
			Landlords:     landlords,
			RoomCount:     roomCount,
			VacantCount:   vacantCount,
			RentedCount:   rentedCount,
			ExpiringCount: expiringCount,
		})
	}
	c.JSON(http.StatusOK, gin.H{"buildings": result})
}

func (h *BuildingHandler) ListPublic(c *gin.Context) {
	var buildings []models.Building
	query := h.DB.Where("status = ?", "active").Order("created_at desc")
	if district := c.Query("district"); district != "" {
		query = query.Where("district = ?", district)
	}
	query.Find(&buildings)
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
		h.DB.Where("building_id = ?", b.ID).Find(&landlords)
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
	c.JSON(http.StatusOK, gin.H{"buildings": result})
}

func (h *BuildingHandler) GetPublic(c *gin.Context) {
	id := c.Param("id")
	var building models.Building
	if err := h.DB.First(&building, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "公寓不存在"})
		return
	}
	var landlords []models.BuildingLandlord
	h.DB.Where("building_id = ?", building.ID).Find(&landlords)
	var roomCount, vacantCount, rentedCount, expiringCount int64
	h.DB.Model(&models.Room{}).Where("building_id = ?", building.ID).Count(&roomCount)
	h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", building.ID, "vacant").Count(&vacantCount)
	h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", building.ID, "expiring").Count(&expiringCount)
	h.DB.Model(&models.Room{}).Where("building_id = ? AND status IN ?", building.ID, []string{"rented", "expiring"}).Count(&rentedCount)
	c.JSON(http.StatusOK, gin.H{
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
		c.JSON(http.StatusNotFound, gin.H{"error": "公寓不存在"})
		return
	}
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
	query.Find(&rooms)
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
			h.DB.Where("room_id = ? AND status = ?", r.ID, "active").Select("end_date").First(&contract)
			vo.EndDate = contract.EndDate
		}
		result = append(result, vo)
	}
	c.JSON(http.StatusOK, gin.H{"rooms": result})
}

// 房东管理后台 - 获取当前楼栋信息
func (h *BuildingHandler) MyBuilding(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	var building models.Building
	if err := h.DB.First(&building, bid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "公寓不存在"})
		return
	}
	var landlords []models.BuildingLandlord
	h.DB.Where("building_id = ?", bid).Find(&landlords)
	c.JSON(http.StatusOK, gin.H{
		"building": building,
		"landlords": landlords,
	})
}

// 房东管理后台 - 更新楼栋信息
func (h *BuildingHandler) UpdateMyBuilding(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	var building models.Building
	if err := h.DB.First(&building, bid).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "公寓不存在"})
		return
	}
	var req UpdateBuildingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误"})
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
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
	if len(updates) > 0 {
		h.DB.Model(&building).Updates(updates)
	}
	if req.Landlords != nil {
		h.DB.Where("building_id = ?", bid).Delete(&models.BuildingLandlord{})
		for _, l := range req.Landlords {
			ll := models.BuildingLandlord{
				BuildingID: bid,
				Name:       l.Name,
				Phone:      l.Phone,
			}
			h.DB.Create(&ll)
		}
	}
	c.JSON(http.StatusOK, gin.H{"message": "更新成功"})
}

// 返回公寓列表中城市区域供筛选
func (h *BuildingHandler) Districts(c *gin.Context) {
	type DistrictRow struct {
		District string `json:"district"`
	}
	var rows []DistrictRow
	h.DB.Model(&models.Building{}).Select("DISTINCT district").Where("district != '' AND status = ?", "active").Find(&rows)
	districts := make([]string, 0)
	for _, r := range rows {
		if r.District != "" {
			districts = append(districts, r.District)
		}
	}
	c.JSON(http.StatusOK, gin.H{"districts": districts})
}

// 房东后台首页统计
func (h *BuildingHandler) MyStats(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)

	var roomCount, vacantCount, rentedCount, expiringCount int64
	h.DB.Model(&models.Room{}).Where("building_id = ?", bid).Count(&roomCount)
	h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", bid, "vacant").Count(&vacantCount)
	h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", bid, "rented").Count(&rentedCount)
	h.DB.Model(&models.Room{}).Where("building_id = ? AND status = ?", bid, "expiring").Count(&expiringCount)

	var taskCount int64
	h.DB.Model(&models.Task{}).Where("building_id = ? AND status = ?", bid, "pending").Count(&taskCount)

	month := c.Query("month")
	if month == "" {
		month = utils.Now().Format("2006-01")
	}

	type TotalRow struct {
		Type  string
		Total float64
	}
	var rows []TotalRow
	h.DB.Model(&models.Bill{}).
		Select("type, COALESCE(SUM(amount), 0) as total").
		Where("building_id = ? AND DATE_FORMAT(bill_date, '%Y-%m') = ?", bid, month).
		Group("type").
		Find(&rows)
	var totalIncome, totalExpense float64
	for _, r := range rows {
		if r.Type == "income" {
			totalIncome = r.Total
		} else {
			totalExpense = r.Total
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"building_id":  bid,
		"room_count":   roomCount,
		"vacant_count": vacantCount,
		"rented_count":  rentedCount,
		"expiring_count": expiringCount,
		"task_count":   taskCount,
		"month":        month,
		"total_income":  totalIncome,
		"total_expense": totalExpense,
		"net_profit":    totalIncome - totalExpense,
	})
}
