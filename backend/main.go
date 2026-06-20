package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"

	"api-mocker/cache"
	"api-mocker/config"
	"api-mocker/database"
	"api-mocker/handlers"
	"api-mocker/middleware"
	"api-mocker/probe"
	"api-mocker/websocket"
)

func requestSizeLimitMiddleware(maxSize int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.ContentLength > maxSize {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{
				"error": fmt.Sprintf("请求过大，最大允许 %d MB", maxSize/1024/1024),
			})
			c.Abort()
			return
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxSize)
		c.Next()
	}
}

func main() {
	cfg := config.Load()

	db := database.Connect(cfg)
	defer db.Close()

	rdb := cache.Connect(cfg)

	wsHub := websocket.NewHub()

	scheduler := probe.NewScheduler(db, cfg.MockBaseURL, wsHub)
	scheduler.Start()
	defer scheduler.Stop()

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

	h := handlers.New(db, rdb, cfg, scheduler, wsHub)

	api := r.Group("/api")
	{
		auth := api.Group("")
		auth.Use(authMiddleware)
		{
			workspaces := auth.Group("/workspaces")
			{
				workspaces.GET("", h.ListWorkspaces)
				workspaces.POST("", h.CreateWorkspace)
				workspaces.GET("/:workspaceId", h.GetWorkspace)
				workspaces.PUT("/:workspaceId", h.UpdateWorkspace)
				workspaces.DELETE("/:workspaceId", h.DeleteWorkspace)

				workspaces.GET("/:workspaceId/members", h.ListMembers)
				workspaces.POST("/:workspaceId/members/invite", h.InviteMember)
				workspaces.POST("/join", h.JoinWorkspace)
				workspaces.PUT("/:workspaceId/members/:memberId", h.UpdateMemberRole)
				workspaces.DELETE("/:workspaceId/members/:memberId", h.RemoveMember)

				workspaces.GET("/:workspaceId/projects", h.ListProjects)
				workspaces.POST("/:workspaceId/projects", h.CreateProject)
				workspaces.GET("/:workspaceId/projects/:projectId", h.GetProject)
				workspaces.PUT("/:workspaceId/projects/:projectId", h.UpdateProject)
				workspaces.DELETE("/:workspaceId/projects/:projectId", h.DeleteProject)
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
				apis.POST("/import", requestSizeLimitMiddleware(2*1024*1024), h.ImportOpenAPI)
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

			probes := auth.Group("/projects/:projectId/probes")
			{
				probes.GET("/ws", h.ProbeWebSocket)
				probes.GET("", h.ListProbes)
				probes.POST("", h.CreateProbe)
				probes.POST("/batch/enable", h.BatchEnableProbes)
				probes.POST("/batch/disable", h.BatchDisableProbes)
				probes.POST("/batch/delete", h.BatchDeleteProbes)
				probes.GET("/dashboard", h.GetProbeDashboard)
				probes.GET("/alerts", h.GetProbeAlerts)
				probes.GET("/:probeId", h.GetProbeDetail)
				probes.GET("/:probeId/availability-trend", h.GetProbeAvailabilityTrend)
				probes.PUT("/:probeId", h.UpdateProbe)
				probes.DELETE("/:probeId", h.DeleteProbe)
			}

			apis.GET("/:id/probe", h.GetAPIProbe)
			apis.POST("/:id/probe", h.CreateProbeForAPI)
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
