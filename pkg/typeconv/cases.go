package typeconv

import (
	"strings"
	"unicode"
)

// WordCase describes how a single word is capitalized while rendering a case style.
type WordCase uint8

const (
	// WordInherit is only meaningful as the first-word case and falls back to the word case.
	// As the word case itself it behaves like WordLower.
	WordInherit WordCase = iota

	// WordLower renders the word in lower case ("value").
	WordLower

	// WordUpper renders the word in upper case ("VALUE").
	WordUpper

	// WordTitle renders the word with an upper case first rune and a lower case rest ("Value").
	WordTitle
)

// Convert rewrites a value in the case style described by delimiter, wordCase and firstCase,
// regardless of the style the value was written in. It tokenizes the value and joins the words
// back together, so every named style is just a pairing of a delimiter and a word case.
//
//	delimiter │ WordLower      WordUpper       WordTitle
//	──────────┼──────────────────────────────────────────
//	"_"       │ snake_case     CONSTANT_CASE   Ada_Case
//	"-"       │ kebab-case     COBOL-CASE      Train-Case
//	""        │ flatcase       UPPERFLATCASE   PascalCase
//	" "       │ lower case     UPPER CASE      Title Case
//
// camelCase is the only style that needs firstCase: it is PascalCase with firstCase set to
// WordLower. Everything else passes WordInherit to apply wordCase to all words. Custom styles are
// just as valid, e.g. Convert(value, ".", WordUpper, WordInherit).
//
// The ToSnakeCase, ToCamelCase, … helpers wrap the styles listed above.
func Convert(value, delimiter string, wordCase, firstCase WordCase) string {
	return Join(Tokenize(value), delimiter, wordCase, firstCase)
}

// Join renders already tokenized words in the case style described by delimiter, wordCase and
// firstCase. Use it together with Tokenize when the words need to be filtered or amended in
// between, otherwise use Convert.
func Join(words []string, delimiter string, wordCase, firstCase WordCase) string {
	if len(words) == 0 {
		return ""
	}

	size := len(delimiter) * (len(words) - 1)
	for _, word := range words {
		size += len(word)
	}

	builder := strings.Builder{}
	builder.Grow(size)

	for i, word := range words {
		if i > 0 {
			builder.WriteString(delimiter)
		}

		if i == 0 && firstCase != WordInherit {
			writeWord(&builder, word, firstCase)

			continue
		}

		writeWord(&builder, word, wordCase)
	}

	return builder.String()
}

// Tokenize splits a value into its words, independent of the style it is written in.
//
// Any rune that is neither a letter nor a digit acts as a separator and is dropped, which covers
// "_", "-", " " and stray punctuation alike. Within a run of letters and digits a new word starts
// before an upper case rune that follows a lower case rune or a digit ("userID" → "user", "ID";
// "utf8Decode" → "utf8", "Decode") and before the last upper case rune of an acronym that is
// followed by a lower case rune ("HTTPServer" → "HTTP", "Server"). Digits stay attached to the word
// they follow, so "Int64Value" yields "Int64" and "Value".
//
// Words of scripts without case (e.g. CJK) contain no boundaries and therefore stay in one token.
func Tokenize(value string) []string {
	runes := []rune(value)
	words := make([]string, 0, len(runes)/4+1)
	start := -1

	for i, char := range runes {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) {
			if start >= 0 {
				words = append(words, string(runes[start:i]))
				start = -1
			}

			continue
		}

		switch {
		case start < 0:
			start = i
		case isWordBoundary(runes, i):
			words = append(words, string(runes[start:i]))
			start = i
		}
	}

	if start >= 0 {
		words = append(words, string(runes[start:]))
	}

	return words
}

// ToAdaCase converts a value to "Ada_Case".
func ToAdaCase(value string) string {
	return Convert(value, "_", WordTitle, WordInherit)
}

// ToCamelCase converts a value to "camelCase".
func ToCamelCase(value string) string {
	return Convert(value, "", WordTitle, WordLower)
}

// ToCobolCase converts a value to "COBOL-CASE".
func ToCobolCase(value string) string {
	return Convert(value, "-", WordUpper, WordInherit)
}

// ToConstantCase converts a value to "CONSTANT_CASE".
func ToConstantCase(value string) string {
	return Convert(value, "_", WordUpper, WordInherit)
}

// ToFlatCase converts a value to "flatcase".
func ToFlatCase(value string) string {
	return Convert(value, "", WordLower, WordInherit)
}

// ToKebabCase converts a value to "kebab-case".
func ToKebabCase(value string) string {
	return Convert(value, "-", WordLower, WordInherit)
}

// ToLowerCase converts a value to "lower case". Unlike strings.ToLower it also splits words, so
// "HTTPServer" becomes "http server".
func ToLowerCase(value string) string {
	return Convert(value, " ", WordLower, WordInherit)
}

// ToPascalCase converts a value to "PascalCase".
func ToPascalCase(value string) string {
	return Convert(value, "", WordTitle, WordInherit)
}

// ToSnakeCase converts a value to "snake_case".
func ToSnakeCase(value string) string {
	return Convert(value, "_", WordLower, WordInherit)
}

// ToTitleCase converts a value to "Title Case".
func ToTitleCase(value string) string {
	return Convert(value, " ", WordTitle, WordInherit)
}

// ToTrainCase converts a value to "Train-Case".
func ToTrainCase(value string) string {
	return Convert(value, "-", WordTitle, WordInherit)
}

// ToUpperCase converts a value to "UPPER CASE". Unlike strings.ToUpper it also splits words, so
// "HTTPServer" becomes "HTTP SERVER".
func ToUpperCase(value string) string {
	return Convert(value, " ", WordUpper, WordInherit)
}

// ToUpperFlatCase converts a value to "UPPERFLATCASE".
func ToUpperFlatCase(value string) string {
	return Convert(value, "", WordUpper, WordInherit)
}

// isWordBoundary reports whether a new word starts at runes[i], given that runes[i-1] belongs to
// the current word.
func isWordBoundary(runes []rune, i int) bool {
	// Only an upper case rune can open a word inside a run of letters and digits.
	if !unicode.IsUpper(runes[i]) {
		return false
	}

	// "userID", "utf8Decode": an upper case rune after anything not upper case.
	if !unicode.IsUpper(runes[i-1]) {
		return true
	}

	// "HTTPServer": the trailing upper case rune of an acronym belongs to the following word.
	return i+1 < len(runes) && unicode.IsLower(runes[i+1])
}

// writeWord appends a word to the builder using the given capitalization.
func writeWord(builder *strings.Builder, word string, wordCase WordCase) {
	switch wordCase {
	case WordUpper:
		builder.WriteString(strings.ToUpper(word))

	case WordTitle:
		for i, char := range word {
			if i == 0 {
				builder.WriteRune(unicode.ToTitle(char))
			} else {
				builder.WriteRune(unicode.ToLower(char))
			}
		}

	case WordLower, WordInherit:
		builder.WriteString(strings.ToLower(word))
	}
}
