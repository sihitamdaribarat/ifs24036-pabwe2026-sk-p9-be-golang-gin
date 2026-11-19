package post

import (
	"strings"

	"app/internal/features/auth"

	"gorm.io/gorm"
)

// Repository adalah akses data tabel posts.
type Repository interface {
	List(filter Filter, userID uint, publishedOnly bool) ([]Post, error)
	FindPublishedBySlug(slug string) (*Post, error)
	FindByID(id uint) (*Post, error)
	SlugExists(slug string, excludeID uint) (bool, error)
	Create(post *Post) error
	Update(post *Post) error
	Delete(post *Post) error
	Stats(userID uint) (Stats, error)
	ListUserSummaries() ([]UserSummary, error)
}

type repository struct {
	db *gorm.DB
}

// NewRepository membuat Repository berbasis GORM.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

// List mengambil artikel dengan filter kata kunci & kategori.
// userID = 0 berarti semua penulis; publishedOnly = true hanya artikel terbit.
func (r *repository) List(filter Filter, userID uint, publishedOnly bool) ([]Post, error) {
	query := r.db.Preload("User")

	if userID != 0 {
		query = query.Where("user_id = ?", userID)
	}
	if publishedOnly {
		query = query.Where("is_published = ?", true).Order("published_at DESC")
	}
	if q := strings.TrimSpace(filter.Q); q != "" {
		like := "%" + strings.ToLower(q) + "%"
		query = query.Where("(LOWER(title) LIKE ? OR LOWER(excerpt) LIKE ? OR LOWER(content) LIKE ?)", like, like, like)
	}
	if category := strings.TrimSpace(filter.Category); category != "" {
		query = query.Where("category = ?", category)
	}

	var posts []Post
	err := query.Order("created_at DESC").Order("id DESC").Find(&posts).Error
	return posts, err
}

func (r *repository) FindPublishedBySlug(slug string) (*Post, error) {
	var p Post
	if err := r.db.Preload("User").Where("slug = ? AND is_published = ?", slug, true).First(&p).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *repository) FindByID(id uint) (*Post, error) {
	var p Post
	if err := r.db.Preload("User").First(&p, id).Error; err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *repository) SlugExists(slug string, excludeID uint) (bool, error) {
	var count int64
	err := r.db.Model(&Post{}).Where("slug = ? AND id <> ?", slug, excludeID).Count(&count).Error
	return count > 0, err
}

func (r *repository) Create(p *Post) error {
	return r.db.Create(p).Error
}

func (r *repository) Update(p *Post) error {
	return r.db.Omit("User").Save(p).Error
}

func (r *repository) Delete(p *Post) error {
	return r.db.Delete(p).Error
}

func (r *repository) Stats(userID uint) (Stats, error) {
	var stats Stats
	err := r.db.Model(&Post{}).
		Select("COUNT(*) AS total_articles, COALESCE(SUM(CASE WHEN is_published THEN 1 ELSE 0 END), 0) AS total_selesai").
		Where("user_id = ?", userID).
		Scan(&stats).Error
	stats.BelumSelesai = stats.TotalArticles - stats.TotalSelesai
	return stats, err
}

func (r *repository) ListUserSummaries() ([]UserSummary, error) {
	var summaries []UserSummary
	err := r.db.Model(&auth.User{}).
		Select(`users.id, users.name, users.username, users.email, users.role, users.created_at,
			COUNT(posts.id) AS total_posts,
			COALESCE(SUM(CASE WHEN posts.is_published THEN 1 ELSE 0 END), 0) AS published_posts`).
		Joins("LEFT JOIN posts ON posts.user_id = users.id").
		Group("users.id, users.name, users.username, users.email, users.role, users.created_at").
		Order("users.id ASC").
		Scan(&summaries).Error
	return summaries, err
}
