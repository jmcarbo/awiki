package synth

import "strings"

func FuzzySuggestion(source string, quote string) string {
	paragraphs := splitParagraphs(source)
	if len(paragraphs) == 0 {
		return ""
	}
	normalizedQuote := NormalizeEvidenceText(quote)
	bestIndex := -1
	bestScore := 0.0
	for i, paragraph := range paragraphs {
		score := similarity(normalizedQuote, NormalizeEvidenceText(paragraph))
		if score > bestScore {
			bestScore = score
			bestIndex = i
		}
	}
	if bestIndex < 0 || bestScore < 0.55 {
		return ""
	}
	return truncateSuggestion(paragraphs[bestIndex])
}

func splitParagraphs(text string) []string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	var paragraphs []string
	for _, paragraph := range strings.Split(text, "\n\n") {
		paragraph = strings.TrimSpace(paragraph)
		if paragraph != "" {
			paragraphs = append(paragraphs, paragraph)
		}
	}
	return paragraphs
}

func similarity(a string, b string) float64 {
	if a == "" || b == "" {
		return 0
	}
	aTokens := strings.Fields(a)
	if len(aTokens) > 0 {
		matches := 0
		for _, token := range aTokens {
			if strings.Contains(b, token) {
				matches++
			}
		}
		tokenScore := float64(matches) / float64(len(aTokens))
		if tokenScore >= 0.55 {
			return tokenScore
		}
	}
	distance := levenshtein(a, b)
	maxLen := len([]rune(a))
	if other := len([]rune(b)); other > maxLen {
		maxLen = other
	}
	if maxLen == 0 {
		return 1
	}
	return 1 - float64(distance)/float64(maxLen)
}

func levenshtein(a string, b string) int {
	ar := []rune(a)
	br := []rune(b)
	previous := make([]int, len(br)+1)
	current := make([]int, len(br)+1)
	for j := range previous {
		previous[j] = j
	}
	for i, ra := range ar {
		current[0] = i + 1
		for j, rb := range br {
			cost := 0
			if ra != rb {
				cost = 1
			}
			current[j+1] = minInt(
				current[j]+1,
				previous[j+1]+1,
				previous[j]+cost,
			)
		}
		previous, current = current, previous
	}
	return previous[len(br)]
}

func minInt(values ...int) int {
	min := values[0]
	for _, value := range values[1:] {
		if value < min {
			min = value
		}
	}
	return min
}

func truncateSuggestion(suggestion string) string {
	if len(suggestion) <= 200 {
		return suggestion
	}
	cut := suggestion[:200]
	if lastSpace := strings.LastIndex(cut, " "); lastSpace > 0 {
		cut = cut[:lastSpace]
	}
	return cut + "..."
}
