package repository

import (
	"inkwell-backend-V2.0/internal/db"
	"inkwell-backend-V2.0/internal/model"
)

type UserRepository interface {
	CreateUser(user *model.User) error
	GetUserByEmail(email string) (*model.User, error)
	GetAllUsers() ([]model.User, error)
}

type userRepository struct{}

func NewUserRepository() UserRepository {
	return &userRepository{}
}

func (r *userRepository) CreateUser(user *model.User) error {
	return db.GetDB().Create(user).Error
}

func (r *userRepository) GetUserByEmail(email string) (*model.User, error) {
	var user model.User
	err := db.GetDB().Where("email = ?", email).First(&user).Error
	return &user, err
}

func (r *userRepository) GetAllUsers() ([]model.User, error) {
	var users []model.User
	err := db.GetDB().Find(&users).Error
	return users, err
}
