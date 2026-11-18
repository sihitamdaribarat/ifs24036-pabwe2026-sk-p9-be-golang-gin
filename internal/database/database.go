// Package database mengelola koneksi PostgreSQL (runtime) / SQLite in-memory (testing),
// migrasi tabel dan seeding data awal.
package database

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"app/internal/config"
	"app/internal/features/auth"
	"app/internal/features/post"

	"github.com/glebarez/sqlite"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Connect membuka koneksi database sesuai DB_CONNECTION (pgsql / sqlite).
func Connect(cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.DBConnection {
	case "sqlite":
		dialector = sqlite.Open(cfg.DBDatabase)
	case "pgsql", "postgres", "postgresql":
		// PreferSimpleProtocol: tanpa prepared statement agar kompatibel dengan PgBouncer
		// (transaction pooling) di server deployment.
		dialector = postgres.New(postgres.Config{DSN: cfg.DSN(), PreferSimpleProtocol: true})
	default:
		return nil, fmt.Errorf("DB_CONNECTION %q tidak didukung", cfg.DBConnection)
	}

	db, err := gorm.Open(dialector, &gorm.Config{Logger: logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		SlowThreshold:             time.Second,
		LogLevel:                  logger.Warn,
		IgnoreRecordNotFoundError: true,
		Colorful:                  true,
	})})
	if err != nil {
		return nil, fmt.Errorf("gagal terhubung ke database: %w", err)
	}
	return db, nil
}

// ConnectTest membuka database SQLite in-memory yang terisolasi untuk pengujian TDD.
func ConnectTest() (*gorm.DB, error) {
	return openTest(":memory:")
}

func openTest(dsn string) (*gorm.DB, error) {
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, err
	}
	// Satu koneksi agar seluruh query memakai database in-memory yang sama.
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	return db, Migrate(db)
}

// Migrate memigrasikan tabel users dan posts.
func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&auth.User{}, &post.Post{})
}

// Seed membuat akun admin (dari ADMIN_USERNAME / ADMIN_PASSWORD) dan satu artikel sambutan
// jika belum ada.
func Seed(db *gorm.DB, username, password string) error {
	var admin auth.User
	err := db.Where("username = ?", username).First(&admin).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		hash, hashErr := auth.HashPassword(password)
		if hashErr != nil {
			return hashErr
		}
		admin = auth.User{
			Name:     "Administrator",
			Username: username,
			Email:    username + "@delcom.org",
			Password: hash,
			Role:     auth.RoleAdmin,
			Bio:      "Administrator Delcom Blog",
		}
		err = db.Create(&admin).Error
	}
	if err != nil {
		return err
	}

	var count int64
	if err := db.Model(&post.Post{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}

	now := time.Now()
	welcome := post.Post{
		UserID:      admin.ID,
		Title:       "Selamat Datang di Delcom Blog",
		Slug:        "selamat-datang-di-delcom-blog",
		Category:    "Umum",
		Excerpt:     "Delcom Blog adalah platform publikasi artikel multi-tier dengan AI Blog Generator yang terhubung ke mesin pencari.",
		Content:     welcomeContent,
		IsPublished: true,
		PublishedAt: &now,
	}
	return db.Create(&welcome).Error
}

const welcomeContent = `## Halo, Pembaca!

**Delcom Blog** adalah platform publikasi artikel dengan tiga tingkatan akses:

1. **Publik** — membaca, mencari dan memfilter artikel terbit tanpa login.
2. **Penulis** — mengelola artikel sendiri, melihat statistik, dan membuat draf otomatis lewat *AI Blog Generator*.
3. **Admin** — memantau seluruh pengguna dan artikel di dalam sistem.

## AI Blog Generator

Fitur AI menelusuri topik ke **mesin pencari** terlebih dahulu, kemudian menyusun artikel
lengkap beserta **sumber rujukan** yang dapat diaudit.

> Daftar sekarang dan mulai menulis artikel pertama Anda!
`
