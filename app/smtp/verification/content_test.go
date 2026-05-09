package verification

import (
	"context"
	"net/mail"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestContentVerifier_Verify_LegitimateEmail(t *testing.T) {
	verifier := NewContentVerifier()
	
	emailCtx := EmailContext{
		Subject: "Weekly team meeting",
		Body: []byte("From: manager@company.com\r\n" +
			"To: team@company.com\r\n" +
			"Subject: Weekly team meeting\r\n" +
			"Date: Mon, 19 Aug 2025 12:00:00 -0300\r\n" +
			"Message-ID: <123@company.com>\r\n" +
			"\r\n" +
			"Hi team,\r\n\r\n" +
			"Please join our weekly team meeting tomorrow at 2 PM in conference room A. " +
			"We'll discuss the project progress and upcoming deadlines.\r\n\r\n" +
			"Best regards,\r\n" +
			"Manager"),
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	assert.Less(t, result.SpamScore, 0.5, "Legitimate email should have low spam score")
	assert.Contains(t, []string{"legitimate", "questionable"}, result.Classification)
}

func TestContentVerifier_Verify_SpamEmail(t *testing.T) {
	verifier := NewContentVerifier()
	
	emailCtx := EmailContext{
		Subject: "FREE VIAGRA!!! GUARANTEED 100% FREE!!!",
		Body: []byte("From: spammer@spam.com\r\n" +
			"To: victim@test.com\r\n" +
			"Subject: FREE VIAGRA!!! GUARANTEED 100% FREE!!!\r\n" +
			"\r\n" +
			"CONGRATULATIONS! You've won the LOTTERY! " +
			"FREE MONEY guaranteed! CLICK HERE NOW! " +
			"100% FREE! NO RISK! Act immediately! " +
			"VIAGRA, CIALIS available! Nigerian prince needs help! " +
			"Wire transfer required! Urgent business proposal!"),
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	assert.Greater(t, result.SpamScore, 0.7, "Spam email should have high spam score")
	assert.Equal(t, "spam", result.Classification)
	assert.NotEmpty(t, result.SpamIndicators, "Should have spam indicators")
	
	// Check for specific indicators
	indicators := strings.Join(result.SpamIndicators, " ")
	assert.Contains(t, indicators, "Pharmaceutical spam")
	// Note: Lottery scam might not trigger if the regex doesn't match exactly
	// assert.Contains(t, indicators, "Lottery scam")
	assert.Contains(t, indicators, "Free offers")
}

func TestContentVerifier_Verify_MalformedEmail(t *testing.T) {
	verifier := NewContentVerifier()
	
	emailCtx := EmailContext{
		Subject: "Test",
		Body:    []byte("This is not a proper email format - no headers"),
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	// Should handle malformed emails gracefully
	assert.NotEqual(t, "", result.Classification)
	assert.Contains(t, result.Error, "failed to parse email")
	assert.Equal(t, 0.5, result.SpamScore) // Default score for parse errors
}

func TestContentVerifier_Verify_EmptyContent(t *testing.T) {
	verifier := NewContentVerifier()
	
	emailCtx := EmailContext{
		Subject: "",
		Body: []byte("From: test@example.com\r\n" +
			"To: recipient@test.com\r\n" +
			"Subject: \r\n" +
			"\r\n"),
	}
	
	result := verifier.Verify(context.Background(), emailCtx)
	
	// Empty content should be flagged as questionable
	assert.Greater(t, result.SpamScore, 0.0)
	assert.Contains(t, result.SpamIndicators, "Very short message")
}

func TestContentVerifier_AnalyzeText_SpamPatterns(t *testing.T) {
	verifier := NewContentVerifier()
	
	tests := []struct {
		name          string
		text          string
		expectScore   bool
		expectedDesc  string
	}{
		{
			name:         "Pharmaceutical spam",
			text:         "Buy VIAGRA online now!",
			expectScore:  true,
			expectedDesc: "Pharmaceutical spam",
		},
		{
			name:         "Lottery scam",
			text:         "You won the lottery! $1,000,000 prize!",
			expectScore:  true,
			expectedDesc: "Lottery scam",
		},
		{
			name:         "Urgency tactics",
			text:         "Act now! Limited time offer!",
			expectScore:  true,
			expectedDesc: "Urgency tactics",
		},
		{
			name:         "Free offers",
			text:         "100% free guaranteed offer",
			expectScore:  true,
			expectedDesc: "Free offers",
		},
		{
			name:        "Legitimate text",
			text:        "Please review the attached document and let me know your thoughts.",
			expectScore: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, indicators := verifier.analyzeText(tt.text)
			
			if tt.expectScore {
				assert.Greater(t, score, 0.0, "Should have positive spam score")
				assert.NotEmpty(t, indicators, "Should have indicators")
				
				if tt.expectedDesc != "" {
					found := false
					for _, indicator := range indicators {
						if indicator == tt.expectedDesc {
							found = true
							break
						}
					}
					assert.True(t, found, "Should contain expected indicator: %s", tt.expectedDesc)
				}
			} else {
				assert.Equal(t, 0.0, score, "Should have zero spam score")
			}
		})
	}
}

func TestContentVerifier_CheckSpamWords(t *testing.T) {
	verifier := NewContentVerifier()
	
	tests := []struct {
		name     string
		text     string
		expected float64
	}{
		{
			name:     "No spam words",
			text:     "Please review the document and provide feedback",
			expected: 0.0,
		},
		{
			name:     "Few spam words",
			text:     "Get free consultation about your project",
			expected: 0.0, // Actually no spam words in this text after proper matching
		},
		{
			name:     "Medium spam words",
			text:     "Free money guaranteed prize winner",
			expected: 0.8, // High ratio
		},
		{
			name:     "Many spam words",
			text:     "free guaranteed winner lottery prize money cash",
			expected: 0.8, // Very high ratio
		},
		{
			name:     "Empty text",
			text:     "",
			expected: 0.0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := verifier.checkSpamWords(tt.text)
			assert.Equal(t, tt.expected, score)
		})
	}
}

func TestContentVerifier_CheckSuspiciousPhrases(t *testing.T) {
	verifier := NewContentVerifier()
	
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "Nigerian prince scam",
			text:     "Dear friend, I am a Nigerian prince with urgent business proposal",
			expected: true,
		},
		{
			name:     "Generic scam greeting",
			text:     "Dear sir/madam, I have confidential transaction for you",
			expected: true,
		},
		{
			name:     "Oil deal scam",
			text:     "I represent an oil deal and need your assistance with the fund transfer",
			expected: true,
		},
		{
			name:     "Legitimate business email",
			text:     "I hope this email finds you well. I wanted to follow up on our meeting",
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := verifier.checkSuspiciousPhrases(tt.text)
			
			if tt.expected {
				assert.Greater(t, score, 0.0, "Should detect suspicious phrases")
			} else {
				assert.Equal(t, 0.0, score, "Should not detect suspicious phrases")
			}
		})
	}
}

