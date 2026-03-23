package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"

	"snap-erp-api/internal/api"
	"snap-erp-api/internal/middleware"
	"snap-erp-api/internal/db"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("⚠️  .env 파일 없음 — 환경변수 직접 사용")
	}

	db.Init()

	r := gin.Default()
	r.Use(middleware.CORS())

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok", "service": "ECOYA SNAP ERP API", "version": "v1.0"})
	})

	v1 := r.Group("/api/v1")
	v1.Use(middleware.Auth())
	{
		api.RegisterDocumentRoutes(v1)
		api.RegisterBLRoutes(v1)
		api.RegisterPORoutes(v1)
		api.RegisterSettleRoutes(v1)
		api.RegisterSnapRoutes(v1)
		api.RegisterPortalRoutes(v1)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("🚀 SNAP ERP API 시작 — http://localhost:%s", port)
	r.Run(":" + port)
}
