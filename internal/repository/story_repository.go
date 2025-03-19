package repository

import (
	"inkwell-backend-V2.0/internal/db"
	"inkwell-backend-V2.0/internal/db/query"
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
	GetAllStoriesWithoutComics() ([]model.Story, error)
}

type storyRepository struct {
	executor *db.QueryExecutor
}

func NewStoryRepository() StoryRepository {
	return &storyRepository{
		executor: db.NewQueryExecutor(db.GetDB()),
	}
}

func (r *storyRepository) GetStories() ([]model.Story, error) {
	var stories []model.Story
	qb := query.NewQueryBuilder().Select("*").From("stories")
	queryStr, args := qb.Build()
	err := r.executor.RawQuery(queryStr, args...).Scan(&stories).Error
	return stories, err
}

func (r *storyRepository) GetStoryByID(storyID uint) (*model.Story, error) {
	var story model.Story
	fp := query.NewFilterPredicate().Equal("id", storyID)
	qb := query.NewQueryBuilder().Select("*").From("stories").Where(fp.Build())
	queryStr, args := qb.Build()
	err := r.executor.RawQuery(queryStr, args...).Scan(&story).Error
	return &story, err
}

func (r *storyRepository) CreateStory(story *model.Story) error {
	// Capture returned id if needed
	_, err := r.executor.Insert("stories", map[string]interface{}{
		"title":   story.Title,
		"content": story.Content,
		"user_id": story.UserID,
		"status":  story.Status,
	})
	return err
}

func (r *storyRepository) CreateSentence(sentence *model.Sentence) error {
	// Use OriginalText (or CorrectedText) as per your design
	_, err := r.executor.Insert("sentences", map[string]interface{}{
		"story_id":       sentence.StoryID,
		"original_text":  sentence.OriginalText,
		"corrected_text": sentence.CorrectedText,
		"feedback":       sentence.Feedback,
		"image_url":      sentence.ImageURL,
	})
	return err
}

func (r *storyRepository) CompleteStory(storyID uint) error {
	conditions := map[string]interface{}{"id": storyID}
	updates := map[string]interface{}{"status": "completed"}
	return r.executor.Update("stories", conditions, updates)
}

func (r *storyRepository) GetCurrentStoryByUser(userID uint) (*model.Story, error) {
	var story model.Story
	fp := query.NewFilterPredicate().Equal("user_id", userID).And().Equal("status", "in_progress")
	qb := query.NewQueryBuilder().Select("*").From("stories").Where(fp.Build())
	queryStr, args := qb.Build()
	err := r.executor.RawQuery(queryStr, args...).Scan(&story).Error
	return &story, err
}

func (r *storyRepository) GetSentenceCount(storyID uint) (int, error) {
	var count int
	fp := query.NewFilterPredicate().Equal("story_id", storyID)
	qb := query.NewQueryBuilder().Select("COUNT(*)").From("sentences").Where(fp.Build())
	queryStr, args := qb.Build()
	err := r.executor.RawQuery(queryStr, args...).Scan(&count).Error
	return count, err
}

func (r *storyRepository) GetSentencesByStory(storyID uint) ([]model.Sentence, error) {
	var sentences []model.Sentence
	fp := query.NewFilterPredicate().Equal("story_id", storyID)
	qb := query.NewQueryBuilder().Select("*").From("sentences").Where(fp.Build())
	queryStr, args := qb.Build()
	err := r.executor.RawQuery(queryStr, args...).Scan(&sentences).Error
	return sentences, err
}

func (r *storyRepository) SaveComic(comic *model.Comic) error {
	_, err := r.executor.Insert("comics", map[string]interface{}{
		"story_id":     comic.StoryID,
		"user_id":      comic.UserID,
		"title":        comic.Title,
		"thumbnail":    comic.Thumbnail,
		"view_url":     comic.ViewURL,
		"download_url": comic.DownloadURL,
		"done_on":      comic.DoneOn,
	})
	return err
}

func (r *storyRepository) GetComicsByUser(userID uint) ([]model.Comic, error) {
	var comics []model.Comic
	fp := query.NewFilterPredicate().Equal("user_id", userID)
	qb := query.NewQueryBuilder().Select("*").From("comics").Where(fp.Build())
	queryStr, args := qb.Build()
	err := r.executor.RawQuery(queryStr, args...).Scan(&comics).Error
	return comics, err
}

func (r *storyRepository) GetAllStoriesWithoutComics() ([]model.Story, error) {
	var stories []model.Story
	err := r.executor.RawQuery(`
        SELECT * FROM stories 
        WHERE id NOT IN (SELECT DISTINCT story_id FROM comics)
    `).Scan(&stories).Error
	return stories, err
}
