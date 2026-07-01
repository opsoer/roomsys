package utils

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "success", Data: data})
}

func SuccessWithMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: msg, Data: data})
}

func Created(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{Code: 0, Message: msg, Data: data})
}

func Error(c *gin.Context, httpCode int, msg string) {
	c.JSON(httpCode, APIResponse{Code: httpCode, Message: msg})
}

var ErrUnauthorized = errors.New("未授权")
var ErrBuildingNotFound = errors.New("未关联公寓")
var ErrInvalidBuildingID = errors.New("无效的公寓ID")

func GetBuildingID(c *gin.Context) (uint, error) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		return 0, ErrUnauthorized
	}
	bid, ok := buildingID.(uint)
	if !ok || bid == 0 {
		return 0, ErrBuildingNotFound
	}
	return bid, nil
}

func GetUserID(c *gin.Context) (uint, error) {
	userID, exists := c.Get("user_id")
	if !exists {
		return 0, ErrUnauthorized
	}
	uid, ok := userID.(uint)
	if !ok {
		return 0, errors.New("无效的用户ID")
	}
	return uid, nil
}
