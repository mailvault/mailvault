package verification

import (
	"context"
	"net/mail"
	"regexp"
	"strings"
	"bytes"
	"unicode"
)

// ContentVerifier handles content-based spam detection
type ContentVerifier struct {
	spamPatterns     []spamPattern
	spamWords        []string
	suspiciousPhases []string
}

// spamPattern represents a spam detection pattern
type spamPattern struct {
	pattern     *regexp.Regexp
	description string
	weight      float64
}

// NewContentVerifier creates a new content verifier
func NewContentVerifier() *ContentVerifier {
	v := &ContentVerifier{
		spamWords: []string{
			"viagra", "cialis", "lottery", "winner", "congratulations",
			"urgent", "immediately", "act now", "limited time", "free money",
			"guaranteed", "no risk", "100% free", "cash bonus", "prize",
			"inheritance", "beneficiary", "nigerian prince", "wire transfer",
			"click here", "unsubscribe", "remove me", "opt out",
		},
		suspiciousPhases: []string{
			"dear sir/madam", "dear friend", "urgent business proposal",
			"confidential transaction", "foreign investment", "oil deal",
			"fund transfer", "deceased person", "next of kin", "beneficiary",
			"lawyer", "attorney", "law firm", "bank manager", "finance ministry",
		},
	}

	// Compile spam patterns
	patterns := []struct {
		regex       string
		description string
		weight      float64
	}{
		{`(?i)\b(viagra|cialis|levitra)\b`, "Pharmaceutical spam", 0.8},
		{`(?i)\b(lottery|winner|win|won)\b.*\$([\d,]+)`, "Lottery scam", 0.9},
		{`(?i)\b(urgent|immediately|act now|limited time)\b`, "Urgency tactics", 0.6},
		{`(?i)\b(free|100% free|no cost|no charge)\b`, "Free offers", 0.4},
		{`(?i)\b(guaranteed|no risk|risk free)\b`, "Guarantees", 0.5},
		{`(?i)\b(click here|click now|visit now)\b`, "Click baiting", 0.5},
		{`(?i)\b(unsubscribe|remove|opt.?out)\b`, "Fake unsubscribe", 0.3},
		{`\$[\d,]+\s*(million|billion|thousand)`, "Large money amounts", 0.7},
		{`(?i)\b(nigerian?|prince|princess|royal)\b`, "Nigerian scam", 0.95},
		{`(?i)\b(inheritance|beneficiary|next.?of.?kin)\b`, "Inheritance scam", 0.8},
		{`(?i)\b(wire.?transfer|western.?union|money.?gram)\b`, "Money transfer", 0.7},
		{`(?i)\b(attorney|lawyer|law.?firm|legal.?representative)\b`, "Legal impersonation", 0.6},
		{`(?i)\b(bank.?manager|finance.?minister|director)\b`, "Authority impersonation", 0.7},
		{`[A-Z]{10,}`, "Excessive capitals", 0.4},
		{`!{3,}`, "Excessive exclamation marks", 0.3},
		{`\?{3,}`, "Excessive question marks", 0.3},
		{`(?i)\b(enlarge|enhancement|enlargement)\b`, "Enhancement spam", 0.7},
		{`(?i)\b(lose.?weight|weight.?loss|diet.?pill)\b`, "Weight loss spam", 0.6},
		{`(?i)\b(make.?money|earn.?money|work.?from.?home)\b`, "Money making schemes", 0.6},
		{`(?i)\b(casino|poker|gambling|bet)\b`, "Gambling", 0.5},
		{`\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b`, "Email in content", 0.2},
		{`https?://[^\s]+`, "URLs in content", 0.2},
		{`(?i)\b(sex|adult|xxx|porn)\b`, "Adult content", 0.8},
	}

	for _, p := range patterns {
		compiled, err := regexp.Compile(p.regex)
		if err == nil {
			v.spamPatterns = append(v.spamPatterns, spamPattern{
				pattern:     compiled,
				description: p.description,
				weight:      p.weight,
			})
		}
	}

	return v
}

