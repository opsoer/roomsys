package handlers

import (
	"net/http"
	"time"

	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type SystemHandler struct {
	DB *gorm.DB
}

type SetTimeReq struct {
	OffsetSeconds int64 `json:"offset_seconds" binding:"required"`
}

type TimeResp struct {
	SimulatedTime string `json:"simulated_time"`
	OffsetSeconds int64  `json:"offset_seconds"`
}

func (h *SystemHandler) GetTime(c *gin.Context) {
	now := utils.Now()
	offset := utils.GetTimeOffset()
	c.JSON(http.StatusOK, TimeResp{
		SimulatedTime: now.Format(time.RFC3339),
		OffsetSeconds: int64(offset.Seconds()),
	})
}

func (h *SystemHandler) SetTime(c *gin.Context) {
	var req SetTimeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	utils.SetTimeOffset(time.Duration(req.OffsetSeconds) * time.Second)
	AutoCheckExpiringContracts(h.DB)
	now := utils.Now()
	offset := utils.GetTimeOffset()
	c.JSON(http.StatusOK, TimeResp{
		SimulatedTime: now.Format(time.RFC3339),
		OffsetSeconds: int64(offset.Seconds()),
	})
}
