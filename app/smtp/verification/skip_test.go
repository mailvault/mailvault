package verification

import "testing"

// skipNeedsDNSInjection marks tests that exercise the SPF/DKIM/DMARC/reputation/
// content/verifier code paths against mocks-by-embedding. The current mock
// pattern wraps the concrete verifier and tries to override an unexported
// method, which Go's embedding does not honour at runtime — so the wrapper's
// override is never called and the real DNS path runs (or fails to mock).
//
// Re-enable these tests after refactoring the verifiers to accept a
// DNSExchanger interface at construction time, then injecting a mock directly
// instead of relying on method shadowing.
func skipNeedsDNSInjection(t *testing.T) {
	t.Helper()
	t.Skip("verifier needs DNSExchanger injection; mock-by-embedding does not dispatch correctly")
}
