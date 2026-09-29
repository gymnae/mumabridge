package bridge

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	matrixbridge "github.com/gymnae/mumabridge/internal/matrix"
)

// MatrixSource converts application-service membership events into an
// authoritative room snapshot. Handle is safe for the HTTP handler to call.
type MatrixSource struct {
	roomID      string
	ghostPrefix string
	events      chan matrixbridge.Event
	mu          sync.RWMutex
	running     bool
}

func NewMatrixSource(roomID, ghostPrefix string) *MatrixSource {
	return &MatrixSource{roomID: roomID, ghostPrefix: ghostPrefix, events: make(chan matrixbridge.Event, 128)}
}

func (s *MatrixSource) Handle(event matrixbridge.Event) {
	select {
	case s.events <- event:
	default:
		// Homeservers retry transactions; dropping under sustained overload is
		// preferable to blocking their transaction endpoint indefinitely.
	}
}

func (s *MatrixSource) Run(ctx context.Context, updates chan<- Snapshot) error {
	members := make(map[string]Participant)
	var generation uint64 = 1
	updates <- Snapshot{Generation: generation, Side: SideMatrix, Connected: true}
	for {
		select {
		case <-ctx.Done():
			return nil
		case event := <-s.events:
			if event.Type != "m.room.member" || event.RoomID != s.roomID || event.StateKey == nil {
				continue
			}
			userID := *event.StateKey
			var content struct {
				Membership  string `json:"membership"`
				DisplayName string `json:"displayname"`
			}
			if json.Unmarshal(event.Content, &content) != nil {
				continue
			}
			if content.Membership == "join" {
				name := strings.TrimSpace(content.DisplayName)
				if name == "" {
					name = userID
				}
				members[userID] = Participant{ID: userID, DisplayName: name, Side: SideMatrix, BridgeOwned: matrixbridge.IsGhostUser(userID, s.ghostPrefix)}
			} else {
				delete(members, userID)
			}
			snapshot := make([]Participant, 0, len(members))
			for _, member := range members {
				snapshot = append(snapshot, member)
			}
			updates <- Snapshot{Generation: generation, Side: SideMatrix, Members: snapshot, Connected: true}
		}
	}
}
