package emailrender

import (
	"time"

	"github.com/gofrs/uuid/v5"
)

// DisplayMode represents different ways to display email content
type DisplayMode string

const (
	DisplayModeAuto     DisplayMode = "auto"     // Smart detection (prefer plain text, fallback to HTML)
	DisplayModePlain    DisplayMode = "plain"    // Plain text only
	DisplayModeHTML     DisplayMode = "html"     // Sanitized HTML
	DisplayModeRaw      DisplayMode = "raw"      // Raw content without parsing
	DisplayModeMarkdown DisplayMode = "markdown" // Convert HTML to markdown
)

// ParsedEmail represents a fully parsed email with all content types extracted
type ParsedEmail struct {
	// Basic metadata
	Headers     map[string]string `json:"headers"`
	ContentType string            `json:"content_type"`
	IsMultipart bool              `json:"is_multipart"`

	// Content variants
	PlainText string `json:"plain_text,omitempty"`
	HTML      string `json:"html,omitempty"`

	// Attachments and inline content
	Attachments  []Attachment  `json:"attachments,omitempty"`
	InlineImages []InlineImage `json:"inline_images,omitempty"`

	// Parsed metadata
	Subject   string    `json:"subject"`
	From      string    `json:"from"`
	To        []string  `json:"to,omitempty"`
	CC        []string  `json:"cc,omitempty"`
	BCC       []string  `json:"bcc,omitempty"`
	Date      time.Time `json:"date,omitempty"`
	MessageID string    `json:"message_id,omitempty"`

	// Statistics
	PlainTextSize int `json:"plain_text_size"`
	HTMLSize      int `json:"html_size"`
	TotalSize     int `json:"total_size"`
}

// Attachment represents an email attachment
type Attachment struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Content     []byte `json:"-"`             // Not serialized in JSON for API responses
	CID         string `json:"cid,omitempty"` // Content-ID for inline attachments
}

// InlineImage represents an inline image in HTML email
type InlineImage struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	Content     []byte `json:"-"`   // Not serialized in JSON
	CID         string `json:"cid"` // Content-ID for referencing in HTML
}

// ContentFormat represents the format of email content for API responses
type ContentFormat struct {
	Type    string `json:"type"` // "text/plain", "text/html", "text/markdown"
	Content string `json:"content"`
	Size    int    `json:"size"`
}

// EmailResponse represents the API response for email content
type EmailResponse struct {
	ID      uuid.UUID `json:"id"`
	Subject string    `json:"subject"`
	From    string    `json:"from"`
	Date    time.Time `json:"date"`

	// Available content formats
	AvailableFormats []string        `json:"available_formats"`
	Content          []ContentFormat `json:"content"`

	// Metadata
	IsMultipart      bool `json:"is_multipart"`
	HasAttachments   bool `json:"has_attachments"`
	AttachmentCount  int  `json:"attachment_count"`
	InlineImageCount int  `json:"inline_image_count"`

	// Attachments metadata (content served separately)
	Attachments []AttachmentInfo `json:"attachments,omitempty"`
}

// AttachmentInfo represents attachment metadata for API responses
type AttachmentInfo struct {
	ID          string `json:"id"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int64  `json:"size"`
	IsInline    bool   `json:"is_inline"`
	CID         string `json:"cid,omitempty"`
}

// ParseOptions represents options for email parsing
type ParseOptions struct {
	ExtractAttachments  bool  `json:"extract_attachments"`
	ExtractInlineImages bool  `json:"extract_inline_images"`
	MaxAttachmentSize   int64 `json:"max_attachment_size"` // In bytes, 0 = no limit
	SanitizeHTML        bool  `json:"sanitize_html"`
}

// DefaultParseOptions returns sensible defaults for email parsing
func DefaultParseOptions() ParseOptions {
	return ParseOptions{
		ExtractAttachments:  true,
		ExtractInlineImages: true,
		MaxAttachmentSize:   10 * 1024 * 1024, // 10MB
		SanitizeHTML:        true,
	}
}
