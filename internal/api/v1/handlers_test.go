package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestApiHandlers_Health(t *testing.T) {
	t.Parallel()
	h := &ApiHandlers{}
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, w.Code)
	}
}
