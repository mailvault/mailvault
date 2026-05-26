package verification

import (
	"github.com/mailvault/mailvault/internal/utils"

	"github.com/emersion/go-msgauth/authres"
)

// BuildAuthResultsHeader builds a full Authentication-Results header line
// based on the computed verification results.
// Example output: "Authentication-Results: mailvault; spf=pass smtp.mailfrom=example.com; dkim=pass header.d=example.com; dmarc=pass header.from=example.com"
func BuildAuthResultsHeader(identity string, emailCtx EmailContext, vr VerificationResult) string {
	if identity == "" {
		identity = "mailvault"
	}

	results := make([]authres.Result, 0, 4)

	// SPF result
	spfVal := mapSPFStatusToAuthRes(vr.SPF.Result)
	if spfVal != authres.ResultNone || vr.SPF.Mechanism != "" {
		results = append(results, &authres.SPFResult{
			Value: spfVal,
			From:  emailCtx.From,
			Helo:  "",
		})
	}

	// DKIM results (one per signature)
	for _, s := range vr.DKIM.Results {
		results = append(results, &authres.DKIMResult{
			Value:  mapDKIMStatusToAuthRes(s.Status),
			Domain: s.Domain,
		})
	}

	// DMARC result
	dmarcVal := mapDMARCStatusToAuthRes(vr.DMARC.Result)
	if dmarcVal != authres.ResultNone {
		results = append(results, &authres.DMARCResult{
			Value: dmarcVal,
			From:  extractDomainFromAddress(emailCtx.From),
		})
	}

	// Format header value
	value := authres.Format(identity, results)
	return "Authentication-Results: " + value
}

func mapSPFStatusToAuthRes(s SPFStatus) authres.ResultValue {
	switch s {
	case SPFPass:
		return authres.ResultPass
	case SPFFail:
		return authres.ResultFail
	case SPFSoftFail:
		return authres.ResultSoftFail
	case SPFNeutral:
		return authres.ResultNeutral
	case SPFTempError:
		return authres.ResultTempError
	case SPFPermError:
		return authres.ResultPermError
	case SPFNone:
		fallthrough
	default:
		return authres.ResultNone
	}
}

func mapDKIMStatusToAuthRes(s DKIMStatus) authres.ResultValue {
	switch s {
	case DKIMPass:
		return authres.ResultPass
	case DKIMFail:
		return authres.ResultFail
	case DKIMPolicy:
		return authres.ResultPolicy
	case DKIMNeutral:
		return authres.ResultNeutral
	case DKIMTempError:
		return authres.ResultTempError
	case DKIMPermError:
		return authres.ResultPermError
	case DKIMNone:
		fallthrough
	default:
		return authres.ResultNone
	}
}

func mapDMARCStatusToAuthRes(s DMARCStatus) authres.ResultValue {
	switch s {
	case DMARCPass:
		return authres.ResultPass
	case DMARCFail:
		return authres.ResultFail
	case DMARCTempError:
		return authres.ResultTempError
	case DMARCPermError:
		return authres.ResultPermError
	case DMARCNone:
		fallthrough
	default:
		return authres.ResultNone
	}
}

func extractDomainFromAddress(addr string) string {
	domain, err := utils.ExtractDomain(addr)
	if err != nil {
		return ""
	}
	return domain
}
