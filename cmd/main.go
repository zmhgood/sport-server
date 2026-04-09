package main

import (
	"fmt"
	"log"

	"elderly-fitness/config"
	"elderly-fitness/internal/handler"
	"elderly-fitness/internal/middleware"
	"elderly-fitness/internal/model"
	"elderly-fitness/internal/repository"
	"elderly-fitness/internal/service"
	"elderly-fitness/pkg/database"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func main() {
	// 加载配置
	if err := config.LoadConfig("./config/config.yaml"); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	// 初始化数据库
	if err := database.InitDB(&config.AppConfig.Database); err != nil {
		log.Fatalf("初始化数据库失败: %v", err)
	}
	defer database.Close()

	// 自动迁移数据库表
	database.DB.AutoMigrate(
		&model.Admin{},
		&model.User{},
		&model.MuscleGroup{},
		&model.Exercise{},
		&model.ExerciseStep{},
		&model.UserExerciseRecord{},
		&model.UserDailyStats{},
		&model.Comment{},
		&model.CommentLike{},
		// 家庭功能相关表
		&model.Family{},
		&model.FamilyMember{},
		&model.DailyGoal{},
		&model.GoalMember{},
		&model.GoalExercise{},
		&model.GoalCompletion{},
		// 系统配置
		&model.SystemConfig{},
	)

	// 初始化默认管理员
	initDefaultAdmin(database.DB)

	// 初始化Repository
	userRepo := repository.NewUserRepository(database.DB)
	exerciseRepo := repository.NewExerciseRepository(database.DB)
	smsCodeRepo := repository.NewSMSCodeRepository(database.DB)
	commentRepo := repository.NewCommentRepository(database.DB)
	adminRepo := repository.NewAdminRepository(database.DB)
	familyRepo := repository.NewFamilyRepository(database.DB)
	goalRepo := repository.NewGoalRepository(database.DB)
	systemRepo := repository.NewSystemRepository(database.DB)

	// 初始化Service
	smsService := service.NewSMSService(smsCodeRepo)
	authService := service.NewAuthService(userRepo, smsService)
	exerciseService := service.NewExerciseService(exerciseRepo, userRepo)
	userService := service.NewUserService(userRepo, exerciseRepo)
	userService.SetCommentRepo(commentRepo)
	commentService := service.NewCommentService(commentRepo)
	adminService := service.NewAdminService(adminRepo, userRepo)
	familyService := service.NewFamilyService(familyRepo)
	goalService := service.NewGoalService(goalRepo, familyRepo, exerciseRepo)
	systemService := service.NewSystemService(systemRepo)

	// 初始化Handler
	authHandler := handler.NewAuthHandler(authService, smsService)
	exerciseHandler := handler.NewExerciseHandler(exerciseService)
	userHandler := handler.NewUserHandler(userService)
	commentHandler := handler.NewCommentHandler(commentService)
	adminHandler := handler.NewAdminHandler(adminService, userService)
	uploadHandler := handler.NewUploadHandler()
	familyHandler := handler.NewFamilyHandler(familyService)
	goalHandler := handler.NewGoalHandler(goalService)
	systemHandler := handler.NewSystemHandler(systemService)

	// 创建路由
	r := gin.Default()

	// 使用中间件
	r.Use(middleware.CORS())

	// API路由组
	api := r.Group("/api")
	{
		// 公开路由 - 认证
		api.POST("/auth/login", authHandler.Login)           // 微信登录
		api.POST("/auth/sms-login", authHandler.SMSLogin)    // 短信登录
		api.POST("/auth/send-code", authHandler.SendSMSCode) // 发送验证码

		// 管理员登录
		api.POST("/admin/login", adminHandler.AdminLogin)

		// 锻炼相关 - 公开
		api.GET("/muscle-groups", exerciseHandler.GetMuscleGroups)
		api.GET("/exercises", exerciseHandler.GetExercises)
		api.GET("/exercises/:id", exerciseHandler.GetExerciseDetail)

		// 评论相关 - 公开（获取评论列表）
		api.GET("/comments", commentHandler.GetComments)

		// 系统配置 - 公开
		api.GET("/configs", systemHandler.GetConfigs)

		// 需要认证的路由
		authGroup := api.Group("")
		authGroup.Use(middleware.AuthMiddleware())
		{
			// 文件上传
			authGroup.POST("/upload/image", uploadHandler.UploadImage)
			authGroup.POST("/upload/video", uploadHandler.UploadVideo)

			// 用户信息
			authGroup.GET("/user/info", authHandler.GetUserInfo)
			authGroup.PUT("/user/info", authHandler.UpdateUserInfo)
			authGroup.GET("/user/statistics", userHandler.GetStatistics)
			authGroup.GET("/user/history", userHandler.GetHistory)
			authGroup.GET("/user/today-progress", userHandler.GetTodayProgress)
			authGroup.GET("/user/week-stats", userHandler.GetWeekStats)
			authGroup.GET("/user/comments", commentHandler.GetUserComments)

			// 锻炼相关
			authGroup.GET("/exercises/recommend", exerciseHandler.GetRecommendExercises)
			authGroup.POST("/exercises/record", exerciseHandler.RecordExercise)

			// 评论相关 - 需要认证
			authGroup.POST("/comments", commentHandler.CreateComment)
			authGroup.DELETE("/comments/:id", commentHandler.DeleteComment)
			authGroup.POST("/comments/:id/like", commentHandler.ToggleLike)

			// 家庭相关
			authGroup.POST("/family", familyHandler.CreateFamily)              // 创建家庭
			authGroup.POST("/family/join", familyHandler.JoinFamily)           // 加入家庭
			authGroup.GET("/families", familyHandler.GetMyFamilies)            // 获取我的所有家庭
			authGroup.GET("/family/:id", familyHandler.GetFamily)              // 获取指定家庭信息
			authGroup.GET("/family/members", familyHandler.GetFamilyMembers)   // 获取家庭成员
			authGroup.POST("/family/leave", familyHandler.LeaveFamily)         // 退出家庭
			authGroup.POST("/family/remove", familyHandler.RemoveMember)       // 移除成员
			authGroup.POST("/family/transfer", familyHandler.TransferAdmin)    // 转移管理员
			authGroup.POST("/family/invite-code/refresh", familyHandler.RefreshInviteCode) // 刷新邀请码

			// 目标相关 - 注意：固定路径必须在参数路径之前
			authGroup.POST("/goals", goalHandler.CreateGoal)                   // 创建目标
			authGroup.GET("/goals", goalHandler.GetMyGoals)                    // 获取我的目标
			authGroup.GET("/goals/families", goalHandler.GetUserFamiliesWithGoals) // 获取用户所有家庭及目标（必须在 :id 之前）
			authGroup.GET("/goals/family", goalHandler.GetFamilyGoals)         // 获取家庭目标（必须在 :id 之前）
			authGroup.GET("/goals/:id/progress", goalHandler.GetGoalProgress)  // 获取目标进度（必须在 :id 之前）
			authGroup.GET("/goals/:id/history", goalHandler.GetGoalHistory)    // 获取历史记录（必须在 :id 之前）
			authGroup.POST("/goals/:id/members", goalHandler.AddGoalMember)    // 添加目标成员
			authGroup.POST("/goals/:id/exercises", goalHandler.AddGoalExercise) // 添加目标动作
			authGroup.POST("/goals/:id/complete", goalHandler.CompleteExercise) // 完成动作
			authGroup.GET("/goals/:id", goalHandler.GetGoal)                   // 获取目标详情（必须在所有 /goals/:id/xxx 之后）
			authGroup.PUT("/goals/:id", goalHandler.UpdateGoal)                // 更新目标
			authGroup.DELETE("/goals/:id", goalHandler.DeleteGoal)             // 删除目标
		}

		// 管理后台路由 - 需要管理员认证
		adminGroup := api.Group("/admin")
		adminGroup.Use(middleware.AuthMiddleware())
		{
			adminGroup.GET("/info", adminHandler.GetAdminInfo)
			adminGroup.GET("/users", adminHandler.GetUsers)
			adminGroup.GET("/users/:id/stats", adminHandler.GetUserStats)
			adminGroup.GET("/comments", adminHandler.GetComments)
			adminGroup.PUT("/comments/:id/status", adminHandler.UpdateCommentStatus)
			adminGroup.DELETE("/comments/:id", adminHandler.DeleteComment)
			// 锻炼动作管理
			adminGroup.POST("/exercises", exerciseHandler.CreateExercise)
			adminGroup.PUT("/exercises/:id", exerciseHandler.UpdateExercise)
			adminGroup.DELETE("/exercises/:id", exerciseHandler.DeleteExercise)
			adminGroup.GET("/stats/dashboard", adminHandler.GetDashboardStats)
			// 家庭管理
			adminGroup.GET("/families", familyHandler.ListFamiliesForAdmin)
			adminGroup.GET("/families/:id", familyHandler.GetFamily)
			// 系统配置
			adminGroup.GET("/configs", systemHandler.GetConfigs)
			adminGroup.POST("/configs", systemHandler.UpdateConfigs)
		}
	}

	// 健康检查
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "ok",
		})
	})

	// 启动服务
	addr := fmt.Sprintf(":%d", config.AppConfig.Server.Port)
	log.Printf("服务启动在 %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("启动服务失败: %v", err)
	}
}

// initDefaultAdmin 初始化默认管理员账号
func initDefaultAdmin(db *gorm.DB) {
	var count int64
	db.Model(&model.Admin{}).Count(&count)
	if count == 0 {
		adminService := service.NewAdminService(
			repository.NewAdminRepository(db),
			repository.NewUserRepository(db),
		)
		if err := adminService.CreateAdmin("admin", "admin123", "管理员"); err != nil {
			log.Printf("创建默认管理员失败: %v", err)
		} else {
			log.Println("默认管理员账号创建成功: admin / admin123")
		}
	}
}
