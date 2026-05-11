package emailrender

import (
	"bytes"
	"fmt"
	"mime"
	"strings"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/jhillyerd/enmime"
)

// Parser handles email parsing and content extraction
type Parser struct {
	options ParseOptions
}

// NewParser creates a new email parser with the given options
func NewParser(options ParseOptions) *Parser {
	return &Parser{
		options: options,
	}
}

// NewDefaultParser creates a new email parser with default options
func NewDefaultParser() *Parser {
	return NewParser(DefaultParseOptions())
}

// ParseEmail parses raw email bytes into a structured ParsedEmail
func (p *Parser) ParseEmail(rawEmail []byte) (*ParsedEmail, error) {
	// Parse the email using enmime
	envelope, err := enmime.ReadEnvelope(bytes.NewReader(rawEmail))
	if err != nil {
		return nil, fmt.Errorf("failed to parse email: %w", err)
	}

	// Extract basic information
	parsed := &ParsedEmail{
		Headers:     make(map[string]string),
		Subject:     envelope.GetHeader("Subject"),
		From:        envelope.GetHeader("From"),
		MessageID:   envelope.GetHeader("Message-ID"),
		ContentType: envelope.GetHeader("Content-Type"),
		PlainText:   envelope.Text,
		HTML:        envelope.HTML,
		TotalSize:   len(rawEmail),
	}

	// Parse date
	if dateStr := envelope.GetHeader("Date"); dateStr != "" {
		if parsedDate, err := time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", dateStr); err == nil {
			parsed.Date = parsedDate
		}
	}

	// Extract recipients
	parsed.To = parseAddressList(envelope.GetHeader("To"))
	parsed.CC = parseAddressList(envelope.GetHeader("CC"))
	parsed.BCC = parseAddressList(envelope.GetHeader("BCC"))

	// Copy headers
	for _, key := range envelope.GetHeaderKeys() {
		parsed.Headers[key] = envelope.GetHeader(key)
	}

	// Set content sizes
	parsed.PlainTextSize = len(parsed.PlainText)
	parsed.HTMLSize = len(parsed.HTML)

	// Determine if multipart
	parsed.IsMultipart = strings.HasPrefix(strings.ToLower(parsed.ContentType), "multipart/")

	// Extract attachments if requested
	if p.options.ExtractAttachments {
		parsed.Attachments = p.extractAttachments(envelope)
	}

	// Extract inline images if requested
	if p.options.ExtractInlineImages {
		parsed.InlineImages = p.extractInlineImages(envelope)
	}

	return parsed, nil
}

// extractAttachments extracts attachments from the email envelope
func (p *Parser) extractAttachments(envelope *enmime.Envelope) []Attachment {
	var attachments []Attachment

	for _, attachment := range envelope.Attachments {
		// Check size limit
		if p.options.MaxAttachmentSize > 0 && int64(len(attachment.Content)) > p.options.MaxAttachmentSize {
			continue
		}

		// Decode filename if needed
		filename := attachment.FileName
		if decodedName, err := (&mime.WordDecoder{}).DecodeHeader(filename); err == nil {
			filename = decodedName
		}

		att := Attachment{
			ID:          uuid.Must(uuid.NewV4()).String(),
			Filename:    filename,
			ContentType: attachment.ContentType,
			Size:        int64(len(attachment.Content)),
			Content:     attachment.Content,
		}

		attachments = append(attachments, att)
	}

	return attachments
}

// extractInlineImages extracts inline images from the email envelope
func (p *Parser) extractInlineImages(envelope *enmime.Envelope) []InlineImage {
	var inlineImages []InlineImage

	for _, inline := range envelope.Inlines {
		// Check if it's an image
		if !strings.HasPrefix(inline.ContentType, "image/") {
			continue
		}

		// Check size limit
		if p.options.MaxAttachmentSize > 0 && int64(len(inline.Content)) > p.options.MaxAttachmentSize {
			continue
		}

		// Decode filename if needed
		filename := inline.FileName
		if decodedName, err := (&mime.WordDecoder{}).DecodeHeader(filename); err == nil {
			filename = decodedName
		}

		img := InlineImage{
			ID:          uuid.Must(uuid.NewV4()).String(),
			Filename:    filename,
			ContentType: inline.ContentType,
			Size:        int64(len(inline.Content)),
			Content:     inline.Content,
			CID:         inline.ContentID,
		}

		inlineImages = append(inlineImages, img)
	}

	return inlineImages
}

