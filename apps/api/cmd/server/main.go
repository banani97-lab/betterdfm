package main

import (
	"context"
	"log"
	"net/http"
	"os"

	"github.com/betterdfm/api/src/db"
	"github.com/betterdfm/api/src/lib"
	"github.com/betterdfm/api/src/routes"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		databaseURL = "postgres://postgres:secret@localhost:5432/betterdfm"
	}
	s3Bucket := os.Getenv("S3_BUCKET")
	if s3Bucket == "" {
		s3Bucket = "betterdfm-uploads"
	}
	sqsQueueURL := os.Getenv("SQS_QUEUE_URL")
	jwtIssuer := os.Getenv("JWT_ISSUER")
	cognitoClientID := os.Getenv("COGNITO_CLIENT_ID")
	adminCognitoClientID := os.Getenv("ADMIN_COGNITO_CLIENT_ID")

	// Database
	database, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	log.Println("connected to database")

	// Auto-migrate
	if err := database.AutoMigrate(
		&db.Organization{},
		&db.User{},
		&db.CapabilityProfile{},
		&db.Submission{},
		&db.AnalysisJob{},
		&db.Violation{},
	); err != nil {
		log.Fatalf("auto-migrate failed: %v", err)
	}
	log.Println("migrations complete")

	// Seed default org if none exists
	seedDefaultOrg(database)

	// AWS clients
	awsClients, err := lib.NewAWSClients(context.Background(), s3Bucket, sqsQueueURL)
	if err != nil {
		log.Printf("warning: AWS clients unavailable (dev mode): %v", err)
		awsClients = &lib.AWSClients{Bucket: s3Bucket, QueueURL: sqsQueueURL}
	}

	// Echo
	e := echo.New()
	e.HideBanner = true

	// Middleware
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(middleware.GzipWithConfig(middleware.GzipConfig{Level: 5}))
	e.Use(middleware.CORSWithConfig(middleware.CORSConfig{
		AllowOrigins: []string{"http://localhost:3000", "https://*"},
		AllowMethods: []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions},
		AllowHeaders: []string{echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization},
	}))

	// Health check (unauthenticated)
	e.GET("/health", func(c echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// JWT middleware for app users (validates audience against app client ID)
	jwtMW := lib.NewJWTMiddleware(jwtIssuer, cognitoClientID)

	// JWT middleware for BetterDFM admins (validates audience against admin client ID)
	adminJWTMW := lib.NewJWTMiddleware(jwtIssuer, adminCognitoClientID)

	// Route handlers
	authHandler := routes.NewAuthHandler(database)
	submissionsHandler := routes.NewSubmissionsHandler(database, awsClients)
	jobsHandler := routes.NewJobsHandler(database)
	reportHandler := routes.NewReportHandler(database)
	profilesHandler := routes.NewProfilesHandler(database)
	adminOrgHandler := routes.NewAdminOrgHandler(database)

	// Auth routes (no JWT required for callback)
	authGroup := e.Group("/auth")
	authGroup.POST("/callback", authHandler.Callback)
	authGroup.GET("/me", authHandler.Me, jwtMW.Middleware())

	// Protected app routes
	api := e.Group("", jwtMW.Middleware())

	api.GET("/submissions", submissionsHandler.ListSubmissions)
	api.POST("/submissions", submissionsHandler.CreateSubmission)
	api.GET("/submissions/:id", submissionsHandler.GetSubmission)
	api.POST("/submissions/:id/analyze", submissionsHandler.StartAnalysis)

	api.GET("/jobs/:id", jobsHandler.GetJob)
	api.GET("/jobs/:id/violations", jobsHandler.GetViolations)
	api.GET("/jobs/:id/board", jobsHandler.GetBoardData)
	api.GET("/jobs/:id/report.pdf", reportHandler.GetJobReport)
	api.PATCH("/jobs/:id/violations/by-layer", jobsHandler.BulkIgnoreLayerViolations)
	api.PATCH("/violations/:id", jobsHandler.UpdateViolation)

	api.GET("/profiles", profilesHandler.ListProfiles)
	api.POST("/profiles", profilesHandler.CreateProfile)
	api.GET("/profiles/:id", profilesHandler.GetProfile)
	api.PUT("/profiles/:id", profilesHandler.UpdateProfile)
	api.DELETE("/profiles/:id", profilesHandler.DeleteProfile)

	// Admin routes (separate JWT audience)
	adminAPI := e.Group("/admin", adminJWTMW.AdminMiddleware())
	adminAPI.GET("/stats", adminOrgHandler.GetPlatformStats)
	adminAPI.GET("/organizations", adminOrgHandler.ListOrganizations)
	adminAPI.POST("/organizations", adminOrgHandler.CreateOrganization)
	adminAPI.GET("/organizations/:id", adminOrgHandler.GetOrganization)
	adminAPI.PUT("/organizations/:id", adminOrgHandler.UpdateOrganization)
	adminAPI.GET("/organizations/:id/stats", adminOrgHandler.GetOrganizationStats)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("starting server on :%s", port)
	if err := e.Start(":" + port); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func seedDefaultOrg(database *gorm.DB) {
	var count int64
	database.Model(&db.Organization{}).Count(&count)
	if count == 0 {
		org := db.Organization{
			ID:   "default-org",
			Slug: "default",
			Name: "Default Organization",
		}
		if err := database.Create(&org).Error; err != nil {
			log.Printf("seed org: %v", err)
		}
	}
}
