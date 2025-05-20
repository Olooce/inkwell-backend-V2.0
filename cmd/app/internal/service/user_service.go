package service

import (
	"inkwell-backend-V2.0/cmd/app/internal/model"
	"inkwell-backend-V2.0/cmd/app/internal/repository"
)

type UserService interface {
	GetAllUsers() ([]model.User, error)
}

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

func (s *userService) GetAllUsers() ([]model.User, error) {
	return s.userRepo.GetAllUsers()
}
