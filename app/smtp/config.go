package smtp

import "crypto/tls"

// TLSMode defines the type of TLS configuration to use
type TLSMode string

const (
	TLSModeOff  TLSMode = "off"  // No TLS
	TLSModeCert TLSMode = "cert" // Use certificate files
)

// Config holds SMTP server configuration
type Config struct {
	Addr   string
	Domain string
	Debug  bool

	// TLS Configuration
	TLSMode TLSMode
	TLSCert string
	TLSKey  string
	// If true, the listener will expect TLS from the start (implicit TLS, e.g. port 465).
	// If false and TLSMode is cert, STARTTLS will be offered on the plaintext connection (e.g. port 25/587).
	TLSImplicit bool

	// Runtime TLS config (set programmatically)
	TLSConfig *tls.Config
}
