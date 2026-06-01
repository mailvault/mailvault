package email_sending_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mailvault/mailvault/domain/email_sending"
	"github.com/mailvault/mailvault/domain/email_sending/mocks"
	"github.com/mailvault/mailvault/domain/entities"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubSender is a tiny in-test Sender — moq isn't wired for the Sender
// interface and a typed stub keeps the tests readable.
type stubSender struct {
	send       func(ctx context.Context, msg *entities.SentEmail) (string, error)
	callCount  atomic.Int32
	lastCalled *entities.SentEmail
}

func (s *stubSender) Send(ctx context.Context, msg *entities.SentEmail) (string, error) {
	s.callCount.Add(1)
	s.lastCalled = msg
	if s.send != nil {
		return s.send(ctx, msg)
	}
	return "", nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func validRequest() email_sending.SendEmailRequest {
	text := "hello"
	return email_sending.SendEmailRequest{
		DomainID:    uuid.Must(uuid.NewV4()),
		MessageID:   "msg-" + uuid.Must(uuid.NewV4()).String(),
		From:        "sender@example.com",
		ToAddresses: []string{"to@example.com"},
		Subject:     "hi",
		TextBody:    &text,
		MaxRetries:  3,
	}
}

func TestSendEmail_HappyPath_PersistsThenSendsThenUpdates(t *testing.T) {
	var created, updated *entities.SentEmail
	repo := &mocks.RepositoryMock{
		CreateSentEmailFunc: func(_ context.Context, e *entities.SentEmail) error {
			created = e
			return nil
		},
		UpdateSentEmailFunc: func(_ context.Context, e *entities.SentEmail) error {
			updated = e
			return nil
		},
	}
	sender := &stubSender{
		send: func(_ context.Context, _ *entities.SentEmail) (string, error) {
			return "smtp-msg-id-123", nil
		},
	}

	uc := email_sending.NewUseCase(repo, sender, discardLogger())
	resp, err := uc.SendEmail(context.Background(), validRequest())

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, int32(1), sender.callCount.Load())
	require.NotNil(t, created, "row should have been persisted")
	require.NotNil(t, updated, "row should have been updated post-send")
	assert.Equal(t, entities.EmailSendStatusSent, updated.Status)
	require.NotNil(t, updated.SMTPMessageID)
	assert.Equal(t, "smtp-msg-id-123", *updated.SMTPMessageID)
	// Response carries the SentEmail's stored MessageID, not the smtp message id.
	assert.Equal(t, created.ID, resp.ID)
}

func TestSendEmail_RejectsInvalidRequest(t *testing.T) {
	cases := []struct {
		name string
		mut  func(*email_sending.SendEmailRequest)
		want string
	}{
		{"missing domain", func(r *email_sending.SendEmailRequest) { r.DomainID = uuid.Nil }, "domain_id"},
		{"missing from", func(r *email_sending.SendEmailRequest) { r.From = "" }, "from address"},
		{"no recipients", func(r *email_sending.SendEmailRequest) { r.ToAddresses = nil }, "recipient"},
		{"no subject", func(r *email_sending.SendEmailRequest) { r.Subject = "" }, "subject"},
		{"no body", func(r *email_sending.SendEmailRequest) { r.TextBody = nil; r.HTMLBody = nil }, "body"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			repo := &mocks.RepositoryMock{
				CreateSentEmailFunc: func(_ context.Context, _ *entities.SentEmail) error {
					t.Fatal("repo must not be touched when request is invalid")
					return nil
				},
			}
			sender := &stubSender{}
			uc := email_sending.NewUseCase(repo, sender, discardLogger())

			req := validRequest()
			tc.mut(&req)
			_, err := uc.SendEmail(context.Background(), req)

			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
			assert.Equal(t, int32(0), sender.callCount.Load(), "sender must not be called on invalid request")
		})
	}
}

func TestSendEmail_RepoCreateFailureSkipsSender(t *testing.T) {
	repoErr := errors.New("pg dead")
	repo := &mocks.RepositoryMock{
		CreateSentEmailFunc: func(_ context.Context, _ *entities.SentEmail) error {
			return repoErr
		},
	}
	sender := &stubSender{}
	uc := email_sending.NewUseCase(repo, sender, discardLogger())

	_, err := uc.SendEmail(context.Background(), validRequest())
	require.Error(t, err)
	assert.ErrorIs(t, err, repoErr)
	assert.Equal(t, int32(0), sender.callCount.Load(), "sender must not be called when persist fails")
}

