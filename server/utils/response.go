package utils

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type PageResult struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

func ParsePage(c *gin.Context) (page, size int) {
	page = 1
	size = 20
	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			page = v
		}
	}
	if s := c.Query("page_size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 && v <= 100 {
			size = v
		}
	}
	return
}

const (
	CodeSuccess            = 0
	CodeBadRequest         = 400
	CodeUnauthorized       = 401
	CodeForbidden          = 403
	CodeNotFound           = 404
	CodeConflict           = 409
	CodeTooManyRequests    = 429
	CodeInternalError      = 500
	CodeMonthSettled       = 1001
	CodeBuildingExpired    = 1002
	CodeActiveContract     = 1003
	CodeInvalidStatus      = 1004
	CodeMissingDescription = 1005
)

func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{Code: CodeSuccess, Message: "success", Data: data})
}

func SuccessWithMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{Code: CodeSuccess, Message: msg, Data: data})
}

func Created(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{Code: CodeSuccess, Message: msg, Data: data})
}

func Error(c *gin.Context, httpCode int, msg string) {
	bizCode := httpCode
	switch httpCode {
	case http.StatusBadRequest:
		bizCode = CodeBadRequest
	case http.StatusUnauthorized:
		bizCode = CodeUnauthorized
	case http.StatusForbidden:
		bizCode = CodeForbidden
	case http.StatusNotFound:
		bizCode = CodeNotFound
	case http.StatusConflict:
		bizCode = CodeConflict
	case http.StatusTooManyRequests:
		bizCode = CodeTooManyRequests
	case http.StatusInternalServerError:
		bizCode = CodeInternalError
	}
	c.JSON(httpCode, APIResponse{Code: bizCode, Message: msg})
}

func ErrorWithCode(c *gin.Context, httpCode, bizCode int, msg string) {
	c.JSON(httpCode, APIResponse{Code: bizCode, Message: msg})
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
