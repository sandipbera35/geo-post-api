package main

import (
	"context"
	"database/sql"
	"log"
	"net/http"
	"path/filepath"
	"runtime"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"github.com/sandipbera35/geo-post-api/config"
	"github.com/sandipbera35/geo-post-api/controllers"
	"github.com/sandipbera35/geo-post-api/db"
	"github.com/sandipbera35/geo-post-api/middlewares"
	"github.com/sandipbera35/geo-post-api/repo"
	"github.com/sandipbera35/geo-post-api/service"
	"github.com/sandipbera35/geo-post-api/workers"
)

// cfg, pg, rdb, and sqlDB are populated once in init, before main runs, and
// are read-only afterwards — main just wires them into routes.
var (
	cfg         *config.Config
	pg          *gorm.DB
	rdb         *redis.Client
	sqlDB       *sql.DB
	minioClient *minio.Client
)

// init loads .env (if present), reads config, loads the JWT public key, and
// opens + pings Postgres and Redis — every external connection is made and
// verified here, before main starts accepting traffic. Any failure is fatal:
// there's no point serving requests against a half-wired app.
func init() {
	loadEnvFile()

	cfg = config.Load()

	if err := middlewares.LoadPublicKey(cfg.JWTPublicKeyPath); err != nil {
		log.Fatalf("jwt public key: %v", err)
	}

	var err error
	pg, err = db.NewPostgres(cfg.DBURL)
	if err != nil {
		log.Fatalf("postgres: %v", err)
	}

	rdb, err = db.NewRedis(context.Background(), cfg.RedisURL)
	if err != nil {
		log.Fatalf("redis: %v", err)
	}

	sqlDB, err = pg.DB()
	if err != nil {
		log.Fatalf("postgres: get underlying sql.DB: %v", err)
	}

	minioClient, err = db.NewMinio(cfg)
	if err != nil {
		log.Fatalf("minio: %v", err)
	}
}

// loadEnvFile loads .env via godotenv, which sets environment variables
// without overriding anything already exported in the shell. It tries the
// current working directory first, then falls back to the directory this
// source file lives in, so `go run .` finds .env the same way regardless of
// which directory it's invoked from. A missing file is not an error — .env
// is optional (variables can always be exported directly instead).
func loadEnvFile() {
	if err := godotenv.Load(); err == nil {
		return
	}

	if _, thisFile, _, ok := runtime.Caller(0); ok {
		_ = godotenv.Load(filepath.Join(filepath.Dir(thisFile), ".env"))
	}
}

// main is the single composition root for geo-post-api. All connections are
// already up (see init above); this just wires routes and starts serving.
func main() {
	eventRepo := repo.NewEventRepository(pg, rdb)
	feedRepo := repo.NewFeedRepository(pg, rdb)
	profileRepo := repo.NewUserProfileRepository(rdb)
	uploadRepo := repo.NewUploadRepository(minioClient)
	eventService := service.NewEventService(eventRepo)
	feedService := service.NewFeedService(feedRepo, profileRepo)
	uploadService := service.NewUploadService(uploadRepo, cfg.MinioEndpoint, cfg.MinioUseSSL)

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		if err := sqlDB.PingContext(c.Request.Context()); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down", "component": "postgres"})
			return
		}
		if err := rdb.Ping(c.Request.Context()).Err(); err != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"status": "down", "component": "redis"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	v1 := router.Group("/api/v1")
	v1.Use(middlewares.RequireAuth())
	v1.GET("/me", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"user_id": c.GetString("user_id")})
	})
	v1.POST("/events", controllers.CreateEventHandler(eventService))
	v1.GET("/feed", controllers.GetFeedHandler(feedService))
	v1.GET("/uploads/presigned-url", controllers.GenerateUploadURL(uploadService))

	go workers.StartExpiryWorker(pg, rdb, 15*time.Minute)

	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
