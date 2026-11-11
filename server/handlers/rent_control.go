// rent_control.go 租控表导出数据接口。
package handlers

import (
	"net/http"

	"rental-server/logger"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
)

// RentControl 返回租控表聚合数据（公寓信息 + 未出租/即将到期房间），供前端生成图片。
// 只读接口，不产生任何数据变更。
func (h *RoomHandler) RentControl(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}

	data, err := h.RoomService.RentControlData(bid)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询租控表数据失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, data)
}
