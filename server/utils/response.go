package utils

import (
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
