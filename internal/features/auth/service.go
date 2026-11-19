package auth

import (
	"errors"
	"regexp"
	"strings"

	"app/internal/middleware"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	// ErrUsernameTaken dikembalikan jika username sudah terdaftar.
	ErrUsernameTaken = errors.New("username sudah digunakan")
	// ErrEmailTaken dikembalikan jika email sudah terdaftar.
	ErrEmailTaken = errors.New("email sudah digunakan")
	// ErrInvalidCredentials dikembalikan jika username/password salah.
	ErrInvalidCredentials = errors.New("username atau password salah")
	// ErrUserNotFound dikembalikan jika user tidak ditemukan.
	ErrUserNotFound = errors.New("user tidak ditemukan")
	// ErrInvalidUsername dikembalikan jika username mengandung karakter yang tidak diizinkan.
	ErrInvalidUsername = errors.New("username hanya boleh berisi huruf, angka, titik, garis bawah, atau tanda hubung")
)

// usernamePattern membatasi karakter username (setelah diubah ke huruf kecil).
var usernamePattern = regexp.MustCompile(`^[a-z0-9_.-]+$`)

// Service berisi logika bisnis autentikasi.
type Service struct {
	repo   Repository
	secret string
}

// NewService membuat Service autentikasi.
func NewService(repo Repository, secret string) *Service {
	return &Service{repo: repo, secret: secret}
}

// HashPassword meng-hash password menggunakan bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}

// Register mendaftarkan akun penulis baru dengan role member.
func (s *Service) Register(req RegisterRequest) (*User, error) {
	username := strings.ToLower(strings.TrimSpace(req.Username))
	if !usernamePattern.MatchString(username) {
		return nil, ErrInvalidUsername
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))
	if email == "" {
		email = username + "@delcom.org"
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = username
	}

	count, err := s.repo.CountByUsername(username)
	if err != nil {
		return nil, err
	}
	if count > 0 {
		return nil, ErrUsernameTaken
	}

	if err := s.ensureEmailAvailable(email, 0); err != nil {
		return nil, err
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &User{
		Name:     name,
		Username: username,
		Email:    email,
		Password: hash,
		Role:     RoleMember,
	}
	if err := s.repo.Create(user); err != nil {
		return nil, err
	}
	return user, nil
}

// Login memverifikasi kredensial dan menerbitkan token JWT.
func (s *Service) Login(req LoginRequest) (*LoginResponse, error) {
	user, err := s.repo.FindByLogin(strings.ToLower(req.Identifier()))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}

	if bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) != nil {
		return nil, ErrInvalidCredentials
	}

	token, err := middleware.GenerateToken(s.secret, user.ID, user.Username, user.Role)
	return &LoginResponse{Token: token, User: *user}, err
}

// Profile mengambil data user berdasarkan ID.
func (s *Service) Profile(userID uint) (*User, error) {
	user, err := s.repo.FindByID(userID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrUserNotFound
	}
	return user, err
}

// UpdateProfile memperbarui nama, email, bio dan password user. Field yang kosong
// (atau tidak dikirim) dibiarkan seperti semula.
func (s *Service) UpdateProfile(userID uint, req UpdateProfileRequest) (*User, error) {
	user, err := s.Profile(userID)
	if err != nil {
		return nil, err
	}

	if email := strings.ToLower(strings.TrimSpace(req.Email)); email != "" {
		if err := s.ensureEmailAvailable(email, user.ID); err != nil {
			return nil, err
		}
		user.Email = email
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		user.Name = name
	}
	if req.Bio != nil {
		user.Bio = strings.TrimSpace(*req.Bio)
	}

	if req.Password != "" {
		hash, err := HashPassword(req.Password)
		if err != nil {
			return nil, err
		}
		user.Password = hash
	}

	if err := s.repo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *Service) ensureEmailAvailable(email string, excludeID uint) error {
	count, err := s.repo.CountByEmail(email, excludeID)
	if err != nil {
		return err
	}
	if count > 0 {
		return ErrEmailTaken
	}
	return nil
}