func TestSendEmail_SenderFailureMarksRowFailedAndReturnsError(t *testing.T) {
	var updated *entities.SentEmail
	repo := &mocks.RepositoryMock{
		CreateSentEmailFunc: func(_ context.Context, _ *entities.SentEmail) error { return nil },
		UpdateSentEmailFunc: func(_ context.Context, e *entities.SentEmail) error {
			updated = e
			return nil
		},
	}
	sendErr := errors.New("relay refused")
	sender := &stubSender{
		send: func(_ context.Context, _ *entities.SentEmail) (string, error) {
			return "", sendErr
		},
	}

	uc := email_sending.NewUseCase(repo, sender, discardLogger())
	_, err := uc.SendEmail(context.Background(), validRequest())

	require.Error(t, err)
	assert.ErrorIs(t, err, sendErr)
	require.NotNil(t, updated)
	assert.Equal(t, entities.EmailSendStatusFailed, updated.Status)
	require.NotNil(t, updated.ErrorMessage)
	assert.Contains(t, *updated.ErrorMessage, "relay refused")
}

func TestSendEmail_SenderFailureSwallowsUpdateError(t *testing.T) {
	// If the post-failure UPDATE also fails, the caller still gets the
	// original send error (the update failure is only logged).
	repo := &mocks.RepositoryMock{
		CreateSentEmailFunc: func(_ context.Context, _ *entities.SentEmail) error { return nil },
		UpdateSentEmailFunc: func(_ context.Context, _ *entities.SentEmail) error {
			return errors.New("pg dropped")
		},
	}
	sendErr := errors.New("relay refused")
	sender := &stubSender{
		send: func(_ context.Context, _ *entities.SentEmail) (string, error) { return "", sendErr },
	}

	uc := email_sending.NewUseCase(repo, sender, discardLogger())
	_, err := uc.SendEmail(context.Background(), validRequest())

	require.Error(t, err)
	assert.ErrorIs(t, err, sendErr, "outer error should still be the send failure, not the update failure")
}

func TestResendEmail_RetriesAndUpdates(t *testing.T) {
	failedAt := time.Now().Add(-time.Hour)
	row := &entities.SentEmail{
		ID:           uuid.Must(uuid.NewV4()),
		DomainID:     uuid.Must(uuid.NewV4()),
		FromAddress:  "x@y.test",
		ToAddresses:  []string{"to@y.test"},
		Subject:      "s",
		TextBody:     strPtr("body"),
		Status:       entities.EmailSendStatusFailed,
		RetryCount:   1,
		MaxRetries:   3,
		FailedAt:     &failedAt,
		ErrorMessage: strPtr("prev"),
	}
	var updated *entities.SentEmail
	repo := &mocks.RepositoryMock{
		GetSentEmailFunc: func(_ context.Context, _ uuid.UUID) (*entities.SentEmail, error) {
			return row, nil
		},
		UpdateSentEmailFunc: func(_ context.Context, e *entities.SentEmail) error {
			updated = e
			return nil
		},
	}
	sender := &stubSender{
		send: func(_ context.Context, _ *entities.SentEmail) (string, error) {
			return "resend-msg-id", nil
		},
	}

	uc := email_sending.NewUseCase(repo, sender, discardLogger())
	resp, err := uc.ResendEmail(context.Background(), row.ID)

	require.NoError(t, err)
	require.NotNil(t, resp)
	require.NotNil(t, updated)
	assert.Equal(t, entities.EmailSendStatusSent, updated.Status)
}