// Verify performs content analysis on the email
func (v *ContentVerifier) Verify(ctx context.Context, emailCtx EmailContext) ContentResult {
	// Parse email
	msg, err := mail.ReadMessage(bytes.NewReader(emailCtx.Body))
	if err != nil {
		return ContentResult{
			SpamScore:      0.5,
			Classification: "unknown",
			Error:          "failed to parse email",
		}
	}

	// Extract text content
	bodyText := v.extractTextContent(emailCtx.Body)
	subject := emailCtx.Subject

	// Analyze content
	indicators := []string{}
	totalScore := 0.0

	// Check subject line
	subjectScore, subjectIndicators := v.analyzeText(subject)
	totalScore += subjectScore * 1.5 // Weight subject more heavily
	indicators = append(indicators, subjectIndicators...)

	// Check body content
	bodyScore, bodyIndicators := v.analyzeText(bodyText)
	totalScore += bodyScore
	indicators = append(indicators, bodyIndicators...)

	// Additional heuristics
	heuristicScore, heuristicIndicators := v.applyHeuristics(subject, bodyText, msg)
	totalScore += heuristicScore
	indicators = append(indicators, heuristicIndicators...)

	// Normalize score
	finalScore := totalScore
	if finalScore > 1.0 {
		finalScore = 1.0
	} else if finalScore < 0.0 {
		finalScore = 0.0
	}

	// Classify
	classification := v.classifyContent(finalScore)

	return ContentResult{
		SpamScore:      finalScore,
		SpamIndicators: indicators,
		Classification: classification,
	}
}

// analyzeText analyzes text content for spam indicators
func (v *ContentVerifier) analyzeText(text string) (float64, []string) {
	if text == "" {
		return 0.0, nil
	}

	text = strings.ToLower(text)
	score := 0.0
	indicators := []string{}

	// Check spam patterns
	for _, pattern := range v.spamPatterns {
		if pattern.pattern.MatchString(text) {
			score += pattern.weight
			indicators = append(indicators, pattern.description)
		}
	}

	// Check spam words
	wordScore := v.checkSpamWords(text)
	score += wordScore
	if wordScore > 0 {
		indicators = append(indicators, "Contains spam words")
	}

	// Check suspicious phrases
	phraseScore := v.checkSuspiciousPhrases(text)
	score += phraseScore
	if phraseScore > 0 {
		indicators = append(indicators, "Contains suspicious phrases")
	}

	return score, indicators
}

// checkSpamWords checks for known spam words
func (v *ContentVerifier) checkSpamWords(text string) float64 {
	words := strings.Fields(text)
	spamWordCount := 0
	totalWords := len(words)

	if totalWords == 0 {
		return 0.0
	}

	for _, word := range words {
		word = strings.ToLower(strings.Trim(word, ".,!?;:()[]{}\"'"))
		for _, spamWord := range v.spamWords {
			if strings.Contains(word, spamWord) {
				spamWordCount++
				break
			}
		}
	}

	// Calculate spam word ratio
	ratio := float64(spamWordCount) / float64(totalWords)
	
	// Convert ratio to score (0-1)
	if ratio >= 0.1 {
		return 0.8
	} else if ratio >= 0.05 {
		return 0.5
	} else if ratio > 0 {
		return 0.2
	}
	
	return 0.0
}

// checkSuspiciousPhrases checks for suspicious phrases
func (v *ContentVerifier) checkSuspiciousPhrases(text string) float64 {
	score := 0.0
	
	for _, phrase := range v.suspiciousPhases {
		if strings.Contains(text, phrase) {
			score += 0.3
		}
	}
	
	return score
}

// applyHeuristics applies additional spam detection heuristics
func (v *ContentVerifier) applyHeuristics(subject, body string, msg *mail.Message) (float64, []string) {
	score := 0.0
	indicators := []string{}

	// Check for excessive capitalization
	if v.hasExcessiveCapitals(subject + " " + body) {
		score += 0.3
		indicators = append(indicators, "Excessive capitalization")
	}

	// Check character encoding issues
	if v.hasSuspiciousEncoding(body) {
		score += 0.2
		indicators = append(indicators, "Suspicious character encoding")
	}

	// Check HTML to text ratio
	htmlRatio := v.calculateHTMLRatio(body)
	if htmlRatio > 0.8 {
		score += 0.3
		indicators = append(indicators, "High HTML to text ratio")
	}

	// Check for hidden text
	if v.hasHiddenText(body) {
		score += 0.5
		indicators = append(indicators, "Hidden text detected")
	}

	// Check for suspicious headers
	headerScore, headerIndicators := v.analyzeHeaders(msg)
	score += headerScore
	indicators = append(indicators, headerIndicators...)

	// Check message length
	if len(body) < 50 {
		score += 0.2
		indicators = append(indicators, "Very short message")
	}

	return score, indicators
}

