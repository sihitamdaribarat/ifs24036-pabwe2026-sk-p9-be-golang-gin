package database

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"app/internal/config"
	"app/internal/features/auth"
	"app/internal/features/post"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConnectSQLite(t *testing.T) {
	cfg := &config.Config{DBConnection: "sqlite", DBDatabase: filepath.Join(t.TempDir(), "test.db")}
	db, err := Connect(cfg)
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })
	assert.NoError(t, Migrate(db))
	assert.True(t, db.Migrator().HasTable("posts"))
	assert.True(t, db.Migrator().HasTable("users"))
}

func TestConnectWithRetry(t *testing.T) {
	cfg := &config.Config{DBConnection: "sqlite", DBDatabase: filepath.Join(t.TempDir(), "retry.db")}
	db, err := ConnectWithRetry(cfg, 3, time.Millisecond)
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	t.Cleanup(func() { sqlDB.Close() })

	start := time.Now()
	_, err = ConnectWithRetry(&config.Config{DBConnection: "mysql"}, 3, 5*time.Millisecond)
	assert.ErrorContains(t, err, "tidak didukung")
	assert.GreaterOrEqual(t, time.Since(start), 10*time.Millisecond)
}

func TestConnectUnsupported(t *testing.T) {
	_, err := Connect(&config.Config{DBConnection: "mysql"})
	assert.ErrorContains(t, err, "tidak didukung")
}

func TestConnectPostgresFailure(t *testing.T) {
	cfg := &config.Config{
		DBConnection: "pgsql", DBHost: "127.0.0.1", DBPort: "1",
		DBUsername: "x", DBPassword: "x", DBDatabase: "x", DBSSLMode: "disable",
	}
	_, err := Connect(cfg)
	assert.ErrorContains(t, err, "gagal terhubung ke database")
}

func TestConnectSQLiteFailure(t *testing.T) {
	// Direktori yang tidak ada membuat SQLite gagal dibuka.
	cfg := &config.Config{DBConnection: "sqlite", DBDatabase: filepath.Join(t.TempDir(), "missing", "dir", "x.db")}
	_, err := Connect(cfg)
	assert.Error(t, err)
}

func TestConnectTest(t *testing.T) {
	db, err := ConnectTest()
	require.NoError(t, err)
	assert.True(t, db.Migrator().HasTable("posts"))
}

func TestSeed(t *testing.T) {
	db, err := ConnectTest()
	require.NoError(t, err)

	require.NoError(t, Seed(db, "admin", "admin"))
	// Seeding kedua bersifat idempoten.
	require.NoError(t, Seed(db, "admin", "admin"))

	var admin auth.User
	require.NoError(t, db.Where("username = ?", "admin").First(&admin).Error)
	assert.Equal(t, auth.RoleAdmin, admin.Role)
	assert.NotEqual(t, "admin", admin.Password)

	var count int64
	db.Model(&post.Post{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestSeedPasswordTooLong(t *testing.T) {
	db, err := ConnectTest()
	require.NoError(t, err)
	assert.Error(t, Seed(db, "admin", strings.Repeat("x", 100)))
}

func TestSeedDatabaseErrors(t *testing.T) {
	// Tabel users tidak ada → query admin gagal.
	db, err := ConnectTest()
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable("users"))
	assert.Error(t, Seed(db, "admin", "admin"))

	// Tabel posts tidak ada → hitung artikel gagal.
	db, err = ConnectTest()
	require.NoError(t, err)
	require.NoError(t, db.Migrator().DropTable("posts"))
	assert.Error(t, Seed(db, "admin", "admin"))
}

func TestOpenTestFailure(t *testing.T) {
	_, err := openTest(filepath.Join(t.TempDir(), "missing", "dir", "x.db"))
	assert.Error(t, err)
}

func TestConnectTestClosedDB(t *testing.T) {
	db, err := ConnectTest()
	require.NoError(t, err)
	sqlDB, _ := db.DB()
	sqlDB.Close()
	assert.Error(t, Migrate(db))
}
