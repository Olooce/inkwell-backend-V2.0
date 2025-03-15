package model

import "time"

type User struct {
	ID                         uint      `json:"id" gorm:"primaryKey"`
	Username                   string    `json:"username"`
	Email                      string    `json:"email"`
	Password                   string    `json:"password,omitempty"` // Exclude from JSON responses
	FirstName                  string    `json:"first_name"`
	LastName                   string    `json:"last_name"`
	InitialAssessmentCompleted bool      `json:"initial_assessment_completed" gorm:"-"`
	CreatedAt                  time.Time `json:"created_at"`
	UpdatedAt                  time.Time `json:"updated_at"`
}

type Assessment struct {
	ID                   uint       `json:"id" gorm:"primaryKey"`
	UserID               uint       `json:"user_id" gorm:"not null"`
	SessionID            string     `json:"session_id" gorm:"not null;unique"`
	Title                string     `json:"title" gorm:"not null"`
	Description          string     `json:"description"`
	Score                int        `json:"score" gorm:"not null"`
	Category             string     `json:"category"`
	Status               string     `json:"status" gorm:"default:'pending'"` // pending, completed
	Questions            []Question `json:"questions" gorm:"foreignKey:AssessmentID"`
	CurrentQuestionIndex int        `json:"current_question_index" gorm:"default:0"`
	Answers              []Answer   `json:"answers" gorm:"foreignKey:AssessmentID"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type Question struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	AssessmentID  uint      `json:"assessment_id"`
	Text          string    `json:"text" gorm:"not null"`
	Choices       string    `json:"choices"` // JSON array of choices
	CorrectAnswer string    `json:"correct_answer" gorm:"not null"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Answer struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	AssessmentID uint      `json:"assessment_id"`
	QuestionID   uint      `json:"question_id"`
	UserID       uint      `json:"user_id"`
	Answer       string    `json:"answer"`
	IsCorrect    bool      `json:"is_correct"`
	Feedback     string    `json:"feedback"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type Story struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
