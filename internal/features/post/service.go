package post

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"gorm.io/gorm"
)

// WordsPerMinute adalah asumsi kecepatan membaca untuk estimasi reading time.
const WordsPerMinute = 200

var (
	// ErrPostNotFound dikembalikan jika artikel tidak ditemukan.
	ErrPostNotFound = errors.New("artikel tidak ditemukan")
	// ErrForbidden dikembalikan jika artikel ada tetapi milik penulis lain.
	ErrForbidden = errors.New("anda tidak memiliki akses ke artikel ini")
	// ErrInvalidCategory dikembalikan jika kategori tidak dikenal.
	ErrInvalidCategory = errors.New("kategori tidak valid")
	// ErrEmptyField dikembalikan jika judul atau konten diperbarui menjadi kosong.
	ErrEmptyField = errors.New("judul dan konten artikel tidak boleh kosong")
)

// Service berisi logika bisnis artikel.
type Service struct {
	repo Repository
	now  func() time.Time
}

// NewService membuat Service artikel.
func NewService(repo Repository) *Service {
	return &Service{repo: repo, now: time.Now}
}

// Slugify mengubah judul menjadi slug URL (huruf kecil, alfanumerik, dipisah "-").
func Slugify(title string) string {
	var b strings.Builder
	lastDash := true
	for _, r := range strings.ToLower(title) {
		if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}

	slug := strings.Trim(b.String(), "-")
	if len(slug) > 200 {
		slug = strings.Trim(slug[:200], "-")
	}
	if slug == "" {
		return "artikel"
	}
	return slug
}

// ReadingTime menghitung estimasi waktu baca dalam menit (kata / 200 wpm, minimal 1).
func ReadingTime(content string) int {
	words := len(strings.Fields(content))
	minutes := (words + WordsPerMinute - 1) / WordsPerMinute
	return max(minutes, 1)
}

// NormalizeCategory memvalidasi kategori; kosong menjadi DefaultCategory.
func NormalizeCategory(category string) (string, error) {
	category = strings.TrimSpace(category)
	if category == "" {
		return DefaultCategory, nil
	}
	for _, c := range Categories {
		if strings.EqualFold(c, category) {
			return c, nil
		}
	}
	return "", ErrInvalidCategory
}

// uniqueSlug membuat slug unik: "judul", "judul-2", "judul-3", dst.
func (s *Service) uniqueSlug(title string, excludeID uint) (string, error) {
	base := Slugify(title)
	slug := base
	for i := 2; ; i++ {
		exists, err := s.repo.SlugExists(slug, excludeID)
		if err != nil {
			return "", err
		}
		if !exists {
			return slug, nil
		}
		slug = fmt.Sprintf("%s-%d", base, i)
	}
}

func notFound(err error, domainErr error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return domainErr
	}
	return err
}

// findOwned mengambil artikel berdasarkan ID: ErrPostNotFound jika tidak ada,
// ErrForbidden jika milik penulis lain.
func (s *Service) findOwned(userID, id uint) (*Post, error) {
	p, err := s.repo.FindByID(id)
	if err != nil {
		return nil, notFound(err, ErrPostNotFound)
	}
	if p.UserID != userID {
		return nil, ErrForbidden
	}
	return p, nil
}

// ListPublic mengambil artikel terbit untuk halaman publik (tanpa user_id).
func (s *Service) ListPublic(filter Filter) ([]PostResponse, error) {
	posts, err := s.repo.List(filter, 0, true)
	res := ToResponses(posts)
	for i := range res {
		res[i].UserID = 0
	}
	return res, err
}

// GetPublicBySlug mengambil detail artikel terbit berdasarkan slug (tanpa user_id).
func (s *Service) GetPublicBySlug(slug string) (*PostResponse, error) {
	p, err := s.repo.FindPublishedBySlug(slug)
	if err != nil {
		return nil, notFound(err, ErrPostNotFound)
	}
	res := ToResponse(*p)
	res.UserID = 0
	return &res, nil
}

// ListByAuthor mengambil seluruh artikel milik penulis (terbit & draf).
func (s *Service) ListByAuthor(userID uint, filter Filter) ([]PostResponse, error) {
	posts, err := s.repo.List(filter, userID, false)
	return ToResponses(posts), err
}

// GetByAuthor mengambil satu artikel milik penulis berdasarkan ID.
func (s *Service) GetByAuthor(userID, id uint) (*PostResponse, error) {
	p, err := s.findOwned(userID, id)
	if err != nil {
		return nil, err
	}
	res := ToResponse(*p)
	return &res, nil
}

