package emailrender

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/mailvault/mailvault/domain/entities"
	"github.com/mailvault/mailvault/internal/encryption"
)

// RenderEngine is the main interface for email rendering
type RenderEngine struct {
	parser    *Parser
	sanitizer *Sanitizer
}

// NewRenderEngine creates a new email rendering engine
func NewRenderEngine() *RenderEngine {
	return &RenderEngine{
		parser:    NewDefaultParser(),
		sanitizer: NewSanitizer(),
	}
}

// NewRenderEngineWithOptions creates a new engine with custom options
func NewRenderEngineWithOptions(parseOptions ParseOptions, strict bool) *RenderEngine {
	var sanitizer *Sanitizer
	if strict {
		sanitizer = NewStrictSanitizer()
	} else {
		sanitizer = NewSanitizer()
	}

	return &RenderEngine{
		parser:    NewParser(parseOptions),
		sanitizer: sanitizer,
	}
}

// ParseFromEncrypted decrypts and parses an encrypted email
func (r *RenderEngine) ParseFromEncrypted(encryptedBody, privateKeyBase64 string) (*ParsedEmail, error) {
	// Deserialize the encrypted data
	encryptedData, err := encryption.DeserializeEncryptedData(encryptedBody)
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize encrypted data: %w", err)
	}

	// Decode private key from base64
	privateKey, err := base64.StdEncoding.DecodeString(privateKeyBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	// Decrypt the email
	decryptedContent, err := encryption.Decrypt(encryptedData, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt email: %w", err)
	}

	return r.ParseFromBytes(decryptedContent)
}

// ParseFromBytes parses raw email bytes
func (r *RenderEngine) ParseFromBytes(rawEmail []byte) (*ParsedEmail, error) {
	parsed, err := r.parser.ParseEmail(rawEmail)
	if err != nil {
		return nil, err
	}

	// Sanitize HTML content if present
	if parsed.HTML != "" {
		parsed.HTML = r.sanitizer.SanitizeHTML(parsed.HTML)
	}

	return parsed, nil
}

// RenderForDisplay renders email content for display in the specified mode
func (r *RenderEngine) RenderForDisplay(parsed *ParsedEmail, mode DisplayMode) (string, string, error) {
	content, contentType := parsed.GetBestContent(mode)
	if content == "" {
		return "", "", fmt.Errorf("no content available for display mode %s", mode)
	}

	// Apply additional sanitization for HTML content
	if contentType == "text/html" {
		content = r.sanitizer.SanitizeHTML(content)
	}

	return content, contentType, nil
}

// CreateEmailResponse creates an API response from parsed email
func (r *RenderEngine) CreateEmailResponse(receivedEmail *entities.ReceivedEmail, parsed *ParsedEmail) *EmailResponse {
	response := &EmailResponse{
		ID:      receivedEmail.ID,
		Subject: parsed.Subject,
		From:    parsed.From,
		Date:    parsed.Date,

		AvailableFormats: parsed.GetAvailableFormats(),
		Content:          parsed.GetAllContentFormats(),
		IsMultipart:      parsed.IsMultipart,
		HasAttachments:   parsed.HasAttachments(),
		AttachmentCount:  len(parsed.Attachments),
		InlineImageCount: len(parsed.InlineImages),
	}

	// Add attachment info (without content)
	for _, att := range parsed.Attachments {
		info := AttachmentInfo{
			ID:          att.ID,
			Filename:    att.Filename,
			ContentType: att.ContentType,
			Size:        att.Size,
			IsInline:    false,
		}
		response.Attachments = append(response.Attachments, info)
	}

	// Add inline image info
	for _, img := range parsed.InlineImages {
		info := AttachmentInfo{
			ID:          img.ID,
			Filename:    img.Filename,
			ContentType: img.ContentType,
			Size:        img.Size,
			IsInline:    true,
			CID:         img.CID,
		}
		response.Attachments = append(response.Attachments, info)
	}

	return response
}

