package routes

import (
	"rental-server/config"
	"rental-server/handlers"
	"rental-server/middleware"
	"rental-server/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func Setup(r *gin.Engine, db *gorm.DB, cfg *config.Config) {
	r.Static("/assets", "../web/dist/assets")
	r.StaticFile("/", "../web/dist/index.html")
	r.NoRoute(func(c *gin.Context) {
		c.File("../web/dist/index.html")
	})

	handlers.AutoCheckExpiringContracts(db)

	buildingSvc := services.NewBuildingService(db)
	roomSvc := services.NewRoomService(db)
	billSvc := services.NewBillService(db)
	dividendSvc := services.NewDividendService(db)
	taskSvc := services.NewTaskService(db)
	mediaSvc := services.NewMediaService(db, cfg)
	authSvc := services.NewAuthService(db, cfg)
	settingsSvc := services.NewSettingsService(db)
	recruitSvc := services.NewRecruitService(db)

	// ========== 公开接口 ==========
	public := r.Group("/api")
	{
		auth := &handlers.AuthHandler{DB: db, Cfg: cfg, AuthService: authSvc}
		public.POST("/auth/login", auth.Login)
		public.POST("/auth/refresh", auth.RefreshToken)

		buildingH := &handlers.BuildingHandler{DB: db, BuildingService: buildingSvc}
		public.GET("/buildings", buildingH.ListPublic)
		public.GET("/buildings/districts", buildingH.Districts)
		public.GET("/buildings/:id", buildingH.GetPublic)
		public.GET("/buildings/:id/rooms", buildingH.GetRooms)

		settingsH := &handlers.SettingsHandler{DB: db, SettingsService: settingsSvc}
		public.GET("/settings/recruit", settingsH.GetPublicRecruit)

		roomH := &handlers.RoomHandler{DB: db, Cfg: cfg, RoomService: roomSvc}
		public.GET("/buildings/:id/rooms/:rid", roomH.GetPublic)
		public.GET("/buildings/:id/rooms/:rid/contract", roomH.GetActiveContractPublic)

		recruitH := &handlers.RecruitHandler{DB: db, RecruitService: recruitSvc}
		public.POST("/recruit/submit", recruitH.Submit)

		mediaH := &handlers.MediaHandler{DB: db, Cfg: cfg, MediaService: mediaSvc}
		public.GET("/media/*filepath", mediaH.Serve)

		public.GET("/config", func(c *gin.Context) {
			domain := ""
			scheme := "http"
			if cfg.QiniuUseHTTPS {
				scheme = "https"
			}
			if cfg.QiniuDomain != "" {
				domain = cfg.QiniuDomain
			}
			c.JSON(200, gin.H{"cdn_domain": domain, "cdn_scheme": scheme})
		})
	}

	// ========== 平台超级管理员 ==========
	platform := r.Group("/api/admin")
	platform.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	platform.Use(middleware.SuperAdminMiddleware())
	{
		buildingH := &handlers.BuildingHandler{DB: db, BuildingService: buildingSvc}
		platform.POST("/buildings", buildingH.Create)
		platform.GET("/buildings", buildingH.List)
		platform.PUT("/buildings/:id", buildingH.Update)
		platform.DELETE("/buildings/:id", buildingH.Delete)
		platform.PUT("/buildings/:id/package", buildingH.UpgradePackage)

		authH := &handlers.AuthHandler{DB: db, Cfg: cfg, AuthService: authSvc}
		platform.POST("/auth/create-admin", authH.CreateAdmin)
		platform.POST("/auth/create-building-admin", authH.CreateBuildingAdmin)
		platform.GET("/auth/users", authH.ListUsers)
		platform.PUT("/auth/users/:id", authH.UpdateUser)
		platform.DELETE("/auth/users/:id", authH.DeleteUser)

		systemH := &handlers.SystemHandler{DB: db, SettingsService: settingsSvc}
		platform.GET("/system/time", systemH.GetTime)
		platform.POST("/system/time", systemH.SetTime)

		recruitH := &handlers.RecruitHandler{DB: db, RecruitService: recruitSvc}
		platform.GET("/recruit/list", recruitH.List)
		platform.PUT("/recruit/process/:id", recruitH.Process)
		platform.GET("/recruit/unprocessed-count", recruitH.UnprocessedCount)

		settingsH := &handlers.SettingsHandler{DB: db, SettingsService: settingsSvc}
		platform.GET("/settings/:key", settingsH.Get)
		platform.PUT("/settings/:key", settingsH.Update)
	}

	// ========== 公寓管理后台（building_admin + admin）==========
	building := r.Group("/api/building")
	building.Use(middleware.AuthMiddleware(cfg.JWTSecret))
	building.Use(middleware.AdminOrBuildingAdminMiddleware())
	building.Use(middleware.BuildingScopeMiddleware(db))
	{
		authH := &handlers.AuthHandler{DB: db, Cfg: cfg, AuthService: authSvc}

		// 楼栋信息
		buildingH := &handlers.BuildingHandler{DB: db, BuildingService: buildingSvc}
		building.GET("/info", buildingH.MyBuilding)
		building.PUT("/info", buildingH.UpdateMyBuilding)
		building.GET("/stats", buildingH.MyStats)

		// 房间管理
		roomH := &handlers.RoomHandler{DB: db, Cfg: cfg, RoomService: roomSvc}
		building.GET("/rooms", roomH.List)
		building.GET("/rooms/:id", roomH.Get)
		building.POST("/rooms", roomH.Create)
		building.PUT("/rooms/:id", roomH.Update)
		building.DELETE("/rooms/:id", roomH.Delete)
		building.PUT("/rooms/:id/status", roomH.UpdateStatus)
		building.GET("/rooms/:id/contract", roomH.GetActiveContract)
		building.PUT("/rooms/:id/contract", roomH.RenewContract)

		// 媒体管理
		mediaH := &handlers.MediaHandler{DB: db, Cfg: cfg, MediaService: mediaSvc}
		building.POST("/rooms/:id/media", mediaH.Upload)
		building.DELETE("/rooms/:id/media/:mediaId", mediaH.Delete)
		building.POST("/cover", mediaH.UploadCover)

		// 管理员管理（building_admin 可创建普通 admin）
		building.POST("/auth/create-admin", authH.CreateRegularAdmin)
		building.GET("/auth/users", authH.ListBuildingUsers)

		// 财务管理（全套餐）
		fullPkg := building.Group("")
		fullPkg.Use(middleware.FullPackageMiddleware(db))
		{
			billH := &handlers.BillHandler{DB: db, BillService: billSvc, RoomService: roomSvc}
			fullPkg.GET("/bills", billH.List)
			fullPkg.POST("/bills", billH.Create)
			fullPkg.PUT("/bills/:id", billH.Update)
			fullPkg.DELETE("/bills/:id", billH.Delete)
			fullPkg.GET("/bills/stats", billH.Stats)
			fullPkg.GET("/bills/trend", billH.Trend)
			fullPkg.GET("/bills/export", billH.ExportCSV)

			// 分红管理
			divH := &handlers.DividendHandler{DB: db, DividendService: dividendSvc}
			fullPkg.GET("/dividends", divH.List)
			fullPkg.GET("/dividends/calculate", divH.Calculate)
			fullPkg.POST("/dividends/settle", divH.Settle)
			fullPkg.GET("/dividends/shareholders", divH.GetShareholders)
			fullPkg.POST("/dividends/shareholders", divH.CreateShareholder)
			fullPkg.PUT("/dividends/shareholders/:id", divH.UpdateShareholder)
			fullPkg.DELETE("/dividends/shareholders/:id", divH.DeleteShareholder)
			fullPkg.GET("/dividends/predict", divH.Predict)

			// 待办任务
			taskH := &handlers.TaskHandler{DB: db, TaskService: taskSvc}
			fullPkg.GET("/tasks", taskH.List)
			fullPkg.POST("/tasks/:id/process", taskH.Process)
			fullPkg.PUT("/tasks/:id/complete", taskH.Complete)
			fullPkg.DELETE("/tasks/:id", taskH.Delete)
		}
	}
}
