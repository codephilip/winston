package sanitize

import (
	"regexp"
	"strings"
)

// MaxInputLength is the maximum number of characters allowed in user input.
const MaxInputLength = 4000

// promptInjectionPatterns are regex patterns that match common prompt injection attempts.
var promptInjectionPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?previous\s+instructions`),
	regexp.MustCompile(`(?i)ignore\s+(all\s+)?prior\s+instructions`),
	regexp.MustCompile(`(?i)disregard\s+(all\s+)?previous\s+instructions`),
	regexp.MustCompile(`(?i)forget\s+(all\s+)?previous\s+instructions`),
	regexp.MustCompile(`(?i)^system\s*:`),
	regexp.MustCompile(`(?i)\bsystem\s+prompt\s*:`),
	regexp.MustCompile(`(?i)you\s+are\s+now\s+`),
	regexp.MustCompile(`(?i)new\s+instructions?\s*:`),
	regexp.MustCompile(`(?i)override\s+(all\s+)?instructions`),
	regexp.MustCompile(`(?i)act\s+as\s+if\s+you\s+have\s+no\s+restrictions`),
	regexp.MustCompile(`(?i)pretend\s+you\s+are\s+(?:a\s+)?(?:different|new)\s+`),
	regexp.MustCompile(`(?i)jailbreak`),
	regexp.MustCompile(`(?i)DAN\s+mode`),
}

// Input cleans user-provided text before it is passed to an agent.
// It truncates to MaxInputLength and strips known prompt injection patterns.
func Input(input string) string {
	// Trim whitespace first.
	input = strings.TrimSpace(input)

	// Enforce length limit.
	if len(input) > MaxInputLength {
		input = input[:MaxInputLength]
	}

	// Strip prompt injection patterns.
	for _, pat := range promptInjectionPatterns {
		input = pat.ReplaceAllString(input, "[redacted]")
	}

	return input
}
