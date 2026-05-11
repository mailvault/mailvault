package emailrender

import (
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

// Sanitizer handles HTML sanitization for safe email display
type Sanitizer struct {
	policy *bluemonday.Policy
}

// NewSanitizer creates a new HTML sanitizer with email-safe policies
func NewSanitizer() *Sanitizer {
	policy := bluemonday.NewPolicy()

	// Allow safe text formatting elements
	policy.AllowElements(
		"p", "br", "div", "span", "strong", "b", "em", "i", "u", "s", "sub", "sup",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"blockquote", "pre", "code",
	)

	// Allow lists
	policy.AllowElements("ul", "ol", "li")

	// Allow tables
	policy.AllowElements("table", "thead", "tbody", "tfoot", "tr", "td", "th", "caption")

	// Allow links but make them safe
	policy.AllowElements("a")
	policy.AllowAttrs("href").OnElements("a")
	policy.RequireNoReferrerOnLinks(true)
	policy.RequireNoFollowOnLinks(true)
	policy.AllowAttrs("title").OnElements("a")

	// Allow images but restrict to safe attributes
	policy.AllowElements("img")
	policy.AllowAttrs("src", "alt", "title", "width", "height").OnElements("img")

	// Allow inline images (data: URLs)
	policy.AllowDataURIImages()
	// Note: cid: URLs for inline images will be handled separately

	// Allow safe styling attributes (very limited)
	policy.AllowAttrs("style").OnElements(
		"p", "div", "span", "h1", "h2", "h3", "h4", "h5", "h6",
		"table", "tr", "td", "th",
	)

	// Allow specific safe CSS properties
	policy.AllowStyles("color", "background-color", "font-size", "font-weight", "text-align", "margin", "padding").OnElements(
		"p", "div", "span", "h1", "h2", "h3", "h4", "h5", "h6",
		"table", "tr", "td", "th",
	)

	// Allow class attributes for basic styling
	policy.AllowAttrs("class").OnElements(
		"p", "div", "span", "h1", "h2", "h3", "h4", "h5", "h6",
		"table", "tr", "td", "th", "ul", "ol", "li",
	)

	// Explicitly deny dangerous elements and attributes
	policy.SkipElementsContent("script", "style", "object", "embed", "iframe", "frame", "frameset")

	return &Sanitizer{
		policy: policy,
	}
}

// NewStrictSanitizer creates a very strict sanitizer that only allows basic text formatting
func NewStrictSanitizer() *Sanitizer {
	policy := bluemonday.StrictPolicy()

	// Only allow the most basic formatting
	policy.AllowElements("p", "br", "strong", "b", "em", "i", "u")

	return &Sanitizer{
		policy: policy,
	}
}

// SanitizeHTML sanitizes HTML content to make it safe for display
func (s *Sanitizer) SanitizeHTML(html string) string {
	if html == "" {
		return ""
	}

	// Remove any potential script injections
	sanitized := s.policy.Sanitize(html)

	// Additional cleaning for email-specific concerns
	sanitized = s.cleanEmailSpecific(sanitized)

	return sanitized
}

// cleanEmailSpecific performs additional email-specific cleaning
func (s *Sanitizer) cleanEmailSpecific(html string) string {
	// Remove any remaining event handlers that might have slipped through
	eventHandlers := []string{
		"onclick", "onload", "onunload", "onchange", "onsubmit", "onreset",
		"onselect", "onblur", "onfocus", "onkeydown", "onkeypress", "onkeyup",
		"onmousedown", "onmousemove", "onmouseout", "onmouseover", "onmouseup",
	}

	cleaned := html
	for _, handler := range eventHandlers {
		// Remove both with and without spaces around the equals sign
		cleaned = removeAttribute(cleaned, handler)
	}

	// Remove any remaining javascript: URLs
	cleaned = strings.ReplaceAll(cleaned, "javascript:", "")
	cleaned = strings.ReplaceAll(cleaned, "vbscript:", "")

	// Remove data: URLs that could be dangerous (except images)
	if !strings.Contains(cleaned, "<img") {
		cleaned = strings.ReplaceAll(cleaned, "data:", "")
	}

	return cleaned
}

// removeAttribute removes HTML attributes from content
func removeAttribute(html, attrName string) string {
	// Simple regex-like replacement to remove attributes
	// This is a basic implementation - in production you might want more sophisticated parsing
	patterns := []string{
		attrName + "=",
		strings.ToUpper(attrName) + "=",
		strings.ToUpper(attrName[:1]) + strings.ToLower(attrName[1:]) + "=",
	}

	for _, pattern := range patterns {
		for {
			start := strings.Index(html, pattern)
			if start == -1 {
				break
			}

			// Find the end of the attribute value
			valueStart := start + len(pattern)
			var end int

			if valueStart < len(html) {
				quote := html[valueStart]
				if quote == '"' || quote == '\'' {
					// Quoted value
					end = strings.Index(html[valueStart+1:], string(quote))
					if end != -1 {
						end += valueStart + 2
					} else {
						end = valueStart + 1
					}
				} else {
					// Unquoted value - find next space or >
					end = valueStart
					for end < len(html) && html[end] != ' ' && html[end] != '>' {
						end++
					}
				}
			} else {
				end = valueStart
			}

			if end > valueStart {
				html = html[:start] + html[end:]
			} else {
				break
			}
		}
	}

	return html
}

// GetSanitizedHTML is a convenience method that combines parsing and sanitization
func (p *ParsedEmail) GetSanitizedHTML() string {
	if p.HTML == "" {
		return ""
	}

	sanitizer := NewSanitizer()
	return sanitizer.SanitizeHTML(p.HTML)
}

// IsHTMLSafe checks if HTML content appears to be safe (basic heuristic)
func IsHTMLSafe(html string) bool {
	if html == "" {
		return true
	}

	// Check for obviously dangerous patterns
	dangerousPatterns := []string{
		"<script", "<object", "<embed", "<iframe", "<frame",
		"javascript:", "vbscript:", "onload=", "onclick=",
		"document.cookie", "document.write", "eval(",
	}

	htmlLower := strings.ToLower(html)
	for _, pattern := range dangerousPatterns {
		if strings.Contains(htmlLower, pattern) {
			return false
		}
	}

	return true
}

// PreviewSafeHTML generates a safe HTML preview with limited content
func PreviewSafeHTML(html string, maxLength int) string {
	if html == "" {
		return ""
	}

	// Sanitize first
	sanitizer := NewStrictSanitizer()
	safe := sanitizer.SanitizeHTML(html)

	// Convert to plain text for preview
	preview := htmlToPlainText(safe)

	// Truncate if needed
	if len(preview) > maxLength {
		preview = truncateContent(preview, maxLength)
	}

	return preview
}
