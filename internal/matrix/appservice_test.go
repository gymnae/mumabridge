package matrix

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAppServiceTransactionAuthenticatesAndDispatches(t *testing.T) {
	events := make(chan Event, 1)
	handler := NewAppService("secret", func(event Event) { events <- event }).Handler()
	req := httptest.NewRequest(http.MethodPut, "/_matrix/app/v1/transactions/1", strings.NewReader(`{"events":[{"type":"m.room.member","room_id":"!room:example","sender":"@arnold:example","content":{}}]}`))
	req.Header.Set("Authorization", "Bearer secret")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
	}
	select {
	case event := <-events:
		if event.Sender != "@arnold:example" {
			t.Fatalf("unexpected event sender %q", event.Sender)
		}
	default:
		t.Fatal("event was not dispatched")
	}
}

func TestAppServiceRejectsBadToken(t *testing.T) {
	handler := NewAppService("secret", nil).Handler()
	req := httptest.NewRequest(http.MethodPut, "/_matrix/app/v1/transactions/1", strings.NewReader(`{"events":[]}`))
	req.Header.Set("Authorization", "Bearer wrong")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unexpected status %d", response.Code)
	}
}
