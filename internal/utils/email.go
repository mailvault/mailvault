package utils

import (
	"fmt"
	"regexp"
	"strings"
)

// EmailParseError represents an error that occurred during email parsing
type EmailParseError struct {
	Input string
	Msg   string
}

func (e *EmailParseError) Error() string {
	return fmt.Sprintf("invalid email format '%s': %s", e.Input, e.Msg)
}

// ParseEmailAddress parses an email address and returns the local part and domain
// It handles common formatting issues like angle brackets, whitespace, and quotes
func ParseEmailAddress(email string) (localPart, domain string, err error) {
	if email == "" {
		return "", "", &EmailParseError{Input: email, Msg: "empty email address"}
	}

	// Clean and normalize the email address
	cleaned := cleanEmailAddress(email)

	// Split on @ symbol
	parts := strings.Split(cleaned, "@")
	if len(parts) != 2 {
		return "", "", &EmailParseError{Input: email, Msg: "must contain exactly one @ symbol"}
	}

	localPart = strings.TrimSpace(parts[0])
	domain = strings.TrimSpace(parts[1])

	// Validate local part
	if localPart == "" {
		return "", "", &EmailParseError{Input: email, Msg: "local part cannot be empty"}
	}

	// Validate and clean domain
	domain, err = cleanDomain(domain)
	if err != nil {
		return "", "", &EmailParseError{Input: email, Msg: err.Error()}
	}

	return localPart, domain, nil
}

// ExtractDomain extracts just the domain part from an email address
// This is a convenience function for cases where only the domain is needed
func ExtractDomain(email string) (string, error) {
	_, domain, err := ParseEmailAddress(email)
	return domain, err
}

// ValidateEmailFormat validates that an email address has the correct basic format
func ValidateEmailFormat(email string) error {
	_, _, err := ParseEmailAddress(email)
	return err
}

// cleanEmailAddress removes common formatting artifacts from email addresses
func cleanEmailAddress(email string) string {
	// Trim whitespace
	email = strings.TrimSpace(email)

	// Handle RFC-style email addresses like "Display Name" <user@domain.com>
	// Look for angle brackets and extract the content
	if strings.Contains(email, "<") && strings.Contains(email, ">") {
		startIdx := strings.Index(email, "<")
		endIdx := strings.Index(email, ">")
		if startIdx < endIdx {
			email = email[startIdx+1 : endIdx]
		}
	}

	// Remove any remaining angle brackets that might be malformed
	email = strings.Trim(email, "<>")

	// Remove quotes if they wrap the entire address
	email = strings.Trim(email, "\"'")

	// Final whitespace trim
	email = strings.TrimSpace(email)

	return email
}

// cleanDomain cleans and validates a domain name
func cleanDomain(domain string) (string, error) {
	if domain == "" {
		return "", fmt.Errorf("domain cannot be empty")
	}

	// Keep track of original for error reporting
	original := domain

	// Remove leading characters
	domain = strings.TrimLeft(domain, "<\"', \t\n\r")

	// Convert to lowercase for consistency
	domain = strings.ToLower(domain)

	// Check if the original domain (after trimming) ends with just a dot (invalid)
	// vs ending with ">." or something similar (which should be cleaned)
	if strings.HasSuffix(original, ".") && !strings.Contains(original, ">") && !strings.Contains(original, "\"") {
		return "", fmt.Errorf("domain cannot end with dot")
	}

	// Remove common trailing characters that might be artifacts
	domain = strings.TrimRight(domain, ">\"', \t\n\r")

	// Special case: also trim trailing dots that come from cleaning
	domain = strings.TrimRight(domain, ".")

	// Basic validation - domain should contain at least one dot and no spaces
	if !strings.Contains(domain, ".") {
		return "", fmt.Errorf("domain must contain at least one dot")
	}

	if strings.Contains(domain, " ") {
		return "", fmt.Errorf("domain cannot contain spaces")
	}

	// Check for valid domain characters (basic check)
	// Allow alphanumeric, dots, hyphens
	validDomainRegex := regexp.MustCompile(`^[a-zA-Z0-9.-]+$`)
	if !validDomainRegex.MatchString(domain) {
		return "", fmt.Errorf("domain contains invalid characters")
	}

	// Domain shouldn't start or end with dot or hyphen after all cleaning
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") ||
		strings.HasPrefix(domain, "-") || strings.HasSuffix(domain, "-") {
		return "", fmt.Errorf("domain cannot start or end with dot or hyphen")
	}

	return domain, nil
}

// NormalizeDomain normalizes a domain name (lowercase, trimmed)
// This is useful for consistent storage and comparison
func NormalizeDomain(domain string) string {
	cleaned, err := cleanDomain(domain)
	if err != nil {
		// If cleaning fails, just do basic normalization
		return strings.ToLower(strings.TrimSpace(domain))
	}
	return cleaned
}

// SplitEmailAddressSafe is a safe alternative to strings.Split(email, "@")
// It uses the robust parsing logic but maintains backward compatibility
func SplitEmailAddressSafe(email string) (localPart, domain string) {
	local, dom, err := ParseEmailAddress(email)
	if err != nil {
		// Fallback to simple split for backward compatibility
		parts := strings.Split(email, "@")
		if len(parts) == 2 {
			return strings.TrimSpace(parts[0]), NormalizeDomain(parts[1])
		}
		return email, ""
	}
	return local, dom
}