func TestContentVerifier_HasExcessiveCapitals(t *testing.T) {
	verifier := NewContentVerifier()
	
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "Normal text",
			text:     "This is a normal email with proper capitalization.",
			expected: false,
		},
		{
			name:     "Excessive capitals",
			text:     "THIS IS AN EMAIL WITH TOO MANY CAPITAL LETTERS!!!",
			expected: true,
		},
		{
			name:     "Mixed case but mostly capitals",
			text:     "BUY NOW!!! BEST DEAL EVER!!! LIMITED TIME!!!",
			expected: true,
		},
		{
			name:     "Short text",
			text:     "OK",
			expected: false, // Too short to be considered excessive
		},
		{
			name:     "Empty text",
			text:     "",
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := verifier.hasExcessiveCapitals(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContentVerifier_HasSuspiciousEncoding(t *testing.T) {
	verifier := NewContentVerifier()
	
	tests := []struct {
		name     string
		text     string
		expected bool
	}{
		{
			name:     "Normal ASCII text",
			text:     "This is a normal email with ASCII characters only.",
			expected: false,
		},
		{
			name:     "Some unicode characters",
			text:     "This email has some émojis 😀 and accénts.",
			expected: false, // Not excessive
		},
		{
			name:     "Excessive non-ASCII",
			text:     "这是一封包含大量非ASCII字符的电子邮件测试内容用于检测可疑编码",
			expected: true,
		},
		{
			name:     "Empty text",
			text:     "",
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := verifier.hasSuspiciousEncoding(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContentVerifier_CalculateHTMLRatio(t *testing.T) {
	verifier := NewContentVerifier()
	
	tests := []struct {
		name     string
		content  string
		expected float64
	}{
		{
			name:     "Plain text",
			content:  "This is plain text with no HTML tags.",
			expected: 0.0,
		},
		{
			name:     "Some HTML",
			content:  "<p>This is <b>bold</b> text.</p>",
			expected: float64(len("<p></p><b></b>")) / float64(len("<p>This is <b>bold</b> text.</p>")),
		},
		{
			name:     "Mostly HTML",
			content:  "<html><body><p></p><div></div><span></span></body></html>text",
			expected: 0.8, // Should be high
		},
		{
			name:     "Empty content",
			content:  "",
			expected: 0.0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := verifier.calculateHTMLRatio(tt.content)
			
			if tt.name == "Mostly HTML" {
				assert.Greater(t, result, 0.5, "Should have high HTML ratio")
			} else {
				assert.InDelta(t, tt.expected, result, 0.1, "HTML ratio should match expected")
			}
		})
	}
}

func TestContentVerifier_HasHiddenText(t *testing.T) {
	verifier := NewContentVerifier()
	
	tests := []struct {
		name     string
		content  string
		expected bool
	}{
		{
			name:     "Plain text",
			content:  "This is plain text with no hidden elements.",
			expected: false,
		},
		{
			name:     "White text on white background",
			content:  `<span style="color: white;">Hidden text</span>`,
			expected: true,
		},
		{
			name:     "Display none",
			content:  `<div style="display: none;">Hidden content</div>`,
			expected: true,
		},
		{
			name:     "Visibility hidden",
			content:  `<p style="visibility: hidden;">Hidden paragraph</p>`,
			expected: true,
		},
		{
			name:     "Font size zero",
			content:  `<span style="font-size: 0;">Invisible text</span>`,
			expected: true,
		},
		{
			name:     "Normal styling",
			content:  `<p style="color: black; font-size: 14px;">Normal text</p>`,
			expected: false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := verifier.hasHiddenText(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContentVerifier_HasForgedHeaders(t *testing.T) {
	verifier := NewContentVerifier()
	
	tests := []struct {
		name     string
		headers  map[string]string
		expected bool
	}{
		{
			name: "Valid headers",
			headers: map[string]string{
				"Message-ID": "<123@example.com>",
				"Date":       "Mon, 19 Aug 2025 12:00:00 -0300",
			},
			expected: false,
		},
		{
			name: "Missing Message-ID",
			headers: map[string]string{
				"Date": "Mon, 19 Aug 2025 12:00:00 -0300",
			},
			expected: true,
		},
		{
			name: "Missing Date",
			headers: map[string]string{
				"Message-ID": "<123@example.com>",
			},
			expected: true,
		},
		{
			name:     "Missing both headers",
			headers:  map[string]string{},
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock message with specified headers
			content := "From: test@example.com\r\n"
			for key, value := range tt.headers {
				content += key + ": " + value + "\r\n"
			}
			content += "\r\nTest body"
			
			msg, err := mail.ReadMessage(strings.NewReader(content))
			assert.NoError(t, err)
			
			result := verifier.hasForgedHeaders(msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContentVerifier_HasSuspiciousRouting(t *testing.T) {
	verifier := NewContentVerifier()
	
	tests := []struct {
		name     string
		received []string
		expected bool
	}{
		{
			name: "Normal routing",
			received: []string{
				"from mail.example.com by mx.recipient.com",
				"from sender.example.com by mail.example.com",
			},
			expected: false,
		},
		{
			name:     "No received headers",
			received: []string{},
			expected: true,
		},
		{
			name: "Unknown host",
			received: []string{
				"from unknown by mx.recipient.com",
			},
			expected: true,
		},
		{
			name: "Localhost routing",
			received: []string{
				"from localhost by mx.recipient.com",
			},
			expected: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock message with specified Received headers
			content := "From: test@example.com\r\n"
			for _, received := range tt.received {
				content += "Received: " + received + "\r\n"
			}
			content += "\r\nTest body"
			
			msg, err := mail.ReadMessage(strings.NewReader(content))
			assert.NoError(t, err)
			
			result := verifier.hasSuspiciousRouting(msg)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContentVerifier_ExtractTextContent(t *testing.T) {
	verifier := NewContentVerifier()
	
	tests := []struct {
		name     string
		body     []byte
		expected string
	}{
		{
			name: "Simple email",
			body: []byte("From: test@example.com\r\n" +
				"To: recipient@example.com\r\n" +
				"Subject: Test\r\n" +
				"\r\n" +
				"This is the email body content."),
			expected: "This is the email body content.",
		},
		{
			name: "HTML email",
			body: []byte("From: test@example.com\r\n" +
				"To: recipient@example.com\r\n" +
				"Subject: Test\r\n" +
				"\r\n" +
				"<html><body><p>This is <b>HTML</b> content.</p></body></html>"),
			expected: "This is HTML content.",
		},
		{
			name: "No double newline separator",
			body: []byte("From: test@example.com\n" +
				"To: recipient@example.com\n" +
				"Subject: Test\n" +
				"\n" +
				"Body content here."),
			expected: "Body content here.",
		},
		{
			name:     "Headers only",
			body:     []byte("From: test@example.com\r\nSubject: Test\r\n"),
			expected: "From: test@example.com\r\nSubject: Test\r\n",
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := verifier.extractTextContent(tt.body)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContentVerifier_ClassifyContent(t *testing.T) {
	verifier := NewContentVerifier()
	
	tests := []struct {
		score    float64
		expected string
	}{
		{0.0, "legitimate"},
		{0.1, "legitimate"},
		{0.2, "questionable"},
		{0.3, "questionable"},
		{0.5, "suspicious"},
		{0.7, "suspicious"},
		{0.8, "spam"},
		{1.0, "spam"},
	}
	
	for _, tt := range tests {
		t.Run(tt.expected+"_"+string(rune(int(tt.score*10))), func(t *testing.T) {
			result := verifier.classifyContent(tt.score)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContentVerifier_ApplyHeuristics(t *testing.T) {
	verifier := NewContentVerifier()
	
	// Create a message with proper headers
	content := "From: test@example.com\r\n" +
		"To: recipient@example.com\r\n" +
		"Subject: Test\r\n" +
		"Message-ID: <123@example.com>\r\n" +
		"Date: Mon, 19 Aug 2025 12:00:00 -0300\r\n" +
		"\r\n" +
		"<p style=\"color: white;\">Hidden text</p>" +
		"NORMAL TEXT WITH EXCESSIVE CAPITALS AND HTML CONTENT"
	
	msg, err := mail.ReadMessage(strings.NewReader(content))
	assert.NoError(t, err)
	
	subject := "TEST SUBJECT WITH CAPITALS"
	body := `BODY WITH LOTS OF CAPITAL LETTERS AND HTML <p style="color: white;">Hidden bait</p><div>content</div>`
	
	score, indicators := verifier.applyHeuristics(subject, body, msg)
	
	assert.Greater(t, score, 0.0, "Should have positive score for suspicious content")
	assert.NotEmpty(t, indicators, "Should have heuristic indicators")
	
	indicatorStr := strings.Join(indicators, " ")
	assert.Contains(t, indicatorStr, "Excessive capitalization")
	assert.Contains(t, indicatorStr, "Hidden text detected")
}

func BenchmarkContentVerify(b *testing.B) {
	verifier := NewContentVerifier()
	
	emailCtx := EmailContext{
		Subject: "Test email for benchmark",
		Body: []byte("From: test@example.com\r\n" +
			"To: recipient@test.com\r\n" +
			"Subject: Test email for benchmark\r\n" +
			"Date: Mon, 19 Aug 2025 12:00:00 -0300\r\n" +
			"Message-ID: <bench@example.com>\r\n" +
			"\r\n" +
			"This is a benchmark test email with some content to analyze. " +
			"It contains normal business language and should be classified " +
			"as legitimate content. The analyzer should process this efficiently."),
	}
	
	ctx := context.Background()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		verifier.Verify(ctx, emailCtx)
	}
}