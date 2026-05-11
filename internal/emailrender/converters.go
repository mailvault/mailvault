package emailrender

import (
	"fmt"
	"html"
	"regexp"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
)

// htmlToPlainText converts HTML content to plain text
func htmlToPlainText(htmlContent string) string {
	if htmlContent == "" {
		return ""
	}

	// Use improved HTML stripping with better formatting
	return stripHTMLTagsImproved(htmlContent)
}

// htmlToMarkdown converts HTML content to Markdown
func htmlToMarkdown(htmlContent string) string {
	if htmlContent == "" {
		return ""
	}

	converter := md.NewConverter("", true, nil)

	// Configure options for email-friendly markdown
	converter.Use(plugin.GitHubFlavored())
	converter.Use(plugin.Table())
	converter.Use(plugin.TaskListItems())

	markdown, err := converter.ConvertString(htmlContent)
	if err != nil {
		// Fallback: convert to plain text
		return htmlToPlainText(htmlContent)
	}

	return markdown
}

// plainTextToHTML converts plain text to simple HTML
func plainTextToHTML(plainText string) string {
	if plainText == "" {
		return ""
	}

	// Escape HTML entities
	escaped := html.EscapeString(plainText)

	// Convert line breaks to <br> tags
	html := strings.ReplaceAll(escaped, "\n", "<br>\n")

	// Wrap in a basic HTML structure
	return fmt.Sprintf(`<div style="font-family: monospace; white-space: pre-wrap;">%s</div>`, html)
}

// plainTextToMarkdown converts plain text to Markdown (minimal conversion)
func plainTextToMarkdown(plainText string) string {
	if plainText == "" {
		return ""
	}

	// For plain text, just add code block formatting to preserve formatting
	return fmt.Sprintf("```\n%s\n```", plainText)
}

// stripHTMLTagsImproved removes HTML tags from text with better formatting
func stripHTMLTagsImproved(htmlContent string) string {
	if htmlContent == "" {
		return ""
	}

	text := htmlContent

	// Remove script and style content entirely
	text = removeTagContent(text, "script")
	text = removeTagContent(text, "style")
	text = removeTagContent(text, "head")

	// Handle common HTML entities
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&#39;", "'")

	// Handle tables - convert to simple text format
	text = handleHTMLTables(text)

	// Handle lists - add proper formatting
	text = handleHTMLLists(text)

	// Replace block elements with appropriate line breaks
	blockElements := map[string]string{
		"p":          "\n\n",
		"div":        "\n",
		"br":         "\n",
		"h1":         "\n\n",
		"h2":         "\n\n",
		"h3":         "\n\n",
		"h4":         "\n\n",
		"h5":         "\n\n",
		"h6":         "\n\n",
		"hr":         "\n---\n",
		"blockquote": "\n> ",
	}

	for tag, replacement := range blockElements {
		// Handle both opening and closing tags
		re := regexp.MustCompile(fmt.Sprintf(`(?i)<%s[^>]*>`, tag))
		text = re.ReplaceAllString(text, replacement)
		text = strings.ReplaceAll(text, fmt.Sprintf("</%s>", tag), "")
	}

	// Handle inline formatting
	text = handleInlineFormatting(text)

	// Remove all remaining HTML tags
	text = removeAllHTMLTags(text)

	// Clean up whitespace
	text = cleanupWhitespace(text)

	return strings.TrimSpace(text)
}

// handleHTMLTables converts HTML tables to simple text format
func handleHTMLTables(text string) string {
	// Simple table handling - convert <td> to spaces and <tr> to newlines
	re := regexp.MustCompile(`(?i)<tr[^>]*>`)
	text = re.ReplaceAllString(text, "\n")

	re = regexp.MustCompile(`(?i)</?tr[^>]*>`)
	text = re.ReplaceAllString(text, "")

	re = regexp.MustCompile(`(?i)<td[^>]*>`)
	text = re.ReplaceAllString(text, " | ")

	re = regexp.MustCompile(`(?i)</?td[^>]*>`)
	text = re.ReplaceAllString(text, "")

	re = regexp.MustCompile(`(?i)</?th[^>]*>`)
	text = re.ReplaceAllString(text, " | ")

	re = regexp.MustCompile(`(?i)</?table[^>]*>`)
	text = re.ReplaceAllString(text, "\n")

	return text
}

