// 统一错误体系：业务错误（BizError）+ 单一出口（Fail）。
//
// 设计目标：
//   - 业务错误自带 HTTP 状态码与业务码，handler 只做翻译，不再各写各的；
//   - 内部错误（数据库、IO 等）对外只返回通用文案，细节只进日志，避免泄露表名/字段/SQL；
//   - 用 errors.Is 判断错误类型，取代脆弱的 err.Error() 字符串比较。
package utils

import (
	"errors"
	"fmt"
	"net/http"

	"rental-server/logger"

	"github.com/gin-gonic/gin"
)

// BizError 业务错误：携带 HTTP 状态码、业务码与面向用户的文案。
type BizError struct {
	// HTTPStatus HTTP 状态码。
	HTTPStatus int
	// Code 业务码（0 表示沿用 HTTP 状态码映射）。
	Code int
	// Message 面向用户的文案。
	Message string
	// cause 内部原因，只写日志、不返回前端。
	cause error
}

// Error 实现 error 接口。
func (e *BizError) Error() string {
	if e.Message == "" && e.cause != nil {
		return e.cause.Error()
	}
	return e.Message
}

// Unwrap 返回内部原因，支持 errors.Is/As 链式判断。
func (e *BizError) Unwrap() error { return e.cause }

// WithCause 附加内部原因（用于记录细节，不影响对外文案）。
func (e *BizError) WithCause(err error) *BizError {
	e.cause = err
	return e
}

// NewBizError 构造业务错误。
func NewBizError(httpStatus, code int, msg string) *BizError {
	return &BizError{HTTPStatus: httpStatus, Code: code, Message: msg}
}

// BadRequest 400 参数错误。
func BadRequest(format string, args ...interface{}) *BizError {
	return NewBizError(http.StatusBadRequest, CodeBadRequest, fmt.Sprintf(format, args...))
}

// Unauthorized 401 未授权。
func Unauthorized(msg string) *BizError {
	return NewBizError(http.StatusUnauthorized, CodeUnauthorized, msg)
}

// Forbidden 403 无权限。
func Forbidden(msg string) *BizError {
	return NewBizError(http.StatusForbidden, CodeForbidden, msg)
}

// NotFound 404 资源不存在。
func NotFound(msg string) *BizError {
	return NewBizError(http.StatusNotFound, CodeNotFound, msg)
}

// Conflict 409 冲突。
func Conflict(msg string) *BizError {
	return NewBizError(http.StatusConflict, CodeConflict, msg)
}

// Internal 500 内部错误：对外使用通用文案，真实原因通过 WithCause 记录。
func Internal(cause error, msg string) *BizError {
	return NewBizError(http.StatusInternalServerError, CodeInternalError, msg).WithCause(cause)
}

// WithCode 给业务错误附加自定义业务码。
func WithCode(err error, code int) *BizError {
	var be *BizError
	if errors.As(err, &be) {
		be.Code = code
		return be
	}
	return NewBizError(http.StatusBadRequest, code, err.Error()).WithCause(err)
}

// Fail 统一错误出口：把 error 翻译成响应。
//   - *BizError：按其 HTTP 状态码与业务码返回，文案直接给用户；
//   - 其他错误：记 Error 日志（含细节），对外返回 500 通用文案，不泄露内部信息。
func Fail(c *gin.Context, err error) {
	if err == nil {
		return
	}
	var be *BizError
	if errors.As(err, &be) {
		code := be.Code
		if code == 0 {
			code = httpCodeToBizCode(be.HTTPStatus)
		}
		if be.cause != nil {
			logger.Log.Error().Err(be.cause).Str("code", fmt.Sprint(code)).Msg("业务失败: " + be.Message)
		}
		c.JSON(be.HTTPStatus, APIResponse{Code: code, Message: be.Message})
		return
	}
	logger.Log.Error().Err(err).Str("path", c.Request.URL.Path).Msg("请求处理失败")
	c.JSON(http.StatusInternalServerError, APIResponse{
		Code:    CodeInternalError,
		Message: "服务器内部错误，请稍后重试",
	})
}

// httpCodeToBizCode 把 HTTP 状态码映射为业务码。
func httpCodeToBizCode(httpCode int) int {
	switch httpCode {
	case http.StatusBadRequest:
		return CodeBadRequest
	case http.StatusUnauthorized:
		return CodeUnauthorized
	case http.StatusForbidden:
		return CodeForbidden
	case http.StatusNotFound:
		return CodeNotFound
	case http.StatusConflict:
		return CodeConflict
	case http.StatusTooManyRequests:
		return CodeTooManyRequests
	case http.StatusInternalServerError:
		return CodeInternalError
	default:
		return httpCode
	}
}
