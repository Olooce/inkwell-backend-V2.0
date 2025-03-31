package repository

import (
	"gorm.io/gorm"
	"inkwell-backend-V2.0/cmd/internal/model"
)

type QuestionRepository interface {
	CreateQuestion(question *model.Question) error
	GetAllQuestions() ([]model.Question, error)
}

type questionRepository struct {
	db *gorm.DB
}

//goland:noinspection ALL
func NewQuestionRepository(db *gorm.DB) QuestionRepository {
	return &questionRepository{db: db}
}

func (r *questionRepository) CreateQuestion(question *model.Question) error {
	return r.db.Create(question).Error
}

func (r *questionRepository) GetAllQuestions() ([]model.Question, error) {
	var questions []model.Question
	err := r.db.Find(&questions).Error
	return questions, err
}
