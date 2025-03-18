package repository

import (
	"inkwell-backend-V2.0/internal/db"
	"inkwell-backend-V2.0/internal/model"
)

type StoryRepository interface {
	GetStories() ([]model.Story, error)
	GetStoryByID(storyID uint) (*model.Story, error)
	CreateStory(story *model.Story) error
	CreateSentence(sentence *model.Sentence) error
	CompleteStory(storyID uint) error
	GetCurrentStoryByUser(userID uint) (*model.Story, error)
	GetSentenceCount(storyID uint) (int, error)
	GetSentencesByStory(storyID uint) ([]model.Sentence, error)
	SaveComic(comic *model.Comic) error
	GetComicsByUser(userID uint) ([]model.Comic, error)
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

func (r *storyRepository) GetStoryByID(storyID uint) (*model.Story, error) {
	var story model.Story
	err := db.GetDB().Where("id = ?", storyID).First(&story).Error
	return &story, err
}

func (r *storyRepository) CreateStory(story *model.Story) error {
	return db.GetDB().Create(story).Error
}

func (r *storyRepository) CreateSentence(sentence *model.Sentence) error {
	return db.GetDB().Create(sentence).Error
}

func (r *storyRepository) CompleteStory(storyID uint) error {
	// Update the story status to "completed"
	return db.GetDB().Model(&model.Story{}).Where("id = ?", storyID).Update("status", "completed").Error
}

func (r *storyRepository) GetCurrentStoryByUser(userID uint) (*model.Story, error) {
	var story model.Story
	err := db.GetDB().Where("user_id = ? AND status = ?", userID, "in_progress").First(&story).Error
	return &story, err
}

func (r *storyRepository) GetSentenceCount(storyID uint) (int, error) {
	var count int64
	err := db.GetDB().Model(&model.Sentence{}).Where("story_id = ?", storyID).Count(&count).Error
	return int(count), err
}

func (r *storyRepository) GetSentencesByStory(storyID uint) ([]model.Sentence, error) {
	var sentences []model.Sentence
	err := db.GetDB().Where("story_id = ?", storyID).Find(&sentences).Error
	return sentences, err
}

func (r *storyRepository) SaveComic(comic *model.Comic) error {
	return db.GetDB().Create(comic).Error
}

func (r *storyRepository) GetComicsByUser(userID uint) ([]model.Comic, error) {
	var comics []model.Comic
	err := db.GetDB().Where("user_id = ?", userID).Find(&comics).Error
	if err != nil {
		return nil, err
	}
	return comics, nil
}
