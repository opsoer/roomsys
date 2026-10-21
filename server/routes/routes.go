// routes 包负责注册所有 HTTP 路由，按公开、平台管理、公寓管理分组。
package routes

import (
	"path/filepath"

	"rental-server/config"
	"rental-server/handlers"
	"rental-server/middleware"
	"rental-server/scheduler"
	"rental-server/services"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// routeHandlers 汇总各业务处理器，供分组注册函数使用（避免 Setup 变成超长函数）。
type routeHandlers struct {
	auth         *handlers.AuthHandler
	building     *handlers.BuildingHandler
	room         *handlers.RoomHandler
	media        *handlers.MediaHandler
	location     *handlers.LocationManageHandler
	recruit      *handlers.RecruitHandler
	stats        *handlers.StatsHandler
	task         *handlers.TaskHandler
	platformTask *handlers.PlatformTaskHandler
	settings     *handlers.SettingsHandler
	system       *handlers.SystemHandler
	bill         *handlers.BillHandler
	dividend     *handlers.DividendHandler
}

// Setup 注册所有路由，包括静态文件服务、公开接口、平台管理后台和公寓管理后台。
// jobs 为定时任务注册表，供管理后台查看/手动执行。
func Setup(r *gin.Engine, db *gorm.DB, cfg *config.Config, jobs *scheduler.Registry) {
	setupStatic(r, cfg)

	buildingSvc := services.NewBuildingService(db)
	roomSvc := services.NewRoomService(db)
	billSvc := services.NewBillService(db)
	dividendSvc := services.NewDividendService(db)
	taskSvc := services.NewTaskService(db)
	mediaSvc := services.NewMediaService(db, cfg)
	authSvc := services.NewAuthService(db, cfg)
	settingsSvc := services.NewSettingsService(db)
	recruitSvc := services.NewRecruitService(db)
	statsSvc := services.NewStatsService(db)

	h := &routeHandlers{
		auth:         &handlers.AuthHandler{DB: db, Cfg: cfg, AuthService: authSvc},
		building:     &handlers.BuildingHandler{DB: db, Cfg: cfg, BuildingService: buildingSvc},
		room:         &handlers.RoomHandler{DB: db, Cfg: cfg, RoomService: roomSvc},
		media:        &handlers.MediaHandler{DB: db, Cfg: cfg, MediaService: mediaSvc},
		location:     &handlers.LocationManageHandler{DB: db},
		recruit:      &handlers.RecruitHandler{DB: db, RecruitService: recruitSvc, TaskService: taskSvc},
		stats:        &handlers.StatsHandler{DB: db, StatsService: statsSvc},
		task:         &handlers.TaskHandler{DB: db, TaskService: taskSvc},
		platformTask: &handlers.PlatformTaskHandler{DB: db, TaskService: taskSvc, RecruitService: recruitSvc},
		settings:     &handlers.SettingsHandler{DB: db, SettingsService: settingsSvc},
		system:       &handlers.SystemHandler{DB: db, SettingsService: settingsSvc, Jobs: jobs},
		bill:         &handlers.BillHandler{DB: db, BillService: billSvc, RoomService: roomSvc},
		dividend:     &handlers.DividendHandler{DB: db, DividendService: dividendSvc},
	}

	setupPublicRoutes(r, cfg, h)
	setupPlatformRoutes(r, db, cfg, h)
	setupBuildingRoutes(r, db, cfg, h)
}

// setupStatic 注册静态资源与 SPA 兜底路由。
func setupStatic(r *gin.Engine, cfg *config.Config) {
	r.Static("/assets", cfg.Server.WebDistDir+"/assets")
	r.StaticFile("/default-image.svg", cfg.Server.WebDistDir+"/default-image.svg")
	r.StaticFile("/", cfg.Server.WebDistDir+"/index.html")
	r.Static("/api/ffmpeg", filepath.Join(cfg.Server.UploadDir, "ffmpeg"))
	r.NoRoute(func(c *gin.Context) {
		c.File(cfg.Server.WebDistDir + "/index.html")
	})
}

// setupPublicRoutes 注册公开接口（无需登录）。
func setupPublicRoutes(r *gin.Engine, cfg *config.Config, h *routeHandlers) {
	public := r.Group("/api")
	{
		public.POST("/auth/login", h.auth.Login)
		public.POST("/auth/refresh", h.auth.RefreshToken)
		public.POST("/auth/logout", h.auth.Logout)

		public.GET("/buildings", h.building.ListPublic)
		public.GET("/buildings/locations", h.building.ListLocations)
		public.GET("/locations/options", h.location.ListOptions)
		public.GET("/buildings/:id", h.building.GetPublic)
		public.GET("/buildings/:id/rooms", h.building.GetRooms)

		public.GET("/buildings/:id/rooms/:rid", h.room.GetPublic)
		public.GET("/captcha", handlers.GetCaptcha)
		public.POST("/buildings/:id/reveal-phone", h.building.RevealPhone)

		public.POST("/recruit/submit", h.recruit.Submit)

		public.GET("/media/*filepath", h.media.Serve)
		public.POST("/qiniu/pfop-callback", h.media.PfopCallback)

		public.POST("/stats/landlord-view", h.stats.RecordLandlordView)

		public.GET("/config", func(c *gin.Context) {
			domain := ""
			scheme := "http"
			if cfg.Storage.Qiniu.UseHTTPS {
				scheme = "https"
			}
			if cfg.Storage.Qiniu.Domain != "" {
				domain = cfg.Storage.Qiniu.Domain
			}
			c.JSON(200, gin.H{"cdn_domain": domain, "cdn_scheme": scheme})
		})
	}
}

// setupPlatformRoutes 注册平台超级管理员接口。
func setupPlatformRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config, h *routeHandlers) {
	platform := r.Group("/api/admin")
	platform.Use(middleware.AuthMiddleware(db, cfg.Server.JWTSecret))
	platform.Use(middleware.SuperAdminMiddleware())
	{
		platform.POST("/buildings", h.building.Create)
		platform.GET("/buildings", h.building.List)
		platform.PUT("/buildings/:id", h.building.Update)
		platform.DELETE("/buildings/:id", h.building.Delete)
		platform.PUT("/buildings/:id/package", h.building.UpgradePackage)
		platform.POST("/buildings/:id/renew", h.building.Renew)
		platform.GET("/buildings/:id/renewals", h.building.Renewals)
		platform.PUT("/buildings/:id/deposit", h.building.AdjustDeposit)
		platform.GET("/buildings/:id/deposit-logs", h.building.DepositLogs)

		platform.POST("/auth/create-admin", h.auth.CreateAdmin)
		platform.POST("/auth/create-building-admin", h.auth.CreateBuildingAdmin)
		platform.GET("/auth/users", h.auth.ListUsers)
		platform.PUT("/auth/users/:id", h.auth.UpdateUser)
		platform.DELETE("/auth/users/:id", h.auth.DeleteUser)

		platform.GET("/system/time", h.system.GetTime)
		platform.POST("/system/time", h.system.SetTime)
		platform.POST("/system/run-tasks", h.system.RunTasks)
		platform.GET("/system/jobs", h.system.ListJobs)
		platform.POST("/system/jobs/:name/run", h.system.RunJob)
		platform.GET("/system/audit-logs", h.system.ListAuditLogs)

		platform.GET("/recruit/list", h.recruit.List)
		platform.PUT("/recruit/process/:id", h.recruit.Process)

		platform.GET("/tasks", h.platformTask.List)
		platform.GET("/tasks/count", h.platformTask.Count)
		platform.POST("/tasks/:id/process", h.platformTask.Process)

		platform.GET("/settings/:key", h.settings.Get)
		platform.PUT("/settings/:key", h.settings.Update)

		platform.GET("/stats/overview", h.stats.Overview)
		platform.GET("/stats/building/:id", h.stats.BuildingDetail)
		platform.GET("/stats/trend", h.stats.Trend)
		platform.GET("/stats/price-reference", h.stats.PriceReference)

		platform.GET("/locations/manage", h.location.List)
		platform.POST("/locations/custom", h.location.AddCustom)
		platform.DELETE("/locations/custom/:id", h.location.DeleteCustom)
		platform.POST("/locations/rename", h.location.Rename)
	}
}

