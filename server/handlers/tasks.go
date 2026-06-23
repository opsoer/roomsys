package handlers

import (
	"net/http"

	"rental-server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TaskHandler struct {
	DB *gorm.DB
}

type ProcessTaskReq struct {
	RefundedDeposit float64 `json:"refunded_deposit"`
}

func (h *TaskHandler) List(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	var tasks []models.Task
	query := h.DB.Preload("Room").Where("building_id = ?", bid).Order("created_at desc")
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}
	query.Find(&tasks)
	c.JSON(http.StatusOK, gin.H{"tasks": tasks})
}

func (h *TaskHandler) Process(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	id := c.Param("id")
	var task models.Task
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	if task.Type != "expired_room" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "该任务不支持此操作"})
		return
	}
	if task.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "任务已处理"})
		return
	}

	var req ProcessTaskReq
	if err := c.ShouldBindJSON(&req); err != nil || req.RefundedDeposit < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请填写退还押金金额"})
		return
	}

	var room models.Room
	if task.RoomID == nil || h.DB.Where("id = ? AND building_id = ?", *task.RoomID, bid).First(&room).Error != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "关联房间不存在"})
		return
	}

	userID, _ := c.Get("user_id")
	handleDepositRefund(h.DB, room, req.RefundedDeposit, userID.(uint), bid)

	h.DB.Model(&room).Where("status != ?", "vacant").Update("status", "vacant")
	h.DB.Model(&task).Update("status", "completed")
	c.JSON(http.StatusOK, gin.H{"message": "退租处理完成"})
}

func (h *TaskHandler) Complete(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	id := c.Param("id")
	var task models.Task
	if err := h.DB.Where("id = ? AND building_id = ?", id, bid).First(&task).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
		return
	}
	h.DB.Model(&task).Update("status", "completed")
	c.JSON(http.StatusOK, gin.H{"message": "任务已完成"})
}

func (h *TaskHandler) Delete(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	id := c.Param("id")
	h.DB.Where("id = ? AND building_id = ?", id, bid).Delete(&models.Task{})
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}
