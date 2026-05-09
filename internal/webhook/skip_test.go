package webhook

import "testing"

// skipDeprecatedNotificationTest marks tests that exercise the legacy
// inline-on-Domain webhook config path. The production flow now resolves
// configurations via ConfigLoader against the webhook_configs table.
//
// Functional coverage of the new path lives in notification_test.go. These
// older end-to-end suites use the same fixtures as the deprecated unit tests
// and would need a parallel rewrite — track that separately.
func skipDeprecatedNotificationTest(t *testing.T) {
	t.Helper()
	t.Skip("legacy notification path: rewrite against webhook_config.ConfigLoader (covered by notification_test.go)")
}