func TestResendEmail_RejectsWhenCannotRetry(t *testing.T) {
	row := &entities.SentEmail{
		ID:          uuid.Must(uuid.NewV4()),
		Status:      entities.EmailSendStatusSent, // not failed → can't retry
		RetryCount:  0,
		MaxRetries:  3,
		FromAddress: "x@y.test",
		ToAddresses: []string{"to@y.test"},
		Subject:     "s",
	}
	repo := &mocks.RepositoryMock{
		GetSentEmailFunc: func(_ context.Context, _ uuid.UUID) (*entities.SentEmail, error) {
			return row, nil
		},
	}
	sender := &stubSender{}
	uc := email_sending.NewUseCase(repo, sender, discardLogger())

	_, err := uc.ResendEmail(context.Background(), row.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be resent")
	assert.Equal(t, int32(0), sender.callCount.Load(), "sender must not be called when retry not allowed")
}

func TestResendEmail_GetFailureSurfacesError(t *testing.T) {
	getErr := errors.New("pg missing row")
	repo := &mocks.RepositoryMock{
		GetSentEmailFunc: func(_ context.Context, _ uuid.UUID) (*entities.SentEmail, error) {
			return nil, getErr
		},
	}
	sender := &stubSender{}
	uc := email_sending.NewUseCase(repo, sender, discardLogger())

	_, err := uc.ResendEmail(context.Background(), uuid.Must(uuid.NewV4()))
	require.Error(t, err)
	assert.ErrorIs(t, err, getErr)
	assert.Equal(t, int32(0), sender.callCount.Load())
}

func TestProcessRetryQueue_IteratesAllReadyEmails(t *testing.T) {
	// ProcessRetryQueue → ResendEmail mutates each row (MarkAsSent flips status
	// to Sent). So each retry must operate on its own copy; the per-ID map below
	// hands ResendEmail a fresh struct on each GetSentEmail call.
	rows := []*entities.SentEmail{newFailedRow(), newFailedRow()}
	byID := map[uuid.UUID]*entities.SentEmail{rows[0].ID: rows[0], rows[1].ID: rows[1]}
	repo := &mocks.RepositoryMock{
		GetSentEmailsForRetryFunc: func(_ context.Context, _ int) ([]*entities.SentEmail, error) {
			return rows, nil
		},
		GetSentEmailFunc: func(_ context.Context, id uuid.UUID) (*entities.SentEmail, error) {
			return byID[id], nil
		},
		UpdateSentEmailFunc: func(_ context.Context, _ *entities.SentEmail) error { return nil },
	}
	sender := &stubSender{
		send: func(_ context.Context, _ *entities.SentEmail) (string, error) { return "ok", nil },
	}

	uc := email_sending.NewUseCase(repo, sender, discardLogger())
	require.NoError(t, uc.ProcessRetryQueue(context.Background(), 10))
	assert.Equal(t, int32(2), sender.callCount.Load(), "every ready email should hit the sender")
}

func newFailedRow() *entities.SentEmail {
	return &entities.SentEmail{
		ID:          uuid.Must(uuid.NewV4()),
		DomainID:    uuid.Must(uuid.NewV4()),
		FromAddress: "x@y.test",
		ToAddresses: []string{"to@y.test"},
		Subject:     "s",
		TextBody:    strPtr("body"),
		Status:      entities.EmailSendStatusFailed,
		RetryCount:  0,
		MaxRetries:  3,
	}
}

func TestProcessRetryQueue_LogsAndContinuesPastIndividualFailures(t *testing.T) {
	// Even if ResendEmail fails for one row, the loop must keep going.
	row1 := &entities.SentEmail{
		ID:          uuid.Must(uuid.NewV4()),
		DomainID:    uuid.Must(uuid.NewV4()),
		FromAddress: "x@y.test",
		ToAddresses: []string{"to@y.test"},
		Subject:     "s",
		TextBody:    strPtr("body"),
		Status:      entities.EmailSendStatusFailed,
		RetryCount:  0,
		MaxRetries:  3,
	}
	row2 := *row1
	row2.ID = uuid.Must(uuid.NewV4())

	rows := []*entities.SentEmail{row1, &row2}

	repo := &mocks.RepositoryMock{
		GetSentEmailsForRetryFunc: func(_ context.Context, _ int) ([]*entities.SentEmail, error) {
			return rows, nil
		},
		GetSentEmailFunc: func(_ context.Context, id uuid.UUID) (*entities.SentEmail, error) {
			for _, r := range rows {
				if r.ID == id {
					return r, nil
				}
			}
			return nil, errors.New("not found")
		},
		UpdateSentEmailFunc: func(_ context.Context, _ *entities.SentEmail) error { return nil },
	}
	sender := &stubSender{
		send: func(_ context.Context, msg *entities.SentEmail) (string, error) {
			if msg.ID == row1.ID {
				return "", errors.New("first send failed")
			}
			return "ok", nil
		},
	}

	uc := email_sending.NewUseCase(repo, sender, discardLogger())
	require.NoError(t, uc.ProcessRetryQueue(context.Background(), 10))
	assert.Equal(t, int32(2), sender.callCount.Load(), "second row must still attempt despite first failure")
}

// --- helpers ---

func strPtr(s string) *string { return &s }
