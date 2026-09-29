package matrix

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strings"
)

type Event struct {
	Type     string          `json:"type"`
	RoomID   string          `json:"room_id"`
	Sender   string          `json:"sender"`
	StateKey *string         `json:"state_key,omitempty"`
	Content  json.RawMessage `json:"content"`
}

type Transaction struct {
	Events []Event `json:"events"`
}

type AppService struct {
	hsToken string
	onEvent func(Event)
}

func NewAppService(hsToken string, onEvent func(Event)) *AppService {
	return &AppService{hsToken: hsToken, onEvent: onEvent}
}

func (a *AppService) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("PUT /_matrix/app/v1/transactions/{txnID}", a.transaction)
	mux.HandleFunc("PUT /transactions/{txnID}", a.transaction)
	mux.HandleFunc("GET /_matrix/app/v1/users/{userID}", a.exists)
	mux.HandleFunc("GET /_matrix/app/v1/rooms/{roomAlias}", a.exists)
	return mux
}

func (a *AppService) transaction(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	defer r.Body.Close()
	var transaction Transaction
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<20)).Decode(&transaction); err != nil {
		http.Error(w, "invalid transaction", http.StatusBadRequest)
		return
	}
	for _, event := range transaction.Events {
		if a.onEvent != nil {
			a.onEvent(event)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (a *AppService) exists(w http.ResponseWriter, r *http.Request) {
	if !a.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{})
}

func (a *AppService) authorized(r *http.Request) bool {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if token == "" {
		token = r.URL.Query().Get("access_token")
	}
	return len(token) == len(a.hsToken) && subtle.ConstantTimeCompare([]byte(token), []byte(a.hsToken)) == 1
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
