package livekit

import (
	"context"
	"errors"
	"sync"

	mediasdk "github.com/livekit/media-sdk"
	"github.com/livekit/protocol/logger"
	lksdk "github.com/livekit/server-sdk-go/v2"
	lkmedia "github.com/livekit/server-sdk-go/v2/pkg/media"
	"github.com/pion/webrtc/v4"

	bridgemedia "github.com/gymnae/mumabridge/internal/media"
)

type SDKDialer struct{}

func NewSDKDialer() *SDKDialer { return &SDKDialer{} }

func (d *SDKDialer) Join(ctx context.Context, url, token string) (Session, error) {
	session := &sdkSession{
		remoteAudio: make(chan RemoteAudio, 32),
		closed:      make(chan struct{}),
		tracks:      make(map[string]*lkmedia.PCMRemoteTrack),
	}
	callback := &lksdk.RoomCallback{ParticipantCallback: lksdk.ParticipantCallback{
		OnTrackSubscribed:   session.onTrackSubscribed,
		OnTrackUnsubscribed: session.onTrackUnsubscribed,
	}}
	room, err := lksdk.ConnectToRoomWithToken(url, token, callback)
	if err != nil {
		return nil, err
	}
	session.room = room
	go func() {
		select {
		case <-ctx.Done():
			_ = session.Close(context.Background())
		case <-session.closed:
		}
	}()
	return session, nil
}

type sdkSession struct {
	mu          sync.Mutex
	room        *lksdk.Room
	remoteAudio chan RemoteAudio
	tracks      map[string]*lkmedia.PCMRemoteTrack
	closed      chan struct{}
	closeOnce   sync.Once
}

func (s *sdkSession) PublishPCM(ctx context.Context, frames <-chan bridgemedia.Frame) error {
	track, err := lkmedia.NewPCMLocalTrack(bridgemedia.SampleRate, bridgemedia.Channels, logger.GetLogger())
	if err != nil {
		return err
	}
	defer track.Close()
	if _, err := s.room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{Name: "mumabridge-audio"}); err != nil {
		return err
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.closed:
			return nil
		case frame, ok := <-frames:
			if !ok {
				return nil
			}
			if err := track.WriteSample(mediasdk.PCM16Sample(frame)); err != nil {
				return err
			}
		}
	}
}

func (s *sdkSession) RemoteAudio() <-chan RemoteAudio { return s.remoteAudio }

func (s *sdkSession) Close(context.Context) error {
	s.closeOnce.Do(func() {
		close(s.closed)
		s.mu.Lock()
		for id, track := range s.tracks {
			track.Close()
			delete(s.tracks, id)
		}
		s.mu.Unlock()
		if s.room != nil {
			s.room.Disconnect()
		}
	})
	return nil
}

func (s *sdkSession) onTrackSubscribed(track *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, participant *lksdk.RemoteParticipant) {
	if track.Codec().MimeType != webrtc.MimeTypeOpus || IsBridgeOwned(participant.Metadata()) {
		return
	}
	queue := bridgemedia.NewQueue(25)
	writer := &pcmWriter{queue: queue, closed: make(chan struct{})}
	pcmTrack, err := lkmedia.NewPCMRemoteTrack(track, writer)
	if err != nil {
		return
	}
	s.mu.Lock()
	s.tracks[publication.SID()] = pcmTrack
	s.mu.Unlock()
	remote := RemoteAudio{Participant: Participant{Identity: participant.Identity(), Name: participant.Name(), Metadata: participant.Metadata()}, Frames: queue.Frames()}
	select {
	case s.remoteAudio <- remote:
	case <-s.closed:
		pcmTrack.Close()
	}
}

func (s *sdkSession) onTrackUnsubscribed(_ *webrtc.TrackRemote, publication *lksdk.RemoteTrackPublication, _ *lksdk.RemoteParticipant) {
	s.mu.Lock()
	track := s.tracks[publication.SID()]
	delete(s.tracks, publication.SID())
	s.mu.Unlock()
	if track != nil {
		track.Close()
	}
}

type pcmWriter struct {
	queue  *bridgemedia.Queue
	closed chan struct{}
	once   sync.Once
}

func (w *pcmWriter) WriteSample(sample mediasdk.PCM16Sample) error {
	select {
	case <-w.closed:
		return errors.New("PCM writer closed")
	default:
		w.queue.Push(bridgemedia.Frame(sample))
		return nil
	}
}

func (w *pcmWriter) Close() error {
	w.once.Do(func() { close(w.closed) })
	return nil
}
