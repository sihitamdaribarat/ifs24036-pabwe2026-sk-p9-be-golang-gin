// Package post menangani CRUD artikel, slug unik, estimasi waktu baca, statistik
// penulis, artikel publik dan monitoring admin.
package post

import (
	"time"

	"app/internal/features/auth"
)

// Categories adalah daftar kategori artikel yang valid.
var Categories = []string{"Teknologi", "Pendidikan", "Gaya Hidup", "Bisnis", "Opini", "Umum"}

// DefaultCategory adalah kategori bawaan jika tidak diisi.
const DefaultCategory = "Umum"

// Post adalah model tabel posts.
type Post struct {
	ID             uint       `gorm:"primaryKey"`
	UserID         uint       `gorm:"not null;index"`
	User           *auth.User `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	Title          string     `gorm:"type:varchar(255);not null"`
	Slug           string     `gorm:"type:varchar(255);uniqueIndex;not null"`
	Category       string     `gorm:"type:varchar(100);not null;default:Umum"`
	Excerpt        string     `gorm:"type:text"`
	Content        string     `gorm:"type:text"`
	CoverImagePath string     `gorm:"type:text"`
	IsPublished    bool       `gorm:"not null;default:false;index"`
	PublishedAt    *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// PostRequest adalah payload pembuatan / pembaruan artikel.
type PostRequest struct {
	Title          string `json:"title" binding:"required,max=255" example:"Belajar Golang Gin"`
	Category       string `json:"category" binding:"max=100" example:"Teknologi"`
	Excerpt        string `json:"excerpt" example:"Ringkasan singkat artikel."`
	Content        string `json:"content" binding:"required" example:"# Judul\n\nIsi artikel dalam Markdown."`
	CoverImagePath string `json:"cover_image_path" example:"https://picsum.photos/1200/600"`
	IsPublished    bool   `json:"is_published" example:"false"`
}

// UpdatePostRequest adalah payload pembaruan artikel. Seluruh field opsional;
// hanya field yang dikirim yang diperbarui.
type UpdatePostRequest struct {
	Title          *string `json:"title" binding:"omitempty,max=255" example:"Belajar Golang Gin"`
	Category       *string `json:"category" binding:"omitempty,max=100" example:"Teknologi"`
	Excerpt        *string `json:"excerpt" example:"Ringkasan singkat artikel."`
	Content        *string `json:"content" example:"## Judul\n\nIsi artikel dalam Markdown."`
	CoverImagePath *string `json:"cover_image_path" example:"https://picsum.photos/1200/600"`
	IsPublished    *bool   `json:"is_published" example:"false"`
}

// Filter adalah parameter pencarian artikel.
type Filter struct {
	Q        string `form:"q"`
	Category string `form:"category"`
}

// AuthorInfo adalah ringkasan data penulis artikel.
type AuthorInfo struct {
	ID       uint   `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Bio      string `json:"bio"`
}

// PostResponse adalah representasi JSON artikel. user_id tidak ditampilkan pada
// endpoint publik (data internal); identitas penulis tersedia di field author.
type PostResponse struct {
	ID             uint        `json:"id"`
	UserID         uint        `json:"user_id,omitempty"`
	Title          string      `json:"title"`
	Slug           string      `json:"slug"`
	Category       string      `json:"category"`
	Excerpt        string      `json:"excerpt"`
	Content        string      `json:"content"`
	CoverImagePath string      `json:"cover_image_path"`
	IsPublished    bool        `json:"is_published"`
	PublishedAt    *time.Time  `json:"published_at"`
	ReadingTime    int         `json:"reading_time"`
	AuthorName     string      `json:"author_name"`
	Author         *AuthorInfo `json:"author,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

// Stats adalah ringkasan 3 kartu statistik penulis (Total Artikel, Total Selesai/Terbit,
// Belum Selesai/Draf).
type Stats struct {
	TotalArticles int64 `json:"total_articles"`
	TotalSelesai  int64 `json:"total_selesai"`
	BelumSelesai  int64 `json:"belum_selesai"`
}

// UserSummary adalah ringkasan pengguna untuk monitoring admin.
type UserSummary struct {
	ID             uint      `json:"id"`
	Name           string    `json:"name"`
	Username       string    `json:"username"`
	Email          string    `json:"email"`
	Role           string    `json:"role"`
	TotalArticles  int64     `json:"total_articles"`
	TotalPublished int64     `json:"total_published"`
	CreatedAt      time.Time `json:"created_at"`
}

// ToResponse mengubah model Post menjadi PostResponse lengkap dengan reading time.
func ToResponse(p Post) PostResponse {
	res := PostResponse{
		ID:             p.ID,
		UserID:         p.UserID,
		Title:          p.Title,
		Slug:           p.Slug,
		Category:       p.Category,
		Excerpt:        p.Excerpt,
		Content:        p.Content,
		CoverImagePath: p.CoverImagePath,
		IsPublished:    p.IsPublished,
		PublishedAt:    p.PublishedAt,
		ReadingTime:    ReadingTime(p.Content),
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
	if p.User != nil {
		res.AuthorName = p.User.Name
		res.Author =&AuthorInfo{ID: p.User.ID, Name: p.User.Name, Username: p.User.Username, Bio: p.User.Bio}
	}
	return res
}

// ToResponses mengubah slice Post menjadi slice PostResponse.
func ToResponses(posts []Post) []PostResponse {
	result := make([]PostResponse, 0, len(posts))
	for _, p := range posts {
		result = append(result, ToResponse(p))
	}
	return result
}
