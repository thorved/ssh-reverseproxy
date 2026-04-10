package routes

import (
	"github.com/gin-gonic/gin"
	"github.com/thorved/ssh-reverseproxy/backend/internal/auth"
	"github.com/thorved/ssh-reverseproxy/backend/internal/config"
	"github.com/thorved/ssh-reverseproxy/backend/internal/handlers"
	"github.com/thorved/ssh-reverseproxy/backend/internal/middleware"
	"github.com/thorved/ssh-reverseproxy/backend/internal/models"
	"gorm.io/gorm"
)

func NewRouter(cfg config.Config, db *gorm.DB, authService *auth.Service) *gin.Engine {
	if cfg.Env != "production" {
		gin.SetMode(gin.DebugMode)
	}

	router := gin.New()
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(middleware.CORS(cfg.FrontendBaseURL))
	router.Use(middleware.Session(authService, cfg.SessionCookieName))

	authHandler := handlers.NewAuthHandler(cfg, authService)
	adminHandler := handlers.NewAdminHandler(db)
	userHandler := handlers.NewUserHandler(db)

	api := router.Group("/api")
	{
		api.GET("/health", authHandler.Health)

		api.GET("/auth/oidc/login", authHandler.Login)
		api.GET("/auth/oidc/callback", authHandler.Callback)
		api.GET("/auth/me", middleware.RequireAuth(), authHandler.Me)
		api.POST("/auth/logout", middleware.RequireAuth(), authHandler.Logout)

		admin := api.Group("/admin")
		admin.Use(middleware.RequireAuth(), middleware.RequireRole(models.RoleAdmin))
		{
			admin.GET("/users", adminHandler.ListUsers)
			admin.POST("/users", adminHandler.CreateUser)
			admin.PATCH("/users/:id", adminHandler.UpdateUser)
			admin.GET("/instances", adminHandler.ListInstances)
			admin.POST("/instances", adminHandler.CreateInstance)
			admin.PATCH("/instances/:id", adminHandler.UpdateInstance)
			admin.POST("/instances/:id/assign", adminHandler.AssignInstance)
		}

		user := api.Group("/user")
		user.Use(middleware.RequireAuth())
		{
			user.GET("/instances", userHandler.ListInstances)
			user.GET("/ssh-keys", userHandler.ListSSHKeys)
			user.POST("/ssh-keys", userHandler.CreateSSHKey)
			user.PATCH("/ssh-keys/:id", userHandler.UpdateSSHKey)
			user.DELETE("/ssh-keys/:id", userHandler.DeleteSSHKey)
		}
	}

	return router
}
