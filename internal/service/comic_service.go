package service

import (
	"encoding/base64"
	"fmt"
	"inkwell-backend-V2.0/utilities"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
)

type ComicService interface {
	GenerateComic(storyID uint) error
}

type comicService struct {
	storyRepo repository.StoryRepository
}

func NewComicService(storyRepo repository.StoryRepository) ComicService {
	return &comicService{storyRepo: storyRepo}
}

func InitComicEventListeners(storyRepo repository.StoryRepository) {
	utilities.GlobalEventBus.Subscribe("story_completed", func(data interface{}) {
		storyID, ok := data.(uint)
		if !ok {
			fmt.Println("Invalid story ID received for comic generation")
			return
		}

		log.Printf("[Event] Story completed: Generating comic for story ID %d", storyID)
		comicService := NewComicService(storyRepo)
		err := comicService.GenerateComic(storyID)
		if err != nil {
			log.Printf("Error generating comic for story %d: %v", storyID, err)
		}
	})
}

func encodeImageToBase64(imgPath string) (string, error) {
	imgData, err := os.ReadFile(imgPath)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(imgData), nil
}

func (s *comicService) GenerateComic(storyID uint) error {
	log.Printf("[Start] Generating comic for story ID %d", storyID)

	story, err := s.storyRepo.GetStoryByID(storyID)
	if err != nil {
		return fmt.Errorf("failed to fetch story: %w", err)
	}
	log.Printf("Fetched story: ID %d, Title: %s", story.ID, story.Title)

	sentences, err := s.storyRepo.GetSentencesByStory(storyID)
	if err != nil {
		return fmt.Errorf("failed to fetch sentences: %w", err)
	}
	log.Printf("Fetched %d sentences for story ID %d", len(sentences), storyID)

	if _, err := os.Stat("working/comics"); os.IsNotExist(err) {
		log.Println("Creating missing directory: working/comics")
		err := os.MkdirAll("working/comics", os.ModePerm)
		if err != nil {
			return err
		}
	}

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetFont("Arial", "B", 16)
	pdf.AddPage()
	pdf.Cell(40, 10, story.Title)
	pdf.Ln(20)

	for _, sentence := range sentences {
		pdf.SetFont("Arial", "", 12)
		imgPath := sentence.ImageURL
		if !filepath.IsAbs(imgPath) {
			imgPath = filepath.Join("working", imgPath)
		}

		log.Printf("Processing image: %s", imgPath)
		if sentence.ImageURL != "" {
			if _, err := os.Stat(imgPath); err == nil {
				log.Printf("Image found: %s", imgPath)
				base64Img, err := encodeImageToBase64(imgPath)
				if err == nil {
					pdf.RegisterImageOptionsReader(imgPath, gofpdf.ImageOptions{ImageType: "JPG"}, base64.NewDecoder(base64.StdEncoding, strings.NewReader(base64Img)))
					pdf.Image(imgPath, 10, pdf.GetY(), 180, 100, false, "", 0, "")
				} else {
					log.Printf("Error encoding image to Base64: %v", err)
				}
				pdf.Ln(105)
			} else {
				log.Printf("Image not found: %s, adding empty box", imgPath)
				pdf.Rect(10, pdf.GetY(), 180, 100, "D")
				pdf.Ln(105)
			}
		} else {
			log.Println("No image URL found, adding empty box")
			pdf.Rect(10, pdf.GetY(), 180, 100, "D")
			pdf.Ln(105)
		}
		pdf.MultiCell(0, 10, sentence.CorrectedText, "", "L", false)
		pdf.Ln(10)
	}

	outputPath := filepath.Join("working/comics", fmt.Sprintf("comic_%d.pdf", storyID))
	log.Printf("Saving comic to: %s", outputPath)
	err = pdf.OutputFileAndClose(outputPath)
	if err != nil {
		return fmt.Errorf("failed to save PDF: %w", err)
	}

	comic := model.Comic{
		UserID:      story.UserID,
		Title:       story.Title,
		StoryID:     story.ID,
		Thumbnail:   generateThumbnail(sentences),
		ViewURL:     filepath.Join("comics", fmt.Sprintf("comic_%d.pdf", storyID)),
		DownloadURL: filepath.Join("comics", fmt.Sprintf("comic_%d.pdf", storyID)),
		DoneOn:      time.Now(),
	}

	err = s.storyRepo.SaveComic(&comic)
	if err != nil {
		return fmt.Errorf("failed to save comic record: %w", err)
	}

	log.Printf("Successfully generated and saved comic for story ID %d", storyID)
	return nil
}

func generateThumbnail(sentences []model.Sentence) string {
	for _, sentence := range sentences {
		if sentence.ImageURL != "" {
			log.Printf("Thumbnail selected: %s", sentence.ImageURL)
			return sentence.ImageURL
		}
	}
	log.Println("No valid thumbnail found, returning empty string")
	return ""
}

func GenerateMissingComics(storyRepo repository.StoryRepository) {
	stories, err := storyRepo.GetAllStoriesWithoutComics()
	if err != nil {
		log.Printf("Error fetching stories without comics: %v", err)
		return
	}

	log.Printf("Found %d stories without comics", len(stories))

	if len(stories) == 0 {
		log.Println("All stories already have comics.")
		return
	}

	comicService := NewComicService(storyRepo)

	for _, story := range stories {
		log.Printf("Generating comic for story ID: %d, Title: %s", story.ID, story.Title)
		err := comicService.GenerateComic(story.ID)
		if err != nil {
			log.Printf("Failed to generate comic for story ID %d: %v", story.ID, err)
		} else {
			log.Printf("Successfully generated comic for story ID %d", story.ID)
		}
	}
}
