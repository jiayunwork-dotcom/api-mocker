package main

import (
	"fmt"
	"log"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"api-mocker/cache"
	"api-mocker/config"
	"api-mocker/database"
	"api-mocker/handlers"
	"api-mocker/middleware"
)

func main() {
	cfg := config.Load()

	db := database.Connect(cfg)
	defer db.Close()

	rdb := cache.Connect(cfg)

	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
	}))

	authMiddleware := middleware.AuthMiddleware(cfg)

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	h := handlers.New(db, rdb, cfg)

	api := r.Group("/api")
	{
		auth := api.Group("")
		auth.Use(authMiddleware)
		{
			workspaces := auth.Group("/workspaces")
			{
				workspaces.GET("", h.ListWorkspaces)
				workspaces.POST("", h.CreateWorkspace)
				workspaces.GET("/:id", h.GetWorkspace)
				workspaces.PUT("/:id", h.UpdateWorkspace)
				workspaces.DELETE("/:id", h.DeleteWorkspace)

				workspaces.GET("/:id/members", h.ListMembers)
				workspaces.POST("/:id/members/invite", h.InviteMember)
				workspaces.POST("/join", h.JoinWorkspace)
				workspaces.PUT("/:id/members/:memberId", h.UpdateMemberRole)
				workspaces.DELETE("/:id/members/:memberId", h.RemoveMember)
			}

			projects := auth.Group("/workspaces/:workspaceId/projects")
			{
				projects.GET("", h.ListProjects)
				projects.POST("", h.CreateProject)
				projects.GET("/:id", h.GetProject)
				projects.PUT("/:id", h.UpdateProject)
				projects.DELETE("/:id", h.DeleteProject)
			}

			models := auth.Group("/projects/:projectId/models")
			{
				models.GET("", h.ListModels)
				models.POST("", h.CreateModel)
				models.GET("/:id", h.GetModel)
				models.PUT("/:id", h.UpdateModel)
				models.DELETE("/:id", h.DeleteModel)
			}

			apis := auth.Group("/projects/:projectId/apis")
			{
				apis.GET("", h.ListAPIs)
				apis.POST("", h.CreateAPI)
				apis.GET("/:id", h.GetAPI)
				apis.PUT("/:id", h.UpdateAPI)
				apis.DELETE("/:id", h.DeleteAPI)

				apis.GET("/:id/scenarios", h.ListScenarios)
				apis.POST("/:id/scenarios", h.CreateScenario)
				apis.PUT("/:id/scenarios/:scenarioId", h.UpdateScenario)
				apis.DELETE("/:id/scenarios/:scenarioId", h.DeleteScenario)

				apis.GET("/:id/versions", h.ListVersions)
				apis.GET("/:id/versions/:versionId", h.GetVersion)
				apis.GET("/:id/versions/:versionId/diff", h.DiffVersions)
				apis.POST("/:id/versions/:versionId/rollback", h.RollbackVersion)
			}

			codegen := auth.Group("/codegen")
			{
				codegen.POST("", h.GenerateCode)
			}

			export := auth.Group("/export")
			{
				export.POST("/openapi", h.ExportOpenAPI)
				export.POST("/markdown", h.ExportMarkdown)
				export.POST("/curl", h.GenerateCurl)
			}

			auth.GET("/projects/:projectId/activities", h.ListActivities)
		}

		api.POST("/register", h.Register)
		api.POST("/login", h.Login)
		api.GET("/me", authMiddleware, h.GetCurrentUser)
	}

	mock := r.Group("/mock")
	{
		mock.Any("/*path", h.HandleMock)
	}

	fmt.Printf("Server starting on port %s\n", cfg.Port)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
