package grading

import (
	"errors"
	"testing"
)

// fakeJudge records whether it was consulted and returns a fixed verdict.
type fakeJudge struct {
	accept bool
	err    error
	called bool
}

func (f *fakeJudge) JudgeCorrection(_, _, _ string) (bool, string, error) {
	f.called = true
	return f.accept, "", f.err
}

func TestGradeErrorCorrection(t *testing.T) {
	dog := Question{Type: TypeErrorCorrection, ErrorSentence: "The dog bark at the stranger.", CorrectAnswer: "The dog barks at the stranger."}
	know := Question{Type: TypeErrorCorrection, ErrorSentence: "He don't know the answer.", CorrectAnswer: "He doesn't know the answer."}
	voice := Question{Type: TypeErrorCorrection, ErrorSentence: "He was given a gift by me.", CorrectAnswer: "I gave him a gift."}

	tests := []struct {
		name       string
		q          Question
		answer     string
		judge      *fakeJudge
		want       bool
		wantJudged bool
	}{
		// The reported bug: a one-word answer must never be accepted, whatever the model says.
		{"one word rejected without judge", dog, "is", &fakeJudge{accept: true}, false, false},
		{"exact match", dog, "The dog barks at the stranger.", &fakeJudge{}, true, false},
		{"case, spacing and end punctuation ignored", dog, "  the dog barks at  the stranger ", &fakeJudge{}, true, false},
		{"contraction expanded", know, "He does not know the answer", &fakeJudge{}, true, false},
		{"curly apostrophe", know, "He doesn’t know the answer.", &fakeJudge{}, true, false},
		{"unchanged sentence rejected", dog, "The dog bark at the stranger.", &fakeJudge{accept: true}, false, false},
		{"empty rejected", dog, "   ", &fakeJudge{accept: true}, false, false},
		{"close alternative accepted by judge", voice, "I gave a gift to him.", &fakeJudge{accept: true}, true, true},
		{"close alternative rejected by judge", dog, "The dog barked at the stranger.", &fakeJudge{accept: false}, false, true},
		{"judge error is conservative", voice, "I gave a gift to him.", &fakeJudge{err: errors.New("down")}, false, true},
		{"unrelated sentence rejected without judge", dog, "I like cats.", &fakeJudge{accept: true}, false, false},
		{"mistake kept, other words changed", dog, "The dog bark at the visitor.", &fakeJudge{accept: true}, false, false},
		{"mistake kept, rewrite question still judged", voice, "I gave him a gift by me.", &fakeJudge{accept: false}, false, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Grade(tt.q, tt.answer, tt.judge)
			if got.Correct != tt.want {
				t.Errorf("Correct = %v, want %v (feedback %q)", got.Correct, tt.want, got.Feedback)
			}
			if tt.judge.called != tt.wantJudged {
				t.Errorf("judge called = %v, want %v", tt.judge.called, tt.wantJudged)
			}
			if got.Feedback == "" {
				t.Error("feedback is empty")
			}
		})
	}
}

func TestGradeErrorCorrectionNilJudge(t *testing.T) {
	q := Question{Type: TypeErrorCorrection, ErrorSentence: "He was given a gift by me.", CorrectAnswer: "I gave him a gift."}
	if Grade(q, "I gave a gift to him.", nil).Correct {
		t.Error("answer needing a judge was accepted with no judge")
	}
}

func TestGradePunctuationIsStrict(t *testing.T) {
	q := Question{Type: TypeErrorCorrection, Category: CategoryPunctuation, ErrorSentence: "Lets eat, grandma!", CorrectAnswer: "Let's eat, Grandma!"}
	if !Grade(q, "Let's eat, Grandma!", &fakeJudge{}).Correct {
		t.Error("exact punctuation answer rejected")
	}
	judge := &fakeJudge{accept: false}
	if Grade(q, "Let's eat, grandma!", judge).Correct {
		t.Error("lower-case 'grandma' accepted in a punctuation question")
	}
	if !judge.called {
		t.Error("near-miss punctuation answer should go to the judge, not be auto-accepted")
	}
}

func TestGradeMasked(t *testing.T) {
	q := Question{Type: TypeMasked, MaskedSentence: "I [MASK] to the store yesterday.", CorrectAnswer: "went"}
	for answer, want := range map[string]bool{
		"went":                           true,
		" Went. ":                        true,
		"I went to the store yesterday.": true,
		"go":                             false,
		"gone":                           false,
		"":                               false,
	} {
		if got := Grade(q, answer, nil); got.Correct != want {
			t.Errorf("Grade(%q).Correct = %v, want %v", answer, got.Correct, want)
		}
	}
}

// Every seeded question's own correct answer must pass without the judge.
func TestSeedAnswersPassDeterministically(t *testing.T) {
	seeds := []Question{
		{Type: TypeMasked, MaskedSentence: "She [MASK] seen that movie before.", CorrectAnswer: "has"},
		{Type: TypeErrorCorrection, ErrorSentence: "I has a book.", CorrectAnswer: "I have a book."},
		{Type: TypeErrorCorrection, ErrorSentence: "She said, 'I will call you tomorrow.'", CorrectAnswer: "She said that she would call me the next day."},
		{Type: TypeErrorCorrection, Category: CategoryPunctuation, ErrorSentence: "She said I am happy.", CorrectAnswer: "She said, 'I am happy.'"},
		{Type: TypeErrorCorrection, Category: CategoryPunctuation, ErrorSentence: "Can you help me, he asked.", CorrectAnswer: `"Can you help me?" he asked.`},
		{Type: TypeErrorCorrection, Category: CategoryPunctuation, ErrorSentence: "The book title is Harry Potter.", CorrectAnswer: "The book title is *Harry Potter*."},
	}
	for _, q := range seeds {
		judge := &fakeJudge{}
		if v := Grade(q, q.CorrectAnswer, judge); !v.Correct || judge.called {
			t.Errorf("%q: Correct=%v judged=%v", q.CorrectAnswer, v.Correct, judge.called)
		}
	}
}

func TestSimilarity(t *testing.T) {
	if s := Similarity("is", "the dog barks at the stranger"); s >= minSimilarity {
		t.Errorf("one-word answer similarity %.2f, want < %.2f", s, minSimilarity)
	}
	if s := Similarity("i gave a gift to him", "i gave him a gift"); s < minSimilarity {
		t.Errorf("valid alternative similarity %.2f, want >= %.2f", s, minSimilarity)
	}
}
