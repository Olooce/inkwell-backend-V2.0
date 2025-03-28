

package service

import (
	"fmt"

	"gorm.io/gorm"
	"inkwell-backend-V2.0/internal/model"
)

// ProgressData holds the metrics for the progress report.
type ProgressData struct {
	InitialProgress map[string]interface{} ` json:"initial_progress" `
	CurrentProgress map[string]interface{} ` json:"current_progress" `
}

// GenerateProgressData computes the progress data for a given user.
func GenerateProgressData(db *gorm.DB, userID uint) (*ProgressData, error) {
	// Query for the initial (oldest completed) assessment.
	var initialAssessment model.Assessment
	if err := db.Where("user_id = ? AND status = ?", userID, "completed").
		Order("created_at asc").
		First(&initialAssessment).Error; err != nil {
		return nil, fmt.Errorf("failed to get initial assessment: %w", err)
	}

	// Query for the latest (most recent completed) assessment.
	var latestAssessment model.Assessment
	if err := db.Where("user_id = ? AND status = ?", userID, "completed").
		Order("created_at desc").
		First(&latestAssessment).Error; err != nil {
		return nil, fmt.Errorf("failed to get latest assessment: %w", err)
	}

	// Calculate improvement as the difference between the latest and the initial scores.
	improvement := float64(latestAssessment.Score - initialAssessment.Score)

	// Retrieve stories
	var stories []model.Story
	if err := db.Where("user_id = ?", userID).
		Find(&stories).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch stories: %w", err)
	}
	totalStories := len(stories)
	var totalPerfScore int
	for _, story := range stories {
		totalPerfScore += story.PerformanceScore
	}
	var avgPerformance float64
	if totalStories > 0 {
		avgPerformance = float64(totalPerfScore) / float64(totalStories)
	} else {
		avgPerformance = 0
	}

	// Count total sentences by joining stories and sentences.
	var totalSentences int64
	if err := db.Table("sentences").
		Joins("JOIN stories ON sentences.story_id = stories.id").
		Where("stories.user_id = ?", userID).
		Count(&totalSentences).Error; err != nil {
		return nil, fmt.Errorf("failed to count sentences: %w", err)
	}

	// Compute overall accuracy from the Answer table.
	var totalAnswers int64
	var correctAnswers int64
	if err := db.Model(&model.Answer{}).
		Where("user_id = ?", userID).
		Count(&totalAnswers).Error; err != nil {
		return nil, fmt.Errorf("failed to count answers: %w", err)
	}
	if err := db.Model(&model.Answer{}).
		Where("user_id = ? AND is_correct = ?", userID, true).
		Count(&correctAnswers).Error; err != nil {
		return nil, fmt.Errorf("failed to count correct answers: %w", err)
	}
	var accuracy float64
	if totalAnswers > 0 {
		accuracy = (float64(correctAnswers) / float64(totalAnswers)) * 100
	} else {
		accuracy = 0
	}

	initialProgress := map[string]interface{}{
		"level": "BEGINNER",
		"scores": map[string]float64{
			"masked":           float64(initialAssessment.Score),
			"error_correction": float64(initialAssessment.Score),
		},
	}

	currentProgress := map[string]interface{}{
		"improvement": improvement,
		"stats": map[string]interface{}{
			"total_stories":       totalStories,
			"total_sentences":     totalSentences,
			"accuracy":            accuracy,
			"average_performance": avgPerformance,
		},
	}

	return &ProgressData{
		InitialProgress: initialProgress,
		CurrentProgress: currentProgress,
	}, nil
}