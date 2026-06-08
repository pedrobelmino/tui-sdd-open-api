package workflow

import (
	"fmt"
	"strings"
)

// ExtractQuestions extracts numbered questions from the "## Clarifying Questions for User" section.
// Returns each question as a string, without the leading "1. " prefix.
func ExtractQuestions(markdown string) []string {
	body := extractClarifyingBody(markdown)
	if body == "" {
		return nil
	}
	var questions []string
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Match "1. question text" or "1. **question text**"
		trimmed := line
		for strings.HasPrefix(trimmed, "**") {
			trimmed = strings.TrimPrefix(trimmed, "**")
		}
		// Check for numbered format: digit(s). text
		after, ok := strings.CutPrefix(trimmed, "1. ")
		if !ok {
			after, ok = strings.CutPrefix(trimmed, "2. ")
		}
		if !ok {
			after, ok = strings.CutPrefix(trimmed, "3. ")
		}
		if !ok {
			after, ok = strings.CutPrefix(trimmed, "4. ")
		}
		if !ok {
			after, ok = strings.CutPrefix(trimmed, "5. ")
		}
		if !ok {
			after, ok = strings.CutPrefix(trimmed, "6. ")
		}
		if !ok {
			after, ok = strings.CutPrefix(trimmed, "7. ")
		}
		if !ok {
			after, ok = strings.CutPrefix(trimmed, "8. ")
		}
		if !ok {
			after, ok = strings.CutPrefix(trimmed, "9. ")
		}
		if !ok {
			after, ok = strings.CutPrefix(trimmed, "10. ")
		}
		if ok {
			questions = append(questions, strings.TrimSpace(after))
		}
	}
	return questions
}

// extractClarifyingBody finds the "## Clarifying Questions for User" section
// and returns its content (text between the heading and the next heading or end).
func extractClarifyingBody(markdown string) string {
	idx := strings.Index(markdown, "## Clarifying Questions for User")
	if idx < 0 {
		return ""
	}
	// Skip past the heading line
	rest := markdown[idx+len("## Clarifying Questions for User"):]
	// Find the end of this heading's line
	if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
		rest = rest[nl+1:]
	}
	// Collect lines until we hit the next ## heading or end
	var body strings.Builder
	for _, line := range strings.Split(rest, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "## ") {
			break
		}
		if body.Len() > 0 {
			body.WriteByte('\n')
		}
		body.WriteString(line)
	}
	return strings.TrimSpace(body.String())
}

// HasClarifyingQuestions returns true if the markdown contains a
// "Clarifying Questions for User" or "Decision Checkpoints" section with content.
func HasClarifyingQuestions(markdown string) bool {
	return len(ExtractQuestions(markdown)) > 0 || len(ExtractCheckpointQuestions(markdown)) > 0
}

// FormatQuestionsForDisplay formats a list of questions for the user prompt.
func FormatQuestionsForDisplay(questions []string) string {
	if len(questions) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("The model has the following clarifying questions:\n\n")
	for i, q := range questions {
		b.WriteString(fmt.Sprintf("%d. %s\n", i+1, q))
	}
	b.WriteString("\nPlease answer each question (or press enter to skip individual questions).")
	return b.String()
}

// CheckpointQuestion represents a single decision checkpoint extracted from the LLM draft.
type CheckpointQuestion struct {
	Question string
	Options  []string // parsed from the question text (e.g., "SPA vs SSG vs SSR" → ["SPA", "SSG", "SSR"])
}

// ExtractCheckpointQuestions extracts decision checkpoints from the
// "## Decision Checkpoints for User Approval" or "## Clarifying Questions for User" section.
// Returns individual questions with any detected multiple-choice options.
func ExtractCheckpointQuestions(markdown string) []CheckpointQuestion {
	body := extractCheckpointBody(markdown)
	if body == "" {
		body = extractClarifyingBody(markdown)
	}
	if body == "" {
		return nil
	}
	var questions []CheckpointQuestion
	// Split on numbered items: "1. **Title** – ..." or "1. Title — ..."
	lines := strings.Split(body, "\n")
	var current CheckpointQuestion
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			if current.Question != "" {
				questions = append(questions, current)
				current = CheckpointQuestion{}
			}
			continue
		}
		// Check if this line starts a new numbered question
		if isNumberedLine(line) {
			if current.Question != "" {
				questions = append(questions, current)
			}
			current = parseCheckpointLine(line)
		} else {
			if current.Question != "" {
				current.Question += " " + line
				current.Options = append(current.Options, parseOptionsFromLine(line)...)
			}
		}
	}
	if current.Question != "" {
		questions = append(questions, current)
	}
	return questions
}

