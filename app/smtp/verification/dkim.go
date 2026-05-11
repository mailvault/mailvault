package verification

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/emersion/go-msgauth/dkim"
)

// DKIMVerifier handles DKIM signature verification
type DKIMVerifier struct {
	resolver string
	timeout  time.Duration
	// LookupTXT is the DNS TXT resolver used to fetch DKIM public keys. Tests
	// inject a fake resolver here to avoid hitting the network. When nil the
	// underlying library falls back to net.LookupTXT.
	LookupTXT func(domain string) ([]string, error)
}

// NewDKIMVerifier creates a new DKIM verifier
func NewDKIMVerifier(resolver string) *DKIMVerifier {
	if resolver == "" {
		resolver = "8.8.8.8:53"
	}

	return &DKIMVerifier{
		resolver: resolver,
		timeout:  5 * time.Second,
	}
}

// Verify performs DKIM verification for the given email
func (v *DKIMVerifier) Verify(ctx context.Context, emailCtx EmailContext) DKIMResult {
	// Use go-msgauth to verify DKIM signatures on the raw message
	// We intentionally pass the raw bytes to preserve folding/canonicalization
	verifs, err := dkim.VerifyWithOptions(bytes.NewReader(emailCtx.Body), &dkim.VerifyOptions{
		LookupTXT: v.LookupTXT,
	})

	// Map per-signature results first even if there is a global error
	mapped := make([]DKIMSignatureResult, 0, len(verifs))
	anyPass := false
	for _, vr := range verifs {
		status := dkimErrorToStatus(vr.Err)
		if status == DKIMPass {
			anyPass = true
		}
		mr := DKIMSignatureResult{
			Domain:   vr.Domain,
			Selector: "", // go-msgauth doesn't expose selector publicly
			Status:   status,
		}
		if vr.Err != nil {
			mr.Error = vr.Err.Error()
		}
		mapped = append(mapped, mr)
	}

	res := DKIMResult{
		Results: mapped,
		Valid:   anyPass,
	}
	// If Verify returned an unexpected error, surface it
	if err != nil {
		res.Error = fmt.Sprintf("dkim verify error: %v", err)
	}
	return res
}

// dkimErrorToStatus classifies go-msgauth verification error into our DKIMStatus
func dkimErrorToStatus(err error) DKIMStatus {
	if err == nil {
		return DKIMPass
	}
	if dkim.IsTempFail(err) {
		return DKIMTempError
	}
	if dkim.IsPermFail(err) {
		return DKIMPermError
	}
	// Signature or body hash verification errors are reported as "failError"
	// which isn't exported; treat any other non-temp/perm error as fail.
	return DKIMFail
}