// parseAddressList parses a comma-separated list of email addresses
func parseAddressList(addressStr string) []string {
	if addressStr == "" {
		return nil
	}

	// Simple parsing - in a real implementation, you might want to use
	// mail.ParseAddressList for proper RFC parsing
	addresses := strings.Split(addressStr, ",")
	var result []string
	for _, addr := range addresses {
		trimmed := strings.TrimSpace(addr)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

// GetBestContent returns the best content for the given display mode
func (p *ParsedEmail) GetBestContent(mode DisplayMode) (string, string) {
	switch mode {
	case DisplayModeAuto:
		// Prefer plain text, fallback to HTML
		if p.PlainText != "" {
			return p.PlainText, "text/plain"
		}
		if p.HTML != "" {
			return p.HTML, "text/html"
		}
		return "", ""

	case DisplayModePlain:
		if p.PlainText != "" {
			return p.PlainText, "text/plain"
		}
		// Convert HTML to plain text if available
		if p.HTML != "" {
			return htmlToPlainText(p.HTML), "text/plain"
		}
		return "", ""

	case DisplayModeHTML:
		if p.HTML != "" {
			return p.HTML, "text/html"
		}
		// Convert plain text to simple HTML if available
		if p.PlainText != "" {
			return plainTextToHTML(p.PlainText), "text/html"
		}
		return "", ""

	case DisplayModeMarkdown:
		if p.HTML != "" {
			return htmlToMarkdown(p.HTML), "text/markdown"
		}
		if p.PlainText != "" {
			return plainTextToMarkdown(p.PlainText), "text/markdown"
		}
		return "", ""

	case DisplayModeRaw:
		// Return whatever content is available as-is
		if p.HTML != "" {
			return p.HTML, "text/html"
		}
		if p.PlainText != "" {
			return p.PlainText, "text/plain"
		}
		return "", ""

	default:
		return p.GetBestContent(DisplayModeAuto)
	}
}

// GetAvailableFormats returns a list of available content formats
func (p *ParsedEmail) GetAvailableFormats() []string {
	var formats []string

	if p.PlainText != "" {
		formats = append(formats, "text/plain")
	}
	if p.HTML != "" {
		formats = append(formats, "text/html", "text/markdown")
	}

	// Always allow raw mode
	formats = append(formats, "raw")

	return formats
}

// GetAllContentFormats returns all available content in different formats
func (p *ParsedEmail) GetAllContentFormats() []ContentFormat {
	var formats []ContentFormat

	if p.PlainText != "" {
		formats = append(formats, ContentFormat{
			Type:    "text/plain",
			Content: p.PlainText,
			Size:    len(p.PlainText),
		})
	}

	if p.HTML != "" {
		formats = append(formats, ContentFormat{
			Type:    "text/html",
			Content: p.HTML,
			Size:    len(p.HTML),
		})

		// Add markdown version of HTML
		markdown := htmlToMarkdown(p.HTML)
		formats = append(formats, ContentFormat{
			Type:    "text/markdown",
			Content: markdown,
			Size:    len(markdown),
		})
	}

	return formats
}

// HasAttachments returns true if the email has attachments
func (p *ParsedEmail) HasAttachments() bool {
	return len(p.Attachments) > 0
}

// HasInlineImages returns true if the email has inline images
func (p *ParsedEmail) HasInlineImages() bool {
	return len(p.InlineImages) > 0
}

// GetAttachmentByID returns an attachment by its ID
func (p *ParsedEmail) GetAttachmentByID(id string) *Attachment {
	for i := range p.Attachments {
		if p.Attachments[i].ID == id {
			return &p.Attachments[i]
		}
	}
	return nil
}

// GetInlineImageByID returns an inline image by its ID
func (p *ParsedEmail) GetInlineImageByID(id string) *InlineImage {
	for i := range p.InlineImages {
		if p.InlineImages[i].ID == id {
			return &p.InlineImages[i]
		}
	}
	return nil
}
