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
	ID        uint      `json:"id" gorm:"primaryKey"`
	UserID    uint      `json:"user_id"`
	Score     int       `json:"score"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Story struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