// setupBuildingRoutes 注册公寓管理后台接口（building_admin + admin）。
func setupBuildingRoutes(r *gin.Engine, db *gorm.DB, cfg *config.Config, h *routeHandlers) {
	building := r.Group("/api/building")
	building.Use(middleware.AuthMiddleware(db, cfg.Server.JWTSecret))
	building.Use(middleware.AdminOrBuildingAdminMiddleware())
	building.Use(middleware.BuildingScopeMiddleware(db))
	{
		// 楼栋信息
		building.GET("/info", h.building.MyBuilding)
		building.PUT("/info", h.building.UpdateMyBuilding)
		building.GET("/stats", h.building.MyStats)

		// 房间管理
		building.GET("/rooms", h.room.List)
		building.GET("/rooms/:id", h.room.Get)
		building.POST("/rooms", h.room.Create)
		building.POST("/rooms/import-rented", h.room.ImportRented)
		building.POST("/rooms/batch-pricing", h.room.BatchUpdatePricing)
		building.PUT("/rooms/:id", h.room.Update)
		building.DELETE("/rooms/:id", h.room.Delete)
		building.PUT("/rooms/:id/status", h.room.UpdateStatus)
		building.GET("/contracts", h.room.ListBuildingContracts)
		building.GET("/rooms/:id/contract", h.room.GetActiveContract)
		building.PUT("/rooms/:id/contract", h.room.RenewContract)

		// 媒体管理
		building.POST("/rooms/:id/media", h.media.Upload)
		building.DELETE("/rooms/:id/media/:mediaId", h.media.Delete)
		building.POST("/rooms/:id/media/copy", h.media.CopyMedia)
		building.POST("/cover", h.media.UploadCover)
		building.GET("/media/upload-token", h.media.GetUploadToken)
		building.POST("/rooms/:id/media/confirm", h.media.ConfirmUpload)
		building.POST("/rooms/:id/media/check-transcode", h.media.CheckTranscodeStatus)
		building.POST("/ffmpeg/re-download", h.media.ReDownloadFFmpeg)

		// 管理员管理（building_admin 可创建普通 admin）
		building.POST("/auth/create-admin", h.auth.CreateRegularAdmin)
		building.GET("/auth/users", h.auth.ListBuildingUsers)

		// 待办任务（不限制套餐：到期提醒/预订超时等待办由系统对所有公寓生成，基础套餐也需可见可处理）
		building.GET("/tasks", h.task.List)
		building.POST("/tasks/:id/process", h.task.Process)
		building.PUT("/tasks/:id/complete", h.task.Complete)
		building.DELETE("/tasks/:id", h.task.Delete)

		// 数据统计
		building.GET("/stats/pv", h.stats.MyBuildingStats)
		building.GET("/stats/trend", h.stats.MyBuildingTrend)

		// 财务管理（全套餐）
		fullPkg := building.Group("")
		fullPkg.Use(middleware.FullPackageMiddleware(db))
		{
			fullPkg.GET("/bills", h.bill.List)
			fullPkg.POST("/bills", h.bill.Create)
			fullPkg.PUT("/bills/:id", h.bill.Update)
			fullPkg.DELETE("/bills/:id", h.bill.Delete)
			fullPkg.GET("/bills/stats", h.bill.Stats)
			fullPkg.GET("/bills/trend", h.bill.Trend)
			fullPkg.GET("/bills/export", h.bill.ExportCSV)

			// 分红管理
			fullPkg.GET("/dividends", h.dividend.List)
			fullPkg.GET("/dividends/calculate", h.dividend.Calculate)
			fullPkg.POST("/dividends/settle", h.dividend.Settle)
			fullPkg.GET("/dividends/shareholders", h.dividend.GetShareholders)
			fullPkg.POST("/dividends/shareholders", h.dividend.CreateShareholder)
			fullPkg.PUT("/dividends/shareholders/:id", h.dividend.UpdateShareholder)
			fullPkg.DELETE("/dividends/shareholders/:id", h.dividend.DeleteShareholder)
			fullPkg.GET("/dividends/predict", h.dividend.Predict)
		}
	}
}
