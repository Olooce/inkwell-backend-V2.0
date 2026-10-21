package service

import (
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
)

type UserService interface {
	GetUser(id uint) (*model.User, error)
}

type userService struct {
	userRepo repository.UserRepository
}

func NewUserService(userRepo repository.UserRepository) UserService {
	return &userService{userRepo: userRepo}
}

// GetUser returns a user's profile
func (s *userService) GetUser(id uint) (*model.User, error) {
	user, err := s.userRepo.GetUserByID(id)
	if err != nil {
		return nil, err
	}
	user.Password = ""
	return user, nil
}
