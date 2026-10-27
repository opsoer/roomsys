package tests

import (
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"

	"rental-server/logger"
	"rental-server/utils"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	logger.Log = zerolog.Nop()
	utils.InitRevealQuota(5)
	utils.SetTimeOffset(0)
	os.Exit(m.Run())
}
