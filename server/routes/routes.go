package routes

import (
	"rental-server/config"
	"rental-server/handlers"
	"rental-server/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(r *gin.Engine, db *gorm.DB, cfg *config.Config) {
	handlers.AutoCheckExpiringContracts(db)

	// ========== 公开接口 ==========
	public := r.Group("/api")
	{
		auth := &handlers.AuthHandler{DB: db, Cfg: cfg}
		public.POST("/auth/login", auth.Login)
		public.POST("/auth/register", auth.Register)

		buildingH := &handlers.BuildingHandler{DB: db}
		public.GET("/buildings", buildingH.ListPublic)
		public.GET("/buildings/districts", buildingH.Districts)
		public.GET("/buildings/:id", buildingH.GetPublic)
		public.GET("/buildings/:id/rooms", buildingH.GetRooms)

		roomH := &handlers.RoomHandler{DB: db}
		public.GET("/buildings/:id/rooms/:rid", roomH.GetPublic)
		public.GET("/buildings/:id/rooms/:rid/contract", roomH.GetActiveContractPublic)

		mediaH := &handlers.MediaHandler{DB: db, Cfg: cfg}
		public.GET("/media/*filepath", mediaH.Serve)
	}

	// ========== 平台超级管理员 ==========
	platform := r.Group("/api/admin")
	platform.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	platform.Use(middleware.SuperAdminMiddleware())
	{
		buildingH := &handlers.BuildingHandler{DB: db}
		platform.POST("/buildings", buildingH.Create)
		platform.GET("/buildings", buildingH.List)
		platform.PUT("/buildings/:id", buildingH.Update)
		platform.DELETE("/buildings/:id", buildingH.Delete)

		authH := &handlers.AuthHandler{DB: db, Cfg: cfg}
		platform.POST("/auth/create-admin", authH.CreateAdmin)
		platform.POST("/auth/create-building-admin", authH.CreateBuildingAdmin)
		platform.GET("/auth/users", authH.ListUsers)
		platform.PUT("/auth/users/:id", authH.UpdateUser)
		platform.DELETE("/auth/users/:id", authH.DeleteUser)
	}

	// ========== 公寓管理后台（building_admin + admin）==========
	building := r.Group("/api/building")
	building.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	building.Use(middleware.AdminOrBuildingAdminMiddleware())
	building.Use(middleware.BuildingScopeMiddleware(db))
	{
		authH := &handlers.AuthHandler{DB: db, Cfg: cfg}

		// 楼栋信息
		buildingH := &handlers.BuildingHandler{DB: db}
		building.GET("/info", buildingH.MyBuilding)
		building.PUT("/info", buildingH.UpdateMyBuilding)
		building.GET("/stats", buildingH.MyStats)

		// 房间管理
		roomH := &handlers.RoomHandler{DB: db}
		building.GET("/rooms", roomH.List)
		building.POST("/rooms", roomH.Create)
		building.GET("/rooms/:id", roomH.Get)
		building.PUT("/rooms/:id", roomH.Update)
		building.DELETE("/rooms/:id", roomH.Delete)
		building.PUT("/rooms/:id/status", roomH.UpdateStatus)
		building.PUT("/rooms/:id/contract", roomH.UpdateContractEndDate)
		building.GET("/rooms/:id/contract", roomH.GetActiveContract)

		// 媒体管理
		mediaH := &handlers.MediaHandler{DB: db, Cfg: cfg}
		building.POST("/rooms/:id/media", mediaH.Upload)
		building.DELETE("/rooms/:id/media/:mediaId", mediaH.Delete)

		// 管理员管理（building_admin 可创建普通 admin）
		building.POST("/auth/create-admin", authH.CreateRegularAdmin)
		building.GET("/auth/users", authH.ListBuildingUsers)

		// 财务管理
		billH := &handlers.BillHandler{DB: db}
		building.GET("/bills", billH.List)
		building.POST("/bills", billH.Create)
		building.PUT("/bills/:id", billH.Update)
		building.DELETE("/bills/:id", billH.Delete)
		building.GET("/bills/stats", billH.Stats)
		building.GET("/bills/trend", billH.Trend)

		// 分红管理
		divH := &handlers.DividendHandler{DB: db}
		building.GET("/dividends", divH.List)
		building.GET("/dividends/calculate", divH.Calculate)
		building.POST("/dividends/settle", divH.Settle)
		building.GET("/dividends/shareholders", divH.GetShareholders)
		building.POST("/dividends/shareholders", divH.CreateShareholder)
		building.PUT("/dividends/shareholders/:id", divH.UpdateShareholder)
		building.DELETE("/dividends/shareholders/:id", divH.DeleteShareholder)
		building.GET("/dividends/predict", divH.Predict)

		// 待办任务
		taskH := &handlers.TaskHandler{DB: db}
		building.GET("/tasks", taskH.List)
		building.POST("/tasks/:id/process", taskH.Process)
		building.PUT("/tasks/:id/complete", taskH.Complete)
		building.DELETE("/tasks/:id", taskH.Delete)

		// 系统时间模拟
		systemH := &handlers.SystemHandler{DB: db}
		building.GET("/system/time", systemH.GetTime)
		building.POST("/system/time", systemH.SetTime)
	}
}