// handleHTMLLists converts HTML lists to simple text format
func handleHTMLLists(text string) string {
	// Handle unordered lists
	re := regexp.MustCompile(`(?i)<li[^>]*>`)
	text = re.ReplaceAllString(text, "\n• ")

	re = regexp.MustCompile(`(?i)</?li[^>]*>`)
	text = re.ReplaceAllString(text, "")

	re = regexp.MustCompile(`(?i)</?ul[^>]*>`)
	text = re.ReplaceAllString(text, "\n")

	re = regexp.MustCompile(`(?i)</?ol[^>]*>`)
	text = re.ReplaceAllString(text, "\n")

	return text
}

// handleInlineFormatting converts inline HTML formatting
func handleInlineFormatting(text string) string {
	// Handle links - extract text and optionally show URL
	re := regexp.MustCompile(`(?i)<a[^>]+href=["']([^"']+)["'][^>]*>([^<]+)</a>`)
	text = re.ReplaceAllString(text, "$2")

	// Handle bold/strong - keep text, remove tags
	re = regexp.MustCompile(`(?i)</?(?:b|strong)[^>]*>`)
	text = re.ReplaceAllString(text, "")

	// Handle italic/emphasis - keep text, remove tags
	re = regexp.MustCompile(`(?i)</?(?:i|em)[^>]*>`)
	text = re.ReplaceAllString(text, "")

	// Handle code
	re = regexp.MustCompile(`(?i)</?code[^>]*>`)
	text = re.ReplaceAllString(text, "`")

	return text
}

// removeAllHTMLTags removes any remaining HTML tags
func removeAllHTMLTags(text string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	return re.ReplaceAllString(text, "")
}

// cleanupWhitespace normalizes whitespace in text
func cleanupWhitespace(text string) string {
	// Replace multiple spaces with single space
	re := regexp.MustCompile(`[ \t]+`)
	text = re.ReplaceAllString(text, " ")

	// Replace multiple newlines with maximum of two
	re = regexp.MustCompile(`\n\s*\n\s*\n+`)
	text = re.ReplaceAllString(text, "\n\n")

	// Clean up space before newlines
	re = regexp.MustCompile(`[ \t]+\n`)
	text = re.ReplaceAllString(text, "\n")

	return text
}

// removeTagContent removes content between opening and closing tags
func removeTagContent(text, tag string) string {
	openTag := fmt.Sprintf("<%s", tag)
	closeTag := fmt.Sprintf("</%s>", tag)

	for {
		start := strings.Index(strings.ToLower(text), openTag)
		if start == -1 {
			break
		}

		// Find the end of the opening tag
		tagEnd := strings.Index(text[start:], ">")
		if tagEnd == -1 {
			break
		}
		tagEnd += start + 1

		// Find the closing tag
		end := strings.Index(strings.ToLower(text[tagEnd:]), closeTag)
		if end == -1 {
			break
		}
		end += tagEnd + len(closeTag)

		// Remove the content
		text = text[:start] + text[end:]
	}

	return text
}

// truncateContent truncates content to a reasonable display length
func truncateContent(content string, maxLength int) string {
	if len(content) <= maxLength {
		return content
	}

	// Try to truncate at a word boundary
	truncated := content[:maxLength]
	lastSpace := strings.LastIndex(truncated, " ")
	if lastSpace > maxLength/2 {
		truncated = truncated[:lastSpace]
	}

	return truncated + "..."
}
