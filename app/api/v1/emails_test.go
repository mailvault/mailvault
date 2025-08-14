package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mailvault/app/api/v1/mocks"
	"mailvault/domain/email"
	"mailvault/domain/entities"

	"github.com/go-chi/chi/v5"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/assert"
)

func TestEmailsHandlers_CreateEmailAddress(t *testing.T) {
	t.Parallel()

	domainID := uuid.Must(uuid.NewV4())
	now := time.Now().UTC()

	mock := &mocks.EmailUseCaseMock{
		CreateEmailAddressFromInputFunc: func(ctx context.Context, in email.CreateEmailAddressInput) (*entities.EmailAddress, error) {
			assert.Equal(t, domainID, in.DomainID)
			assert.Equal(t, "info", in.LocalPart)
			assert.False(t, in.IsCatchAll)
			return &entities.EmailAddress{
				ID:               uuid.Must(uuid.NewV4()),
				DomainID:         domainID,
				LocalPart:        in.LocalPart,
				IsCatchAll:       in.IsCatchAll,
				ForwardAddresses: []string{"a@example.com"},
				CreatedAt:        now,
				UpdatedAt:        now,
			}, nil
		},
	}
	h := NewEmailsHandlers(mock)

	body, _ := json.Marshal(CreateEmailRequest{LocalPart: "info", IsCatchAll: false, ForwardAddresses: []string{"a@example.com"}})
	req := httptest.NewRequest(http.MethodPost, "/domains/"+domainID.String()+"/emails", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("domainId", domainID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.CreateEmailAddress(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
}

func TestEmailsHandlers_GetEmailAddresses(t *testing.T) {
	t.Parallel()

	domainID := uuid.Must(uuid.NewV4())
	now := time.Now().UTC()

	mock := &mocks.EmailUseCaseMock{
		GetEmailAddressesByDomainIDFunc: func(ctx context.Context, dID uuid.UUID) ([]*entities.EmailAddress, error) {
			assert.Equal(t, domainID, dID)
			return []*entities.EmailAddress{
				{ID: uuid.Must(uuid.NewV4()), DomainID: domainID, LocalPart: "info", CreatedAt: now, UpdatedAt: now},
				{ID: uuid.Must(uuid.NewV4()), DomainID: domainID, LocalPart: "sales", CreatedAt: now, UpdatedAt: now},
			}, nil
		},
	}
	h := NewEmailsHandlers(mock)

	req := httptest.NewRequest(http.MethodGet, "/domains/"+domainID.String()+"/emails", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("domainId", domainID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.GetEmailAddresses(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	var resp []*EmailAddressResult
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Len(t, resp, 2)
}

func TestEmailsHandlers_GetEmailAddress(t *testing.T) {
	t.Parallel()

	emailID := uuid.Must(uuid.NewV4())
	domainID := uuid.Must(uuid.NewV4())
	now := time.Now().UTC()

	mock := &mocks.EmailUseCaseMock{
		GetEmailAddressByIDFunc: func(ctx context.Context, id uuid.UUID) (*entities.EmailAddress, error) {
			assert.Equal(t, emailID, id)
			return &entities.EmailAddress{ID: emailID, DomainID: domainID, LocalPart: "info", CreatedAt: now, UpdatedAt: now}, nil
		},
	}
	h := NewEmailsHandlers(mock)

	req := httptest.NewRequest(http.MethodGet, "/emails/"+emailID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("emailId", emailID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.GetEmailAddress(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmailsHandlers_UpdateEmailAddress(t *testing.T) {
	t.Parallel()

	emailID := uuid.Must(uuid.NewV4())
	now := time.Now().UTC()
	newCatchAll := true

	mock := &mocks.EmailUseCaseMock{
		UpdateEmailAddressFunc: func(ctx context.Context, id uuid.UUID, in email.UpdateEmailAddressInput) (*entities.EmailAddress, error) {
			assert.Equal(t, emailID, id)
			assert.NotNil(t, in.IsCatchAll)
			return &entities.EmailAddress{ID: id, DomainID: uuid.Must(uuid.NewV4()), LocalPart: "info", IsCatchAll: true, CreatedAt: now, UpdatedAt: now}, nil
		},
	}
	h := NewEmailsHandlers(mock)

	body, _ := json.Marshal(UpdateEmailRequest{IsCatchAll: &newCatchAll})
	req := httptest.NewRequest(http.MethodPut, "/emails/"+emailID.String(), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("emailId", emailID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.UpdateEmailAddress(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestEmailsHandlers_DeleteEmailAddress(t *testing.T) {
	t.Parallel()

	emailID := uuid.Must(uuid.NewV4())
	mock := &mocks.EmailUseCaseMock{
		DeleteEmailAddressFunc: func(ctx context.Context, id uuid.UUID) error {
			assert.Equal(t, emailID, id)
			return nil
		},
	}
	h := NewEmailsHandlers(mock)

	req := httptest.NewRequest(http.MethodDelete, "/emails/"+emailID.String(), nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("emailId", emailID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.DeleteEmailAddress(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestEmailsHandlers_GetReceivedEmails(t *testing.T) {
	t.Parallel()

	emailID := uuid.Must(uuid.NewV4())
	now := time.Now().UTC()

	mock := &mocks.EmailUseCaseMock{
		GetReceivedEmailsFunc: func(ctx context.Context, id uuid.UUID, limit, offset int) ([]*entities.ReceivedEmail, error) {
			assert.Equal(t, emailID, id)
			assert.Equal(t, 50, limit) // default
			assert.Equal(t, 0, offset) // default
			subj := "Hello"
			return []*entities.ReceivedEmail{
				{ID: uuid.Must(uuid.NewV4()), FromAddress: "a@b.com", Subject: &subj, EncryptedBody: "enc", ReceivedAt: now},
			}, nil
		},
	}
	h := NewEmailsHandlers(mock)

	req := httptest.NewRequest(http.MethodGet, "/emails/"+emailID.String()+"/received", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("emailId", emailID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
	w := httptest.NewRecorder()

	h.GetReceivedEmails(w, req)
	assert.Equal(t, http.StatusOK, w.Code)

	// Assert JSON structure
	var out struct {
		Data       []*ReceivedEmailResult      `json:"data"`
		Pagination struct{ Limit, Offset int } `json:"pagination"`
	}
	assert.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	assert.Len(t, out.Data, 1)
	assert.Equal(t, 50, out.Pagination.Limit)
}
