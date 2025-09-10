package main

//
//import (
//	"fmt"
//	"log"
//
//	"inkwell-backend-V2.0/cmd/app/internal/config"
//	"inkwell-backend-V2.0/cmd/app/internal/db"
//	"inkwell-backend-V2.0/cmd/app/internal/model"
//	"inkwell-backend-V2.0/cmd/app/internal/repository"
//)
//
//func main() {
//
//	// Load XML configuration from file.
//	cfg, err := config.LoadConfig("config.xml")
//	if err != nil {
//		log.Fatalf("failed to load config: %v", err)
//	}
//
//	// Initialize DB using the loaded config.
//	db.InitDBFromConfig(cfg)
//
//	dbConn := db.GetDB()
//	if dbConn == nil {
//		log.Fatal("Database connection failed")
//	}
//
//	questionRepo := repository.NewQuestionRepository(dbConn)
//
//	//grammarTopics := []string{
//	//	"Tenses",
//	//	"Subject-Verb Agreement",
//	//	"Active and Passive Voice",
//	//	"Direct and Indirect Speech",
//	//	"Punctuation Rules",
//	//}
//
//	questions := []model.Question{
//		// Tenses
//		{Category: "Tenses", QuestionType: "masked", MaskedSentence: "She [MASK] very tired after the long journey.", CorrectAnswer: "was"},
//		{Category: "Tenses", QuestionType: "masked", MaskedSentence: "I [MASK] to the store yesterday.", CorrectAnswer: "went"},
//		{Category: "Tenses", QuestionType: "masked", MaskedSentence: "They [MASK] playing football when it started raining.", CorrectAnswer: "were"},
//		{Category: "Tenses", QuestionType: "masked", MaskedSentence: "My brother [MASK] his homework before dinner.", CorrectAnswer: "finished"},
//		{Category: "Tenses", QuestionType: "masked", MaskedSentence: "She [MASK] seen that movie before.", CorrectAnswer: "has"},
//		{Category: "Tenses", QuestionType: "masked", MaskedSentence: "I will call you as soon as I [MASK] home.", CorrectAnswer: "get"},
//		{Category: "Tenses", QuestionType: "masked", MaskedSentence: "We [MASK] waiting for the bus.", CorrectAnswer: "are"},
//		{Category: "Tenses", QuestionType: "masked", MaskedSentence: "He [MASK] a lot of experience in this field.", CorrectAnswer: "has"},
//		{Category: "Tenses", QuestionType: "masked", MaskedSentence: "They [MASK] to the museum last weekend.", CorrectAnswer: "went"},
//		{Category: "Tenses", QuestionType: "masked", MaskedSentence: "She [MASK] English fluently.", CorrectAnswer: "speaks"},
//
//		// Subject-Verb Agreement
//		{Category: "Subject-Verb Agreement", QuestionType: "error_correction", ErrorSentence: "She go to school every day.", CorrectAnswer: "She goes to school every day."},
//		{Category: "Subject-Verb Agreement", QuestionType: "error_correction", ErrorSentence: "I has a book.", CorrectAnswer: "I have a book."},
//		{Category: "Subject-Verb Agreement", QuestionType: "error_correction", ErrorSentence: "They was at the party last night.", CorrectAnswer: "They were at the party last night."},
//		{Category: "Subject-Verb Agreement", QuestionType: "error_correction", ErrorSentence: "The dog bark at the stranger.", CorrectAnswer: "The dog barks at the stranger."},
//		{Category: "Subject-Verb Agreement", QuestionType: "error_correction", ErrorSentence: "He don't know the answer.", CorrectAnswer: "He doesn't know the answer."},
//		{Category: "Subject-Verb Agreement", QuestionType: "error_correction", ErrorSentence: "She have three brothers.", CorrectAnswer: "She has three brothers."},
//		{Category: "Subject-Verb Agreement", QuestionType: "error_correction", ErrorSentence: "The cat sleep on the mat.", CorrectAnswer: "The cat sleeps on the mat."},
//		{Category: "Subject-Verb Agreement", QuestionType: "error_correction", ErrorSentence: "We was watching TV last night.", CorrectAnswer: "We were watching TV last night."},
//		{Category: "Subject-Verb Agreement", QuestionType: "error_correction", ErrorSentence: "My friend play football.", CorrectAnswer: "My friend plays football."},
//		{Category: "Subject-Verb Agreement", QuestionType: "error_correction", ErrorSentence: "She don't like coffee.", CorrectAnswer: "She doesn't like coffee."},
//
//		// Active and Passive Voice
//		{Category: "Active and Passive Voice", QuestionType: "error_correction", ErrorSentence: "The cake was bake by my mother.", CorrectAnswer: "The cake was baked by my mother."},
//		{Category: "Active and Passive Voice", QuestionType: "error_correction", ErrorSentence: "He was given a gift by me.", CorrectAnswer: "I gave him a gift."},
//		{Category: "Active and Passive Voice", QuestionType: "error_correction", ErrorSentence: "The book was read by the student.", CorrectAnswer: "The student read the book."},
//		{Category: "Active and Passive Voice", QuestionType: "error_correction", ErrorSentence: "A song was sung by her.", CorrectAnswer: "She sang a song."},
//		{Category: "Active and Passive Voice", QuestionType: "error_correction", ErrorSentence: "The project was completed by us.", CorrectAnswer: "We completed the project."},
//		{Category: "Active and Passive Voice", QuestionType: "error_correction", ErrorSentence: "A letter is being written by John.", CorrectAnswer: "John is writing a letter."},
//		{Category: "Active and Passive Voice", QuestionType: "error_correction", ErrorSentence: "The homework was finished by me.", CorrectAnswer: "I finished the homework."},
//		{Category: "Active and Passive Voice", QuestionType: "error_correction", ErrorSentence: "The window was broken by the wind.", CorrectAnswer: "The wind broke the window."},
//		{Category: "Active and Passive Voice", QuestionType: "error_correction", ErrorSentence: "A decision was made by the manager.", CorrectAnswer: "The manager made a decision."},
//		{Category: "Active and Passive Voice", QuestionType: "error_correction", ErrorSentence: "The story was told by my grandmother.", CorrectAnswer: "My grandmother told the story."},
//
//		// Direct and Indirect Speech
//		{Category: "Direct and Indirect Speech", QuestionType: "error_correction", ErrorSentence: "He said, 'I am happy.'", CorrectAnswer: "He said that he was happy."},
//		{Category: "Direct and Indirect Speech", QuestionType: "error_correction", ErrorSentence: "She said, 'I will call you tomorrow.'", CorrectAnswer: "She said that she would call me the next day."},
//		{Category: "Direct and Indirect Speech", QuestionType: "error_correction", ErrorSentence: "John said, 'I have finished my homework.'", CorrectAnswer: "John said that he had finished his homework."},
//		{Category: "Direct and Indirect Speech", QuestionType: "error_correction", ErrorSentence: "She said, 'I can swim.'", CorrectAnswer: "She said that she could swim."},
//		{Category: "Direct and Indirect Speech", QuestionType: "error_correction", ErrorSentence: "They said, 'We are going to the market.'", CorrectAnswer: "They said that they were going to the market."},
//		{Category: "Direct and Indirect Speech", QuestionType: "error_correction", ErrorSentence: "He said, 'I don't like coffee.'", CorrectAnswer: "He said that he didn't like coffee."},
//		{Category: "Direct and Indirect Speech", QuestionType: "error_correction", ErrorSentence: "She said, 'I must go now.'", CorrectAnswer: "She said that she had to go then."},
//		{Category: "Direct and Indirect Speech", QuestionType: "error_correction", ErrorSentence: "He said, 'I saw her yesterday.'", CorrectAnswer: "He said that he had seen her the day before."},
//		{Category: "Direct and Indirect Speech", QuestionType: "error_correction", ErrorSentence: "She said, 'I have never been to Paris.'", CorrectAnswer: "She said that she had never been to Paris."},
//		{Category: "Direct and Indirect Speech", QuestionType: "error_correction", ErrorSentence: "They said, 'We will help you tomorrow.'", CorrectAnswer: "They said that they would help me the next day."},
//
//		// Punctuation Rules
//		{Category: "Punctuation Rules", QuestionType: "error_correction", ErrorSentence: "Lets eat, grandma!", CorrectAnswer: "Let's eat, Grandma!"},
//		{Category: "Punctuation Rules", QuestionType: "error_correction", ErrorSentence: "Its a beautiful day.", CorrectAnswer: "It's a beautiful day."},
//		{Category: "Punctuation Rules", QuestionType: "error_correction", ErrorSentence: "She said I am happy.", CorrectAnswer: "She said, 'I am happy.'"},
//		{Category: "Punctuation Rules", QuestionType: "error_correction", ErrorSentence: "The book title is Harry Potter.", CorrectAnswer: "The book title is *Harry Potter*."},
//		{Category: "Punctuation Rules", QuestionType: "error_correction", ErrorSentence: "I have two sisters, Lisa and Anna.", CorrectAnswer: "I have two sisters: Lisa and Anna."},
//		{Category: "Punctuation Rules", QuestionType: "error_correction", ErrorSentence: "Hello my name is John.", CorrectAnswer: "Hello, my name is John."},
//		{Category: "Punctuation Rules", QuestionType: "error_correction", ErrorSentence: "Dont forget your keys.", CorrectAnswer: "Don't forget your keys."},
//		{Category: "Punctuation Rules", QuestionType: "error_correction", ErrorSentence: "Where is my phone?", CorrectAnswer: "Where is my phone?"},
//		{Category: "Punctuation Rules", QuestionType: "error_correction", ErrorSentence: "She said Im tired.", CorrectAnswer: "She said, 'I'm tired.'"},
//		{Category: "Punctuation Rules", QuestionType: "error_correction", ErrorSentence: "Can you help me, he asked.", CorrectAnswer: `"Can you help me?" he asked.`},
//	}
//	// Insert questions
//	for _, question := range questions {
//		err := questionRepo.CreateQuestion(&question)
//		if err != nil {
//			log.Printf("Failed to insert question: %v", err)
//		} else {
//			fmt.Println("Inserted question:", question.Category, "->", question.CorrectAnswer)
//		}
//	}
//
//	fmt.Println("Database seeding completed successfully!")
//}
