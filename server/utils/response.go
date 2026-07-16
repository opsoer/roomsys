// 工具包，提供统一的 API 响应格式和分页参数解析
package utils

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// APIResponse 统一 API 响应结构，包含业务码、消息和数据
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// PageResult 分页结果结构
type PageResult struct {
	Items interface{} `json:"items"`
	Total int64       `json:"total"`
	Page  int         `json:"page"`
	Size  int         `json:"size"`
}

// ParsePage 从请求查询参数中解析分页参数（默认 page=1, size=20）
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

// 业务状态码定义
const (
	CodeSuccess            = 0    // 成功
	CodeBadRequest         = 400  // 请求参数错误
	CodeUnauthorized       = 401  // 未授权
	CodeForbidden          = 403  // 无权限
	CodeNotFound           = 404  // 资源不存在
	CodeConflict           = 409  // 冲突
	CodeTooManyRequests    = 429  // 请求频率超限
	CodeInternalError      = 500  // 服务器内部错误
	CodeMonthSettled       = 1001 // 月份已结算
	CodeBuildingExpired    = 1002 // 楼宇套餐过期
	CodeActiveContract     = 1003 // 存在生效合同
	CodeInvalidStatus      = 1004 // 状态无效
	CodeMissingDescription = 1005 // 缺少描述信息
	CodeNameConflict       = 1006 // 名称冲突
)

// Success 返回 200 成功响应
func Success(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{Code: CodeSuccess, Message: "success", Data: data})
}

// SuccessWithMsg 返回带自定义消息的成功响应
func SuccessWithMsg(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusOK, APIResponse{Code: CodeSuccess, Message: msg, Data: data})
}

// Created 返回 201 创建成功响应
func Created(c *gin.Context, msg string, data interface{}) {
	c.JSON(http.StatusCreated, APIResponse{Code: CodeSuccess, Message: msg, Data: data})
}

// Error 返回错误响应，根据 HTTP 状态码映射业务码
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

// ErrorWithCode 返回自定义业务码的错误响应
func ErrorWithCode(c *gin.Context, httpCode, bizCode int, msg string) {
	c.JSON(httpCode, APIResponse{Code: bizCode, Message: msg})
}

// ErrUnauthorized 未授权错误
var ErrUnauthorized = errors.New("未授权")

// ErrBuildingNotFound 未关联公寓错误
var ErrBuildingNotFound = errors.New("未关联公寓")

// ErrInvalidBuildingID 无效的公寓 ID 错误
var ErrInvalidBuildingID = errors.New("无效的公寓ID")

// GetBuildingID 从上下文中获取楼宇 ID
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

// GetUserID 从上下文中获取用户 ID
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
