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
	cognitoUserPoolID := os.Getenv("COGNITO_USER_POOL_ID")

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
		&db.ShareLink{},
		&db.ShareUpload{},
	); err != nil {
		log.Fatalf("auto-migrate failed: %v", err)
	}
	log.Println("migrations complete")

	// Seed default org if none exists
	seedDefaultOrg(database)

	// AWS clients
	awsClients, err := lib.NewAWSClients(context.Background(), s3Bucket, sqsQueueURL, cognitoUserPoolID)
	if err != nil {
		log.Printf("warning: AWS clients unavailable (dev mode): %v", err)
		awsClients = &lib.AWSClients{Bucket: s3Bucket, QueueURL: sqsQueueURL, UserPoolID: cognitoUserPoolID}
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
	adminOrgHandler := routes.NewAdminOrgHandler(database, awsClients)
	shareHandler := routes.NewShareHandler(database, awsClients)

	// Auth routes (no JWT required for callback)
	authGroup := e.Group("/auth")
	authGroup.POST("/callback", authHandler.Callback)
	authGroup.GET("/me", authHandler.Me, jwtMW.Middleware())

	// Read-only routes — all authenticated users (VIEWER, ANALYST, ADMIN)
	read := e.Group("", jwtMW.Middleware())
	read.GET("/submissions", submissionsHandler.ListSubmissions)
	read.GET("/submissions/:id", submissionsHandler.GetSubmission)
	read.GET("/jobs/:id", jobsHandler.GetJob)
	read.GET("/jobs/:id/violations", jobsHandler.GetViolations)
	read.GET("/jobs/:id/board", jobsHandler.GetBoardData)
	read.GET("/jobs/:id/report.pdf", reportHandler.GetJobReport)
	read.GET("/profiles", profilesHandler.ListProfiles)
	read.GET("/profiles/:id", profilesHandler.GetProfile)

	// Write routes — ANALYST + ADMIN only
	write := e.Group("", jwtMW.Middleware(), lib.RequireRole("ANALYST", "ADMIN"))
	write.POST("/submissions", submissionsHandler.CreateSubmission)
	write.POST("/submissions/:id/analyze", submissionsHandler.StartAnalysis)
	write.PATCH("/violations/:id", jobsHandler.UpdateViolation)
	write.PATCH("/jobs/:id/violations/by-layer", jobsHandler.BulkIgnoreLayerViolations)
	write.POST("/profiles", profilesHandler.CreateProfile)
	write.PUT("/profiles/:id", profilesHandler.UpdateProfile)
	write.DELETE("/profiles/:id", profilesHandler.DeleteProfile)
	write.POST("/share-links", shareHandler.CreateShareLink)
	write.GET("/share-links", shareHandler.ListShareLinks)
	write.DELETE("/share-links/:id", shareHandler.DeactivateShareLink)
	write.GET("/share-links/:id/uploads", shareHandler.ListShareUploads)

	// Public shared routes (token-based auth, no JWT)
	shared := e.Group("/shared/:token", shareHandler.TokenMiddleware())
	shared.GET("", shareHandler.GetShareInfo)
	shared.GET("/submissions", shareHandler.GetSharedSubmissions)
	shared.GET("/jobs/:jobId", shareHandler.GetSharedJob)
	shared.GET("/jobs/:jobId/violations", shareHandler.GetSharedViolations)
	shared.GET("/jobs/:jobId/board", shareHandler.GetSharedBoardData)
	shared.POST("/upload", shareHandler.SharedUpload)

	// Admin routes (separate JWT audience)
	adminAPI := e.Group("/admin", adminJWTMW.AdminMiddleware())
	adminAPI.GET("/stats", adminOrgHandler.GetPlatformStats)
	adminAPI.GET("/organizations", adminOrgHandler.ListOrganizations)
	adminAPI.POST("/organizations", adminOrgHandler.CreateOrganization)
	adminAPI.GET("/organizations/:id", adminOrgHandler.GetOrganization)
	adminAPI.PUT("/organizations/:id", adminOrgHandler.UpdateOrganization)
	adminAPI.GET("/organizations/:id/stats", adminOrgHandler.GetOrganizationStats)
	adminAPI.GET("/organizations/:id/users", adminOrgHandler.ListOrgUsers)
	adminAPI.POST("/organizations/:id/users", adminOrgHandler.CreateOrgUser)
	adminAPI.PUT("/organizations/:id/users/:userId", adminOrgHandler.UpdateOrgUser)
	adminAPI.DELETE("/organizations/:id/users/:userId", adminOrgHandler.DeleteOrgUser)

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
