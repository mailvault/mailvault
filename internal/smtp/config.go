package smtp

import "crypto/tls"

// Config holds SMTP server configuration
type Config struct {
	Addr      string `conf:"env:SMTP_ADDR,default:127.0.0.1:2525"`
	Domain    string `conf:"env:SMTP_DOMAIN,default:localhost"`
	TLSCert   string `conf:"env:SMTP_TLS_CERT,default:cert.pem"`
	TLSKey    string `conf:"env:SMTP_TLS_KEY,default:key.pem"`
	TLSConfig *tls.Config
}
