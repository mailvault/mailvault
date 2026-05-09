package verification

import (
	"context"
	"time"

	"github.com/miekg/dns"
)

// DNSExchanger is the minimal interface all verifiers use to perform DNS
// queries. The real implementation is a *dns.Client; tests inject a mock that
// returns canned responses without hitting the network.
type DNSExchanger interface {
	ExchangeContext(ctx context.Context, msg *dns.Msg, addr string) (*dns.Msg, time.Duration, error)
}
