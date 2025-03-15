package repository

import (
	"inkwell-backend-V2.0/internal/db"
	"inkwell-backend-V2.0/internal/model"
)

type StoryRepository interface {
	GetStories() ([]model.Story, error)
}

type storyRepository struct{}

func NewStoryRepository() StoryRepository {
	return &storyRepository{}
}

func (r *storyRepository) GetStories() ([]model.Story, error) {
	var stories []model.Story
	err := db.GetDB().Find(&stories).Error
	return stories, err
}
