package main

import (
	"fmt"
	"inkwell-backend-V2.0/internal/config"
	"inkwell-backend-V2.0/internal/db"
	"inkwell-backend-V2.0/internal/model"
	"inkwell-backend-V2.0/internal/repository"
	"log"
)

func main() {

	// Load XML configuration from file.
	cfg, err := config.LoadConfig("config.xml")
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}

	// Initialize DB using the loaded config.
	db.InitDBFromConfig(cfg)
	
	dbConn := db.GetDB()
	if dbConn == nil {
		log.Fatal("Database connection failed")
	}

	questionRepo := repository.NewQuestionRepository(dbConn)

	questions := []model.Question{
		// Masked Word Questions
		{QuestionType: "masked", MaskedSentence: "She [MASK] very tired after the long journey.", CorrectAnswer: "was"},
		{QuestionType: "masked", MaskedSentence: "I [MASK] to the store yesterday.", CorrectAnswer: "went"},
		{QuestionType: "masked", MaskedSentence: "They [MASK] playing football when it started raining.", CorrectAnswer: "were"},
		{QuestionType: "masked", MaskedSentence: "This book is [MASK] than that one.", CorrectAnswer: "better"},
		{QuestionType: "masked", MaskedSentence: "John and Mary [MASK] going to the cinema tonight.", CorrectAnswer: "are"},
		{QuestionType: "masked", MaskedSentence: "My brother [MASK] his homework before dinner.", CorrectAnswer: "finished"},
		{QuestionType: "masked", MaskedSentence: "She is the [MASK] student in the class.", CorrectAnswer: "best"},
		{QuestionType: "masked", MaskedSentence: "He runs [MASK] than his friend.", CorrectAnswer: "faster"},
		{QuestionType: "masked", MaskedSentence: "The weather is [MASK] today than it was yesterday.", CorrectAnswer: "colder"},
		{QuestionType: "masked", MaskedSentence: "You [MASK] not talk loudly in the library.", CorrectAnswer: "should"},

		// Error Correction Questions
		{QuestionType: "error_correction", ErrorSentence: "She go to school every day.", CorrectAnswer: "She goes to school every day."},
		{QuestionType: "error_correction", ErrorSentence: "I has a book.", CorrectAnswer: "I have a book."},
		{QuestionType: "error_correction", ErrorSentence: "They was at the party last night.", CorrectAnswer: "They were at the party last night."},
		{QuestionType: "error_correction", ErrorSentence: "The dog bark at the stranger.", CorrectAnswer: "The dog barks at the stranger."},
		{QuestionType: "error_correction", ErrorSentence: "He did not went to the market.", CorrectAnswer: "He did not go to the market."},
		{QuestionType: "error_correction", ErrorSentence: "My father is more taller than my uncle.", CorrectAnswer: "My father is taller than my uncle."},
		{QuestionType: "error_correction", ErrorSentence: "There is many books on the table.", CorrectAnswer: "There are many books on the table."},
		{QuestionType: "error_correction", ErrorSentence: "The childrens are playing outside.", CorrectAnswer: "The children are playing outside."},
		{QuestionType: "error_correction", ErrorSentence: "She don't like coffee.", CorrectAnswer: "She doesn't like coffee."},
		{QuestionType: "error_correction", ErrorSentence: "We was watching TV last night.", CorrectAnswer: "We were watching TV last night."},

		// More questions (masked & error correction)
		{QuestionType: "masked", MaskedSentence: "He is the [MASK] person I have ever met.", CorrectAnswer: "kindest"},
		{QuestionType: "masked", MaskedSentence: "She [MASK] seen that movie before.", CorrectAnswer: "has"},
		{QuestionType: "masked", MaskedSentence: "I will call you as soon as I [MASK] home.", CorrectAnswer: "get"},
		{QuestionType: "masked", MaskedSentence: "This soup tastes [MASK] delicious.", CorrectAnswer: "very"},
		{QuestionType: "masked", MaskedSentence: "You should [MASK] your vegetables.", CorrectAnswer: "eat"},
		{QuestionType: "masked", MaskedSentence: "We [MASK] waiting for the bus.", CorrectAnswer: "are"},
		{QuestionType: "masked", MaskedSentence: "My friend [MASK] from Japan.", CorrectAnswer: "is"},
		{QuestionType: "masked", MaskedSentence: "He [MASK] a lot of experience in this field.", CorrectAnswer: "has"},
		{QuestionType: "masked", MaskedSentence: "They [MASK] to the museum last weekend.", CorrectAnswer: "went"},
		{QuestionType: "masked", MaskedSentence: "She [MASK] English fluently.", CorrectAnswer: "speaks"},

		{QuestionType: "error_correction", ErrorSentence: "The cat sleep on the mat.", CorrectAnswer: "The cat sleeps on the mat."},
		{QuestionType: "error_correction", ErrorSentence: "He don't know the answer.", CorrectAnswer: "He doesn't know the answer."},
		{QuestionType: "error_correction", ErrorSentence: "This is the more interesting book I've read.", CorrectAnswer: "This is the most interesting book I've read."},
		{QuestionType: "error_correction", ErrorSentence: "My car is more faster than yours.", CorrectAnswer: "My car is faster than yours."},
		{QuestionType: "error_correction", ErrorSentence: "She have three brothers.", CorrectAnswer: "She has three brothers."},
		{QuestionType: "error_correction", ErrorSentence: "It is raining since morning.", CorrectAnswer: "It has been raining since morning."},
		{QuestionType: "error_correction", ErrorSentence: "I has been working here for five years.", CorrectAnswer: "I have been working here for five years."},
		{QuestionType: "error_correction", ErrorSentence: "There was a big traffic jam in the morning.", CorrectAnswer: "There was a big traffic jam this morning."},
		{QuestionType: "error_correction", ErrorSentence: "The man which lives next door is a doctor.", CorrectAnswer: "The man who lives next door is a doctor."},
		{QuestionType: "error_correction", ErrorSentence: "He run five kilometers every morning.", CorrectAnswer: "He runs five kilometers every morning."},
	}

	// Insert questions
	for _, question := range questions {
		err := questionRepo.CreateQuestion(&question)
		if err != nil {
			log.Printf("Failed to insert question: %v", err)
		} else {
			fmt.Println("Inserted question:", question.QuestionType, "->", question.CorrectAnswer)
		}
	}

	fmt.Println("Database seeding completed successfully!")
}
