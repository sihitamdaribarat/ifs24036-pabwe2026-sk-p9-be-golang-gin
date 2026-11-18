package auth

import "gorm.io/gorm"

// Repository adalah akses data tabel users.
type Repository interface {
	Create(user *User) error
	Update(user *User) error
	FindByID(id uint) (*User, error)
	FindByLogin(login string) (*User, error)
	CountByUsername(username string) (int64, error)
	CountByEmail(email string, excludeID uint) (int64, error)
}

type repository struct {
	db *gorm.DB
}

// NewRepository membuat Repository berbasis GORM.
func NewRepository(db *gorm.DB) Repository {
	return &repository{db: db}
}

func (r *repository) Create(user *User) error {
	return r.db.Create(user).Error
}

func (r *repository) Update(user *User) error {
	return r.db.Save(user).Error
}

func (r *repository) FindByID(id uint) (*User, error) {
	var user User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *repository) FindByLogin(login string) (*User, error) {
	var user User
	if err := r.db.Where("username = ? OR email = ?", login, login).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *repository) CountByUsername(username string) (int64, error) {
	var count int64
	err := r.db.Model(&User{}).Where("username = ?", username).Count(&count).Error
	return count, err
}

func (r *repository) CountByEmail(email string, excludeID uint) (int64, error) {
	var count int64
	err := r.db.Model(&User{}).Where("email = ? AND id <> ?", email, excludeID).Count(&count).Error
	return count, err
}
