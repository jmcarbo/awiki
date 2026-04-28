package synth

import "testing"

func TestFuzzySuggestionFindsClosestParagraph(t *testing.T) {
	source := `Knowledge work in 1945 hinged on a researcher's ability to traverse
their own associative trails through stored sources.

The memex would compress decades of literature into a desk-sized device.`
	quote := "knowledge work in 1945 hinged on a researchers ability"

	suggestion := FuzzySuggestion(source, quote)

	if suggestion == "" || !containsAny(suggestion, []string{"associative trails", "researcher"}) {
		t.Fatalf("FuzzySuggestion() = %q, want paragraph containing associative trails or researcher", suggestion)
	}
}

func TestFuzzySuggestionNoCloseMatch(t *testing.T) {
	source := `Knowledge work in 1945 hinged on a researcher's ability.`
	quote := "completely unrelated alien text xyzzy"

	if suggestion := FuzzySuggestion(source, quote); suggestion != "" {
		t.Fatalf("FuzzySuggestion() = %q, want empty", suggestion)
	}
}

func TestFuzzySuggestionTruncatesLongParagraph(t *testing.T) {
	source := "alpha beta gamma " + repeatWord("trail", 80)
	quote := "alpha beta gamma trail trail trail"

	suggestion := FuzzySuggestion(source, quote)

	if len(suggestion) > 203 || suggestion[len(suggestion)-3:] != "..." {
		t.Fatalf("FuzzySuggestion() = %q, want truncated suggestion ending in ...", suggestion)
	}
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		if stringsContains(text, needle) {
			return true
		}
	}
	return false
}

func stringsContains(text string, needle string) bool {
	return len(needle) == 0 || (len(text) >= len(needle) && indexString(text, needle) >= 0)
}

func indexString(text string, needle string) int {
	for i := 0; i+len(needle) <= len(text); i++ {
		if text[i:i+len(needle)] == needle {
			return i
		}
	}
	return -1
}

func repeatWord(word string, count int) string {
	out := ""
	for i := 0; i < count; i++ {
		if out != "" {
			out += " "
		}
		out += word
	}
	return out
}
