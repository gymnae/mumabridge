package media

import (
	"encoding/binary"
	"errors"
)

const (
	SampleRate      = 48000
	Channels        = 1
	FrameDuration   = 20
	SamplesPerFrame = SampleRate * FrameDuration / 1000
)

// Frame is one 20 ms, 48 kHz mono signed-linear PCM frame.
type Frame []int16

func FromLittleEndian(data []byte) (Frame, error) {
	if len(data)%2 != 0 {
		return nil, errors.New("PCM byte length must be even")
	}
	frame := make(Frame, len(data)/2)
	for i := range frame {
		frame[i] = int16(binary.LittleEndian.Uint16(data[i*2 : i*2+2]))
	}
	return frame, nil
}

func (f Frame) LittleEndian() []byte {
	data := make([]byte, len(f)*2)
	for i, sample := range f {
		binary.LittleEndian.PutUint16(data[i*2:i*2+2], uint16(sample))
	}
	return data
}

// Queue is a bounded non-blocking media queue. When a producer outruns a
// consumer, the oldest audio is discarded to keep latency bounded.
type Queue struct {
	frames chan Frame
}

func NewQueue(capacity int) *Queue { return &Queue{frames: make(chan Frame, capacity)} }

func (q *Queue) Push(frame Frame) {
	copyOfFrame := append(Frame(nil), frame...)
	select {
	case q.frames <- copyOfFrame:
		return
	default:
	}
	select {
	case <-q.frames:
	default:
	}
	select {
	case q.frames <- copyOfFrame:
	default:
	}
}

func (q *Queue) Frames() <-chan Frame { return q.frames }