func extractCheckpointBody(markdown string) string {
	for _, heading := range []string{
		"Decision Checkpoints for User Approval",
		"Decision Checkpoints",
		"Follow-up Interview Script",
		"Follow-up Interview",
		"Interview Script",
	} {
		for _, line := range strings.Split(markdown, "\n") {
			trimmed := strings.TrimSpace(line)
			if !strings.HasPrefix(trimmed, "## ") {
				continue
			}
			if !strings.Contains(strings.ToLower(trimmed), strings.ToLower(heading)) {
				continue
			}
			// Found the heading line — extract body from after it
			rest := markdown[strings.Index(markdown, line)+len(line):]
			if nl := strings.IndexByte(rest, '\n'); nl >= 0 {
				rest = rest[nl+1:]
			}
			var body strings.Builder
			for _, bline := range strings.Split(rest, "\n") {
				bt := strings.TrimSpace(bline)
				if strings.HasPrefix(bt, "## ") || strings.HasPrefix(bt, "# ") {
					break
				}
				if body.Len() > 0 {
					body.WriteByte('\n')
				}
				body.WriteString(bline)
			}
			return strings.TrimSpace(body.String())
		}
	}
	return ""
}

func isNumberedLine(line string) bool {
	for i, ch := range line {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '.' && i > 0 {
			return true
		}
		return false
	}
	return false
}

func parseCheckpointLine(line string) CheckpointQuestion {
	// Remove the number prefix: "1. **Title** – rest" or "1. Title: rest", "1. **Q-01:** …"
	dotIdx := strings.IndexByte(line, '.')
	if dotIdx < 0 {
		return CheckpointQuestion{Question: line}
	}
	text := strings.TrimSpace(line[dotIdx+1:])
	// Strip bold and italic markers
	text = strings.ReplaceAll(text, "**", "")
	text = strings.ReplaceAll(text, "*", "")

	// Strip "Q-01:" / "Q-1:" prefix (spec interview questions)
	text = stripQPrefix(text)

	// Split on " – " (en-dash), " - " (regular hyphen), "–" or ":" to separate title from option text
	var optionPart string
	if idx := strings.Index(text, " – "); idx >= 0 {
		optionPart = strings.TrimSpace(text[idx+len(" – "):])
	} else if idx := strings.Index(text, " - "); idx >= 0 {
		optionPart = strings.TrimSpace(text[idx+len(" - "):])
	} else if idx := strings.Index(text, "–"); idx >= 0 {
		optionPart = strings.TrimSpace(text[idx+len("–"):])
	} else if idx := strings.Index(text, ":"); idx >= 0 {
		optionPart = strings.TrimSpace(text[idx+1:])
	} else {
		optionPart = text
	}

	// Strip surrounding quotes from the option part (spec interview questions)
	optionPart = strings.Trim(optionPart, `"'`)

	options := parseOptionsFromLine(optionPart)
	return CheckpointQuestion{Question: text, Options: options}
}

// stripQPrefix removes a leading "Q-01:" / "Q-1:" prefix from text.
func stripQPrefix(text string) string {
	lower := strings.ToLower(text)
	for i := 0; i < len(lower); i++ {
		if lower[i] == 'q' && i+1 < len(lower) && lower[i+1] == '-' {
			j := i + 2
			for j < len(lower) && ((lower[j] >= '0' && lower[j] <= '9') || lower[j] == ' ') {
				j++
			}
			if j < len(lower) && lower[j] == ':' {
				text = strings.TrimSpace(text[:i] + text[j+1:])
				text = strings.TrimLeft(text, "- ")
				return text
			}
		}
	}
	return text
}