// hasExcessiveCapitals checks for excessive use of capital letters
func (v *ContentVerifier) hasExcessiveCapitals(text string) bool {
	if len(text) < 10 {
		return false
	}

	capitals := 0
	letters := 0

	for _, r := range text {
		if unicode.IsLetter(r) {
			letters++
			if unicode.IsUpper(r) {
				capitals++
			}
		}
	}

	if letters == 0 {
		return false
	}

	ratio := float64(capitals) / float64(letters)
	return ratio > 0.5 // More than 50% capitals
}

// hasSuspiciousEncoding checks for suspicious character encoding
func (v *ContentVerifier) hasSuspiciousEncoding(text string) bool {
	// Check for excessive non-ASCII characters
	nonASCII := 0
	total := len(text)

	if total == 0 {
		return false
	}

	for _, r := range text {
		if r > 127 {
			nonASCII++
		}
	}

	ratio := float64(nonASCII) / float64(total)
	return ratio > 0.3 // More than 30% non-ASCII
}

// calculateHTMLRatio calculates the ratio of HTML tags to text content
func (v *ContentVerifier) calculateHTMLRatio(content string) float64 {
	// Simple HTML tag detection
	htmlPattern := regexp.MustCompile(`<[^>]+>`)
	htmlTags := htmlPattern.FindAllString(content, -1)
	
	htmlLength := 0
	for _, tag := range htmlTags {
		htmlLength += len(tag)
	}

	if len(content) == 0 {
		return 0.0
	}

	return float64(htmlLength) / float64(len(content))
}

// hasHiddenText checks for hidden text techniques
func (v *ContentVerifier) hasHiddenText(content string) bool {
	// Check for HTML with hidden text (simplified)
	hiddenPatterns := []string{
		`color:\s*#?ffffff`,
		`color:\s*white`,
		`display:\s*none`,
		`visibility:\s*hidden`,
		`font-size:\s*0`,
	}

	content = strings.ToLower(content)
	for _, pattern := range hiddenPatterns {
		if matched, _ := regexp.MatchString(pattern, content); matched {
			return true
		}
	}

	return false
}

// analyzeHeaders analyzes email headers for suspicious patterns
func (v *ContentVerifier) analyzeHeaders(msg *mail.Message) (float64, []string) {
	score := 0.0
	indicators := []string{}

	// Check for forged headers
	if v.hasForgedHeaders(msg) {
		score += 0.4
		indicators = append(indicators, "Forged headers detected")
	}

	// Check for suspicious routing
	if v.hasSuspiciousRouting(msg) {
		score += 0.3
		indicators = append(indicators, "Suspicious routing")
	}

	return score, indicators
}

// hasForgedHeaders checks for common header forgery patterns
func (v *ContentVerifier) hasForgedHeaders(msg *mail.Message) bool {
	// Check for missing or suspicious Message-ID
	messageID := msg.Header.Get("Message-ID")
	if messageID == "" {
		return true
	}

	// Check for suspicious Date header
	date := msg.Header.Get("Date")
	if date == "" {
		return true
	}

	return false
}

// hasSuspiciousRouting checks for suspicious email routing
func (v *ContentVerifier) hasSuspiciousRouting(msg *mail.Message) bool {
	// Check Received headers for suspicious patterns
	received := msg.Header["Received"]
	if len(received) == 0 {
		return true
	}

	// Simple check - in production, this would be more sophisticated
	for _, r := range received {
		if strings.Contains(strings.ToLower(r), "unknown") ||
		   strings.Contains(strings.ToLower(r), "localhost") {
			return true
		}
	}

	return false
}

// extractTextContent extracts plain text from email body
func (v *ContentVerifier) extractTextContent(body []byte) string {
	// Simple text extraction - in production, use a proper email parser
	content := string(body)
	
	// Find body start
	bodyStart := strings.Index(content, "\r\n\r\n")
	if bodyStart == -1 {
		bodyStart = strings.Index(content, "\n\n")
		if bodyStart == -1 {
			return content
		}
		bodyStart += 2
	} else {
		bodyStart += 4
	}

	emailBody := content[bodyStart:]
	
	// Remove HTML tags (simplified)
	htmlPattern := regexp.MustCompile(`<[^>]*>`)
	emailBody = htmlPattern.ReplaceAllString(emailBody, " ")
	
	// Clean up whitespace
	emailBody = regexp.MustCompile(`\s+`).ReplaceAllString(emailBody, " ")
	emailBody = strings.TrimSpace(emailBody)
	
	return emailBody
}

// classifyContent classifies content based on spam score
func (v *ContentVerifier) classifyContent(score float64) string {
	if score >= 0.8 {
		return "spam"
	} else if score >= 0.5 {
		return "suspicious"
	} else if score >= 0.2 {
		return "questionable"
	} else {
		return "legitimate"
	}
}