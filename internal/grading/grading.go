package grading

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Question types
const (
	TypeMasked          = "masked"
	TypeErrorCorrection = "error_correction"
)

// CategoryPunctuation questions test capitals and punctuation, so those are
// compared strictly instead of being normalised away.
const CategoryPunctuation = "Punctuation Rules"

// minSimilarity is the word-level similarity to the expected answer below which
// an error-correction answer is rejected without consulting the judge.
const minSimilarity = 0.6

// Question is the information needed to grade an answer.
type Question struct {
	Type           string
	Category       string
	MaskedSentence string // contains "[MASK]"
	ErrorSentence  string
	CorrectAnswer  string
}

// Verdict is the outcome of grading one answer.
type Verdict struct {
	Correct  bool
	Feedback string
	// Method records how the verdict was reached: "exact", "rule" or "judge".
	Method string
}

// Judge decides whether a student's correction is an acceptable alternative to
// the expected answer. It is only consulted for close, non-identical answers.
type Judge interface {
	JudgeCorrection(original, expected, answer string) (acceptable bool, explanation string, err error)
}

// Grade scores answer against q. judge may be nil, in which case answers that
// would need judging are marked incorrect.
func Grade(q Question, answer string, judge Judge) Verdict {
	switch q.Type {
	case TypeMasked:
		return gradeMasked(q, answer)
	case TypeErrorCorrection:
		return gradeCorrection(q, answer, judge)
	default:
		return Verdict{Feedback: "This question could not be graded.", Method: "rule"}
	}
}

func gradeMasked(q Question, answer string) Verdict {
	got := Normalize(answer)
	want := Normalize(q.CorrectAnswer)
	// Accept the word on its own, or the whole sentence with the word filled in.
	filled := Normalize(strings.Replace(q.MaskedSentence, "[MASK]", q.CorrectAnswer, 1))
	if got == want || got == filled {
		return Verdict{Correct: true, Feedback: fmt.Sprintf("%q is the right word.", q.CorrectAnswer), Method: "exact"}
	}
	return Verdict{
		Feedback: fmt.Sprintf("The missing word is %q: %s", q.CorrectAnswer,
			strings.Replace(q.MaskedSentence, "[MASK]", q.CorrectAnswer, 1)),
		Method: "rule",
	}
}

func gradeCorrection(q Question, answer string, judge Judge) Verdict {
	norm := Normalize
	if q.Category == CategoryPunctuation {
		norm = normalizeStrict
	}
	got := norm(answer)
	want := norm(q.CorrectAnswer)
	wrongAnswer := Verdict{Feedback: fmt.Sprintf("A correct version is: %q", q.CorrectAnswer), Method: "rule"}

	switch {
	case got == "":
		return wrongAnswer
	case got == want:
		return Verdict{Correct: true, Feedback: "Well spotted - that's exactly right.", Method: "exact"}
	case got == norm(q.ErrorSentence):
		wrongAnswer.Feedback = fmt.Sprintf("That's the original sentence - something in it needs fixing. A correct version is: %q", q.CorrectAnswer)
		return wrongAnswer
	case Similarity(Normalize(answer), Normalize(q.CorrectAnswer)) < minSimilarity:
		return wrongAnswer
	case keepsMistake(Normalize(q.ErrorSentence), Normalize(q.CorrectAnswer), Normalize(answer)):
		wrongAnswer.Feedback = fmt.Sprintf("The mistake is still there. A correct version is: %q", q.CorrectAnswer)
		return wrongAnswer
	case judge == nil:
		return wrongAnswer
	}

	ok, explanation, err := judge.JudgeCorrection(q.ErrorSentence, q.CorrectAnswer, answer)
	if err != nil {
		// Be conservative: never award marks we couldn't verify.
		return wrongAnswer
	}
	if ok {
		feedback := "That's another good way to fix it."
		if explanation != "" {
			feedback += " " + explanation
		}
		return Verdict{Correct: true, Feedback: feedback, Method: "judge"}
	}
	feedback := wrongAnswer.Feedback
	if explanation != "" {
		feedback = explanation + " " + feedback
	}
	return Verdict{Feedback: feedback, Method: "judge"}
}

var (
	quoteReplacer = strings.NewReplacer("‘", "'", "’", "'", "“", "\"", "”", "\"", "`", "'")
	contractions  = []struct{ short, long string }{
		{"won't", "will not"}, {"can't", "cannot"}, {"shan't", "shall not"},
		{"n't", " not"}, {"'re", " are"}, {"'ve", " have"}, {"'ll", " will"}, {"'m", " am"},
	}
	spaceRun   = regexp.MustCompile(`\s+`)
	edgePunct  = regexp.MustCompile(`^[\s"'*]+|[\s"'.!?*]+$`)
	innerSpace = regexp.MustCompile(`\s+([,;:])`)
)

// Normalize makes two answers comparable: lower case, straight quotes,
// expanded contractions, single spaces, no surrounding quotes or end punctuation.
func Normalize(s string) string {
	s = strings.ToLower(quoteReplacer.Replace(s))
	for _, c := range contractions {
		s = strings.ReplaceAll(s, c.short, c.long)
	}
	s = spaceRun.ReplaceAllString(s, " ")
	s = innerSpace.ReplaceAllString(s, "$1")
	return edgePunct.ReplaceAllString(s, "")
}

// keepsMistake reports whether answer still contains every word the expected
// correction removed from the original, and none of the words it added
func keepsMistake(original, expected, answer string) bool {
	removed := wordDiff(words(original), words(expected))
	added := wordDiff(words(expected), words(original))
	if len(removed) == 0 {
		return false // punctuation-only fix; handled by the judge
	}
	got := words(answer)
	return len(wordDiff(removed, got)) == 0 && len(wordDiff(added, got)) == len(added)
}

// wordDiff returns the words of a not matched in b (multiset difference).
func wordDiff(a, b []string) []string {
	counts := map[string]int{}
	for _, w := range b {
		counts[w]++
	}
	var out []string
	for _, w := range a {
		if counts[w] > 0 {
			counts[w]--
		} else {
			out = append(out, w)
		}
	}
	return out
}

// normalizeStrict only unifies quote characters and whitespace, keeping case
// and punctuation significant.
func normalizeStrict(s string) string {
	return strings.TrimSpace(spaceRun.ReplaceAllString(quoteReplacer.Replace(s), " "))
}

// Similarity is the word-overlap F1 score of a and b, in [0, 1]. It ignores
// word order, so rephrasings ("I gave a gift to him") score high while
// unrelated or partial answers score low.
func Similarity(a, b string) float64 {
	wa, wb := words(a), words(b)
	if len(wa) == 0 && len(wb) == 0 {
		return 1
	}
	counts := map[string]int{}
	for _, w := range wb {
		counts[w]++
	}
	overlap := 0
	for _, w := range wa {
		if counts[w] > 0 {
			counts[w]--
			overlap++
		}
	}
	return 2 * float64(overlap) / float64(len(wa)+len(wb))
}

func words(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '\''
	})
}