func parseOptionsFromLine(line string) []string {
	// Try "A) X, B) Y, C) Z" pattern first
	var opts []string
	letters := []string{"A)", "B)", "C)", "D)", "E)", "F)"}
	for _, letter := range letters {
		idx := strings.Index(line, letter)
		if idx < 0 {
			continue
		}
		rest := strings.TrimSpace(line[idx+len(letter):])
		end := len(rest)
		for _, l2 := range letters {
			if i := strings.Index(rest, " "+l2); i >= 0 && i < end {
				end = i
			}
			if i := strings.Index(rest, ", "+l2); i >= 0 && i < end {
				end = i
			}
		}
		if comma := strings.IndexByte(rest, ','); comma >= 0 && comma < end {
			end = comma
		}
		opts = append(opts, strings.TrimSpace(rest[:end]))
	}
	if len(opts) > 0 {
		return opts
	}

	cleanOpt := func(p string) string {
		p = strings.TrimSpace(p)
		if paren := strings.LastIndex(p, "("); paren >= 0 {
			if trimmed := strings.TrimSpace(p[:paren]); trimmed != "" {
				p = trimmed
			}
		}
		return strings.TrimRight(p, ". ")
	}

	// Try "Option1 vs Option2 vs Option3"
	if strings.Contains(line, " vs ") {
		parts := strings.Split(line, " vs ")
		var out []string
		for _, p := range parts {
			if c := cleanOpt(p); c != "" && len(c) < 100 {
				out = append(out, c)
			}
		}
		if len(out) >= 2 {
			return out
		}
	}

	// Try "Option1 or Option2 or Option3" (common in spec interview questions)
	if strings.Contains(line, " or ") {
		parts := strings.Split(line, " or ")
		var out []string
		for _, p := range parts {
			if c := cleanOpt(p); c != "" && len(c) < 100 {
				out = append(out, c)
			}
		}
		// Require at least 2 options and at most 5 (avoid splitting on casual "or")
		if len(out) >= 2 && len(out) <= 5 {
			return out
		}
	}

	// Try "Option1 / Option2" or "Option1/Option2"
	sep := " / "
	if !strings.Contains(line, " / ") {
		sep = "/"
	}
	if strings.Contains(line, sep) {
		parts := strings.Split(line, sep)
		var out []string
		for _, p := range parts {
			if c := cleanOpt(p); c != "" && len(c) < 50 {
				out = append(out, c)
			}
		}
		if len(out) >= 2 {
			return out
		}
	}

	return nil
}

// FormatCheckpointForDisplay formats a single checkpoint question for the form UI.
func FormatCheckpointForDisplay(q CheckpointQuestion, idx, total int) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Question %d of %d\n\n", idx+1, total))
	b.WriteString(q.Question)
	if len(q.Options) > 0 {
		b.WriteString("\n\nOptions:\n")
		for i, opt := range q.Options {
			b.WriteString(fmt.Sprintf("  %d. %s\n", i+1, opt))
		}
		b.WriteString("\nType the option number or your answer.")
	}
	return b.String()
}
// incorporating the draft and the user's answers.
func BuildFinalizeUserMsg(feature, draft, answers string, extraContext ...string) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Feature: %s\n\n", feature))
	b.WriteString("== Previous Draft ==\n")
	b.WriteString(draft)
	b.WriteString("\n\n")
	for _, ctx := range extraContext {
		if strings.TrimSpace(ctx) != "" {
			b.WriteString(ctx)
			b.WriteString("\n\n")
		}
	}
	b.WriteString("== User Answers to Clarifying Questions ==\n")
	b.WriteString(answers)
	b.WriteString("\n\n")
	b.WriteString("Generate the final document now. ")
	b.WriteString("Incorporate the user answers into the document. ")
	b.WriteString("Remove the sections '## Clarifying Questions for User' and ")
	b.WriteString("'## Assumptions Pending User Confirmation' from the final output. ")
	b.WriteString("Do not include '## Questions Blocking Execution' if the user has answered the blocking questions.")
	return b.String()
}
