// Package auth menangani registrasi penulis, login JWT, logout dan profil akun.
package auth

import (
	"strings"
	"time"
)

const (
	// RoleAdmin adalah role administrator sistem.
	RoleAdmin = "admin"
	// RoleMember adalah role penulis (anggota) biasa.
	RoleMember = "member"
)

// User adalah model tabel users.
type User struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(100);not null" json:"name"`
	Username  string    `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	Email     string    `gorm:"type:varchar(150);uniqueIndex;not null" json:"email"`
	Password  string    `gorm:"type:varchar(255);not null" json:"-"`
	Role      string    `gorm:"type:varchar(20);not null;default:member" json:"role"`
	Bio       string    `gorm:"type:text" json:"bio"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RegisterRequest adalah payload pendaftaran penulis baru. Name dan email opsional:
// name bawaan = username, email bawaan = <username>@delcom.org.
type RegisterRequest struct {
	Name     string `json:"name" binding:"max=100" example:"Budi Santoso"`
	Username string `json:"username" binding:"required,min=3,max=50" example:"budi"`
	Email    string `json:"email" binding:"omitempty,email,max=150" example:"budi@example.com"`
	Password string `json:"password" binding:"required,min=6,max=72" example:"rahasia123"`
}

// LoginRequest adalah payload login. Identitas boleh dikirim lewat field username atau email.
type LoginRequest struct {
	Username string `json:"username" example:"admin"`
	Email    string `json:"email" example:""`
	Password string `json:"password" binding:"required" example:"admin"`
}

// Identifier mengembalikan username (atau email jika username kosong) yang dipakai login.
func (r LoginRequest) Identifier() string {
	if login := strings.TrimSpace(r.Username); login != "" {
		return login
	}
	return strings.TrimSpace(r.Email)
}

// UpdateProfileRequest adalah payload pembaruan profil. Seluruh field opsional;
// hanya field yang dikirim yang diperbarui.
type UpdateProfileRequest struct {
	Name     string  `json:"name" binding:"max=100" example:"Budi Santoso"`
	Email    string  `json:"email" binding:"omitempty,email,max=150" example:"budi@example.com"`
	Bio      *string `json:"bio" binding:"omitempty,max=1000" example:"Penulis teknologi"`
	Password string  `json:"password" binding:"omitempty,min=6,max=72" example:""`
}

// LoginResponse adalah data respons login berhasil.
type LoginResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
