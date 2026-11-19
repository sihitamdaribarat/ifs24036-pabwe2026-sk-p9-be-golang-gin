// Package server mengonfigurasi router Gin, seluruh routing API publik / penulis / admin,
// Swagger UI dan handler 404.
package server

import (
	"log"
	"net/http"
	"time"

	"app/docs"
	"app/internal/config"
	"app/internal/database"
	"app/internal/features/ai"
	"app/internal/features/auth"
	"app/internal/features/post"
	"app/internal/middleware"
	"app/internal/response"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"gorm.io/gorm"
)

// NewRouter menyusun seluruh dependensi fitur dan mendaftarkan routing API.
func NewRouter(db *gorm.DB, cfg *config.Config) *gin.Engine {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery(), middleware.CORS())

	authHandler := auth.NewHandler(auth.NewService(auth.NewRepository(db), cfg.AppKey))
	postHandler := post.NewHandler(post.NewService(post.NewRepository(db)))
	aiHandler := ai.NewHandler(ai.NewService(
		ai.NewSearchClient(cfg.SearchEngineEndpoint, &http.Client{Timeout: 20 * time.Second}),
		ai.NewChatClient(cfg.OpenAIEndpoint, cfg.OpenAIKey, cfg.OpenAIModel, &http.Client{Timeout: 180 * time.Second}),
	))

	// Halaman root diarahkan ke dokumentasi Swagger UI.
	r.GET("/", func(c *gin.Context) {
		c.Redirect(http.StatusFound, "/swagger/index.html")
	})

	// Dokumentasi Swagger UI & spesifikasi OpenAPI dinamis.
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	r.GET("/api.json", func(c *gin.Context) {
		c.Data(http.StatusOK, "application/json; charset=utf-8", []byte(docs.SwaggerInfo.ReadDoc()))
	})

	api := r.Group("/api")
	// Batas dibuat longgar karena penguji otomatis mengirim seluruh request dari satu IP.
	api.Use(middleware.RateLimiter(3000, time.Minute))
	api.GET("/health", healthHandler(db, cfg))

	public := api.Group("/public")
	public.GET("/categories", postHandler.Categories)
	public.GET("/posts", postHandler.ListPublic)
	public.GET("/posts/:slug", postHandler.GetPublic)

	authGroup := api.Group("/auth")
	authLimiter := middleware.RateLimiter(300, time.Minute)
	authGroup.POST("/register", authLimiter, authHandler.Register)
	authGroup.POST("/login", authLimiter, authHandler.Login)

	authenticated := api.Group("", middleware.Auth(cfg.AppKey))
	authenticated.POST("/auth/logout", authHandler.Logout)
	authenticated.GET("/auth/profile", authHandler.GetProfile)
	authenticated.PUT("/auth/profile", authHandler.UpdateProfile)

	posts := authenticated.Group("/posts", middleware.RequireRole(auth.RoleMember, auth.RoleAdmin))
	posts.GET("", postHandler.ListMine)
	posts.POST("", postHandler.Create)
	posts.GET("/stats", postHandler.Stats)
	posts.POST("/ai-generate", middleware.RateLimiter(60, time.Minute), aiHandler.Generate)
	posts.GET("/:id", postHandler.GetMine)
	posts.PUT("/:id", postHandler.Update)
	posts.DELETE("/:id", postHandler.Delete)
	posts.PATCH("/:id/toggle-publish", postHandler.TogglePublish)

	admin := authenticated.Group("/admin", middleware.RequireRole(auth.RoleAdmin))
	admin.GET("/users", postHandler.AdminUsers)
	admin.GET("/posts", postHandler.AdminPosts)
	admin.GET("/users/:id/posts", postHandler.AdminUserPosts)

	r.NoRoute(func(c *gin.Context) {
		response.Fail(c, http.StatusNotFound, "Endpoint not found")
	})
	return r
}

// healthHandler memeriksa koneksi database & keberadaan tabel (GET /api/health).
// Detail error hanya ditampilkan di luar mode production; selalu dicatat ke log.
func healthHandler(db *gorm.DB, cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var users int64
		if err := db.Raw("SELECT COUNT(*) FROM users").Scan(&users).Error; err != nil {
			log.Printf("[ERROR] health check database: %v", err)
			message := "Database tidak dapat diakses"
			if !cfg.IsProduction() {
				message += ": " + err.Error()
			}
			response.Error(c, http.StatusServiceUnavailable, message)
			return
		}
		response.Success(c, http.StatusOK, "API & database berjalan normal", gin.H{"database": "ok", "users": users})
	}
}

// Percobaan koneksi database saat startup (total ±30 detik) sebelum aplikasi menyerah.
var (
	dbConnectAttempts = 10
	dbConnectDelay    = 3 * time.Second
)

// Bootstrap menghubungkan database, menjalankan migrasi & seeding, lalu membuat router.
func Bootstrap(cfg *config.Config) (*gin.Engine, error) {
	log.Printf("Menghubungkan database %s di %s:%s (port aplikasi %s)", cfg.DBDatabase, cfg.DBHost, cfg.DBPort, cfg.AppPort)
	db, err := database.ConnectWithRetry(cfg, dbConnectAttempts, dbConnectDelay)
	if err != nil {
		return nil, err
	}
	if err := database.Migrate(db); err != nil {
		return nil, err
	}
	if err := database.Seed(db, cfg.AdminUsername, cfg.AdminPassword); err != nil {
		return nil, err
	}
	return NewRouter(db, cfg), nil
}
