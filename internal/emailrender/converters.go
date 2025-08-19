package emailrender

import (
	"fmt"
	"html"
	"strings"

	md "github.com/JohannesKaufmann/html-to-markdown"
	"github.com/JohannesKaufmann/html-to-markdown/plugin"
	"github.com/jaytaylor/html2text"
)

// htmlToPlainText converts HTML content to plain text
func htmlToPlainText(htmlContent string) string {
	if htmlContent == "" {
		return ""
	}

	// Use html2text library for better formatting
	text, err := html2text.FromString(htmlContent, html2text.Options{
		PrettyTables: true,
		OmitLinks:    false,
	})
	if err != nil {
		// Fallback: strip HTML tags manually
		return stripHTMLTags(htmlContent)
	}

	return text
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

// stripHTMLTags removes HTML tags from text (basic fallback)
func stripHTMLTags(htmlContent string) string {
	// This is a very basic implementation
	// In a production environment, you might want to use a proper HTML parser
	text := htmlContent
	
	// Remove script and style content entirely
	text = removeTagContent(text, "script")
	text = removeTagContent(text, "style")
	
	// Replace common block elements with line breaks
	blockElements := []string{"div", "p", "br", "h1", "h2", "h3", "h4", "h5", "h6", "li"}
	for _, tag := range blockElements {
		text = strings.ReplaceAll(text, fmt.Sprintf("<%s>", tag), "\n")
		text = strings.ReplaceAll(text, fmt.Sprintf("</%s>", tag), "\n")
		text = strings.ReplaceAll(text, fmt.Sprintf("<%s/>", tag), "\n")
	}
	
	// Remove all remaining HTML tags
	inTag := false
	var result strings.Builder
	for _, char := range text {
		if char == '<' {
			inTag = true
		} else if char == '>' {
			inTag = false
		} else if !inTag {
			result.WriteRune(char)
		}
	}
	
	// Clean up multiple line breaks
	cleaned := strings.ReplaceAll(result.String(), "\n\n\n", "\n\n")
	return strings.TrimSpace(cleaned)
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

