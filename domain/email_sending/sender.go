package email_sending

import (
	"context"

	"github.com/mailvault/mailvault/domain/entities"
)

// Sender delivers one outbound email and returns the message id that downstream
// systems should use to refer to it. Implementations talk to whatever transport
// the deployment is configured for — by default, the OSS build uses a local
// SMTP relay. Implementations must be safe for concurrent use.
//
// `msg` is the persisted sent-email row built by the use case; the sender is
// free to read From/To/CC/BCC/Subject/TextBody/HTMLBody/MessageID off it and
// must not mutate the row.
type Sender interface {
	Send(ctx context.Context, msg *entities.SentEmail) (messageID string, err error)
}
