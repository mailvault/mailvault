package verification

import (
	"net"
	"time"
)

// VerificationResult contains all spam verification results
type VerificationResult struct {
	SPF        SPFResult        `json:"spf"`
	DKIM       DKIMResult       `json:"dkim"`
	DMARC      DMARCResult      `json:"dmarc"`
	Reputation ReputationResult `json:"reputation"`
	Content    ContentResult    `json:"content"`
	Action     Action           `json:"action"`
	Timestamp  time.Time        `json:"timestamp"`
}

// Action defines what to do with the email
type Action int

const (
	ActionAccept Action = iota
	ActionQuarantine
	ActionReject
	ActionTempFail
)

func (a Action) String() string {
	switch a {
	case ActionAccept:
		return "accept"
	case ActionQuarantine:
		return "quarantine"
	case ActionReject:
		return "reject"
	case ActionTempFail:
		return "tempfail"
	default:
		return "unknown"
	}
}

// SPFResult contains SPF verification results
type SPFResult struct {
	Result    SPFStatus `json:"result"`
	Mechanism string    `json:"mechanism,omitempty"`
	Error     string    `json:"error,omitempty"`
}

type SPFStatus int

const (
	SPFNone SPFStatus = iota
	SPFNeutral
	SPFPass
	SPFFail
	SPFSoftFail
	SPFTempError
	SPFPermError
)

func (s SPFStatus) String() string {
	switch s {
	case SPFNone:
		return "none"
	case SPFNeutral:
		return "neutral"
	case SPFPass:
		return "pass"
	case SPFFail:
		return "fail"
	case SPFSoftFail:
		return "softfail"
	case SPFTempError:
		return "temperror"
	case SPFPermError:
		return "permerror"
	default:
		return "unknown"
	}
}

// DKIMResult contains DKIM verification results
type DKIMResult struct {
	Results []DKIMSignatureResult `json:"results"`
	Valid   bool                  `json:"valid"`
	Error   string                `json:"error,omitempty"`
}

type DKIMSignatureResult struct {
	Domain    string     `json:"domain"`
	Selector  string     `json:"selector"`
	Status    DKIMStatus `json:"status"`
	Algorithm string     `json:"algorithm,omitempty"`
	Error     string     `json:"error,omitempty"`
}

type DKIMStatus int

const (
	DKIMNone DKIMStatus = iota
	DKIMPass
	DKIMFail
	DKIMPolicy
	DKIMNeutral
	DKIMTempError
	DKIMPermError
)

func (d DKIMStatus) String() string {
	switch d {
	case DKIMNone:
		return "none"
	case DKIMPass:
		return "pass"
	case DKIMFail:
		return "fail"
	case DKIMPolicy:
		return "policy"
	case DKIMNeutral:
		return "neutral"
	case DKIMTempError:
		return "temperror"
	case DKIMPermError:
		return "permerror"
	default:
		return "unknown"
	}
}

// DMARCResult contains DMARC verification results
type DMARCResult struct {
	Result     DMARCStatus `json:"result"`
	Policy     string      `json:"policy,omitempty"`
	Percentage int         `json:"percentage,omitempty"`
	SPFAlign   bool        `json:"spf_aligned"`
	DKIMAlign  bool        `json:"dkim_aligned"`
	Error      string      `json:"error,omitempty"`
}

type DMARCStatus int

const (
	DMARCNone DMARCStatus = iota
	DMARCPass
	DMARCFail
	DMARCTempError
	DMARCPermError
)

func (d DMARCStatus) String() string {
	switch d {
	case DMARCNone:
		return "none"
	case DMARCPass:
		return "pass"
	case DMARCFail:
		return "fail"
	case DMARCTempError:
		return "temperror"
	case DMARCPermError:
		return "permerror"
	default:
		return "unknown"
	}
}

// ReputationResult contains reputation check results
type ReputationResult struct {
	IPReputation     IPReputationStatus     `json:"ip_reputation"`
	DomainReputation DomainReputationStatus `json:"domain_reputation"`
	Blacklisted      []string               `json:"blacklisted,omitempty"`
	Score            float64                `json:"score"`
	Error            string                 `json:"error,omitempty"`
}

type IPReputationStatus int

const (
	IPReputationUnknown IPReputationStatus = iota
	IPReputationGood
	IPReputationSuspicious
	IPReputationBad
)

func (i IPReputationStatus) String() string {
	switch i {
	case IPReputationUnknown:
		return "unknown"
	case IPReputationGood:
		return "good"
	case IPReputationSuspicious:
		return "suspicious"
	case IPReputationBad:
		return "bad"
	default:
		return "unknown"
	}
}

type DomainReputationStatus int

const (
	DomainReputationUnknown DomainReputationStatus = iota
	DomainReputationGood
	DomainReputationSuspicious
	DomainReputationBad
)

func (d DomainReputationStatus) String() string {
	switch d {
	case DomainReputationUnknown:
		return "unknown"
	case DomainReputationGood:
		return "good"
	case DomainReputationSuspicious:
		return "suspicious"
	case DomainReputationBad:
		return "bad"
	default:
		return "unknown"
	}
}

// ContentResult contains content analysis results
type ContentResult struct {
	SpamScore      float64  `json:"spam_score"`
	SpamIndicators []string `json:"spam_indicators,omitempty"`
	Classification string   `json:"classification"`
	Error          string   `json:"error,omitempty"`
}

// EmailContext contains information needed for verification
type EmailContext struct {
	From       string    `json:"from"`
	To         []string  `json:"to"`
	Subject    string    `json:"subject"`
	Body       []byte    `json:"body"`
	Headers    []Header  `json:"headers"`
	SenderIP   net.IP    `json:"sender_ip"`
	ReceivedAt time.Time `json:"received_at"`
}

type Header struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// VerificationConfig contains configuration for verification
type VerificationConfig struct {
	EnableSPF        bool    `json:"enable_spf"`
	EnableDKIM       bool    `json:"enable_dkim"`
	EnableDMARC      bool    `json:"enable_dmarc"`
	EnableReputation bool    `json:"enable_reputation"`
	EnableContent    bool    `json:"enable_content"`
	SpamThreshold    float64 `json:"spam_threshold"`
	RejectOnFail     bool    `json:"reject_on_fail"`
	QuarantineMode   bool    `json:"quarantine_mode"`
}

// DefaultConfig returns default verification configuration
func DefaultConfig() VerificationConfig {
	return VerificationConfig{
		EnableSPF:        true,
		EnableDKIM:       true,
		EnableDMARC:      true,
		EnableReputation: true,
		EnableContent:    true,
		SpamThreshold:    0.7,
		RejectOnFail:     false,
		QuarantineMode:   true,
	}
}