// Create membuat artikel baru dengan slug unik.
func (s *Service) Create(userID uint, req PostRequest) (*PostResponse, error) {
	category, err := NormalizeCategory(req.Category)
	if err != nil {
		return nil, err
	}

	slug, err := s.uniqueSlug(req.Title, 0)
	if err != nil {
		return nil, err
	}

	p := &Post{
		UserID:         userID,
		Title:          strings.TrimSpace(req.Title),
		Slug:           slug,
		Category:       category,
		Excerpt:        strings.TrimSpace(req.Excerpt),
		Content:        req.Content,
		CoverImagePath: strings.TrimSpace(req.CoverImagePath),
	}
	s.setPublished(p, req.IsPublished)

	if err := s.repo.Create(p); err != nil {
		return nil, err
	}
	return s.GetByAuthor(userID, p.ID)
}

// Update memperbarui sebagian field artikel milik penulis (hanya field yang dikirim).
// Kepemilikan diperiksa lebih dulu (404/403), baru validasi isi. Slug dibuat ulang jika
// judul berubah.
func (s *Service) Update(userID, id uint, req UpdatePostRequest) (*PostResponse, error) {
	p, err := s.findOwned(userID, id)
	if err != nil {
		return nil, err
	}

	if req.Category != nil {
		category, err := NormalizeCategory(*req.Category)
		if err != nil {
			return nil, err
		}
		p.Category = category
	}
	if req.Content != nil {
		if strings.TrimSpace(*req.Content) == "" {
			return nil, ErrEmptyField
		}
		p.Content = *req.Content
	}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if title == "" {
			return nil, ErrEmptyField
		}
		if title != p.Title {
			slug, err := s.uniqueSlug(title, p.ID)
			if err != nil {
				return nil, err
			}
			p.Slug = slug
		}
		p.Title = title
	}
	if req.Excerpt != nil {
		p.Excerpt = strings.TrimSpace(*req.Excerpt)
	}
	if req.CoverImagePath != nil {
		p.CoverImagePath = strings.TrimSpace(*req.CoverImagePath)
	}
	if req.IsPublished != nil {
		s.setPublished(p, *req.IsPublished)
	}

	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	res := ToResponse(*p)
	return &res, nil
}

// TogglePublish membalik status terbit/draf artikel beserta published_at.
func (s *Service) TogglePublish(userID, id uint) (*PostResponse, error) {
	p, err := s.findOwned(userID, id)
	if err != nil {
		return nil, err
	}

	s.setPublished(p, !p.IsPublished)
	if err := s.repo.Update(p); err != nil {
		return nil, err
	}
	res := ToResponse(*p)
	return &res, nil
}

// Delete menghapus artikel milik penulis.
func (s *Service) Delete(userID, id uint) error {
	p, err := s.findOwned(userID, id)
	if err != nil {
		return err
	}
	return s.repo.Delete(p)
}

// Stats mengambil ringkasan 3 kartu statistik penulis (total, terbit, draf).
func (s *Service) Stats(userID uint) (Stats, error) {
	return s.repo.Stats(userID)
}

// ListAll mengambil seluruh artikel dari semua penulis (monitoring admin).
func (s *Service) ListAll(filter Filter) ([]PostResponse, error) {
	posts, err := s.repo.List(filter, 0, false)
	return ToResponses(posts), err
}

// ListUsers mengambil ringkasan seluruh pengguna beserta jumlah artikelnya.
func (s *Service) ListUsers() ([]UserSummary, error) {
	users, err := s.repo.ListUserSummaries()
	if users == nil {
		users = []UserSummary{}
	}
	return users, err
}

// ListByUser mengambil seluruh artikel pengguna tertentu (monitoring admin).
// Pengguna yang tidak ada menghasilkan daftar kosong, bukan error.
func (s *Service) ListByUser(userID uint) ([]PostResponse, error) {
	return s.ListByAuthor(userID, Filter{})
}

// setPublished mengatur status publikasi; published_at diisi saat pertama kali terbit
// dan dikosongkan saat kembali menjadi draf.
func (s *Service) setPublished(p *Post, published bool) {
	if published && !p.IsPublished {
		now := s.now()
		p.PublishedAt = &now
	}
	if !published {
		p.PublishedAt = nil
	}
	p.IsPublished = published
}
