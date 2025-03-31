package model

import "time"

type User struct {
	ID                         uint      `json:"id" gorm:"primaryKey"`
	Username                   string    `json:"username"`
	Email                      string    `json:"email"`
	Password                   string    `json:"password,omitempty"` // Exclude from JSON responses
	FirstName                  string    `json:"first_name"`
	LastName                   string    `json:"last_name"`
	InitialAssessmentCompleted bool      `json:"initial_assessment_completed" gorm:"default:false"`
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
	Category             string     `json:"category" gorm:"not null"`
	Status               string     `json:"status" gorm:"default:'pending'"` // pending, completed
	CurrentQuestionIndex int        `json:"current_question_index" gorm:"default:0"`
	Answers              []Answer   `json:"answers" gorm:"foreignKey:AssessmentID"`
	Questions            []Question `json:"questions" gorm:"many2many:assessment_questions"`
	CreatedAt            time.Time  `json:"created_at"`
	UpdatedAt            time.Time  `json:"updated_at"`
}

type Question struct {
	ID             uint   `json:"id" gorm:"primaryKey"`
	Category       string `json:"category" gorm:"not null"`
	QuestionType   string `json:"question_type" gorm:"type:varchar(20);not null"` // "masked" or "error_correction"
	MaskedSentence string `json:"masked_sentence" gorm:"type:text"`
	ErrorSentence  string `json:"error_sentence" gorm:"type:text"`
	CorrectAnswer  string `json:"correct_answer" gorm:"type:text;not null"`
}

type Answer struct {
	ID           uint      `json:"id" gorm:"primaryKey"`
	AssessmentID uint      `json:"assessment_id"`
	SessionID    string    `json:"session_id"`
	QuestionID   uint      `json:"question_id"`
	UserID       uint      `json:"user_id"`
	Answer       string    `json:"answer"`
	IsCorrect    bool      `json:"is_correct"`
	Feedback     string    `json:"feedback"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type AnswerResponse struct {
	IsCorrect bool   `json:"is_correct"`
	Feedback  string `json:"feedback"`
}

type Story struct {
	ID               uint      `json:"id" gorm:"primaryKey"`
	UserID           uint      `json:"user_id"`
	Title            string    `json:"title"`
	Content          string    `json:"content"`
	PerformanceScore int       `json:"performance_score" gorm:"default:0"`
	Analysis         string    `json:"analysis,omitempty"`
	Tips             string    `json:"tips,omitempty"`
	Status           string    `json:"status" gorm:"default:'in_progress'"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

type Sentence struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	StoryID       uint      `json:"story_id"`
	OriginalText  string    `json:"original_text"`
	CorrectedText string    `json:"corrected_text"`
	Feedback      string    `json:"feedback"`
	ImageURL      string    `json:"image_url"`
	CreatedAt     time.Time `json:"created_at"`
}
type Comic struct {
	ID          uint      `json:"id" gorm:"primaryKey"`
	UserID      uint      `json:"user_id"`
	StoryID     uint      `json:"story_id" gorm:"not null;index"`
	Title       string    `json:"title"`
	Thumbnail   string    `json:"thumbnail"`
	ViewURL     string    `json:"view_url"`
	DownloadURL string    `json:"download_url"`
	DoneOn      time.Time `json:"done_on"`
}