// FormatForCLI formats email content for CLI display
func (r *RenderEngine) FormatForCLI(parsed *ParsedEmail, mode DisplayMode, maxWidth int) string {
	var builder strings.Builder

	// Header
	builder.WriteString("📧 Email Content\n")
	builder.WriteString(strings.Repeat("=", maxWidth) + "\n")

	// Metadata
	builder.WriteString(fmt.Sprintf("From: %s\n", parsed.From))
	builder.WriteString(fmt.Sprintf("Subject: %s\n", parsed.Subject))
	if !parsed.Date.IsZero() {
		builder.WriteString(fmt.Sprintf("Date: %s\n", parsed.Date.Format("2006-01-02 15:04:05")))
	}

	if len(parsed.To) > 0 {
		builder.WriteString(fmt.Sprintf("To: %s\n", strings.Join(parsed.To, ", ")))
	}

	// Content info
	if parsed.IsMultipart {
		builder.WriteString("Type: Multipart email\n")
	}
	if parsed.HasAttachments() {
		builder.WriteString(fmt.Sprintf("Attachments: %d files\n", len(parsed.Attachments)))
	}
	if parsed.HasInlineImages() {
		builder.WriteString(fmt.Sprintf("Inline Images: %d files\n", len(parsed.InlineImages)))
	}

	builder.WriteString(strings.Repeat("-", maxWidth) + "\n")

	// Content
	content, contentType, err := r.RenderForDisplay(parsed, mode)
	if err != nil {
		builder.WriteString(fmt.Sprintf("Error rendering content: %s\n", err))
		return builder.String()
	}

	// Display mode indicator
	builder.WriteString(fmt.Sprintf("Content (%s, %s):\n", mode, contentType))
	builder.WriteString(strings.Repeat("-", maxWidth) + "\n")

	// Format content based on type
	if contentType == "text/html" {
		builder.WriteString(r.formatHTMLForCLI(content, maxWidth))
	} else {
		builder.WriteString(r.formatPlainTextForCLI(content, maxWidth))
	}

	builder.WriteString("\n" + strings.Repeat("=", maxWidth) + "\n")

	// Attachment list
	if parsed.HasAttachments() {
		builder.WriteString("\nAttachments:\n")
		for i, att := range parsed.Attachments {
			builder.WriteString(fmt.Sprintf("  %d. %s (%s, %d bytes)\n",
				i+1, att.Filename, att.ContentType, att.Size))
		}
	}

	return builder.String()
}

// formatHTMLForCLI formats HTML content for CLI display
func (r *RenderEngine) formatHTMLForCLI(html string, maxWidth int) string {
	// For CLI, we'll show a plain text version of HTML
	plainText := htmlToPlainText(html)
	return r.formatPlainTextForCLI(plainText, maxWidth)
}

// formatPlainTextForCLI formats plain text content for CLI display
func (r *RenderEngine) formatPlainTextForCLI(text string, maxWidth int) string {
	if maxWidth <= 0 {
		maxWidth = 80 // Default width
	}

	var builder strings.Builder
	lines := strings.Split(text, "\n")

	for _, line := range lines {
		if len(line) <= maxWidth {
			builder.WriteString(line + "\n")
		} else {
			// Wrap long lines
			words := strings.Fields(line)
			currentLine := ""

			for _, word := range words {
				if len(currentLine)+len(word)+1 <= maxWidth {
					if currentLine != "" {
						currentLine += " "
					}
					currentLine += word
				} else {
					if currentLine != "" {
						builder.WriteString(currentLine + "\n")
					}
					currentLine = word
				}
			}

			if currentLine != "" {
				builder.WriteString(currentLine + "\n")
			}
		}
	}

	return builder.String()
}

// GetDisplayModeFromString converts string to DisplayMode
func GetDisplayModeFromString(mode string) DisplayMode {
	switch strings.ToLower(mode) {
	case "auto":
		return DisplayModeAuto
	case "plain":
		return DisplayModePlain
	case "html":
		return DisplayModeHTML
	case "markdown", "md":
		return DisplayModeMarkdown
	case "raw":
		return DisplayModeRaw
	default:
		return DisplayModeAuto
	}
}
