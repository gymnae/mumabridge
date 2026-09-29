package media

import (
	"reflect"
	"testing"
)

func TestPCMEndianRoundTrip(t *testing.T) {
	input := Frame{-32768, -1, 0, 1, 32767}
	output, err := FromLittleEndian(input.LittleEndian())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(input, output) {
		t.Fatalf("round trip mismatch: %v != %v", input, output)
	}
}

func TestQueueDropsOldestFrame(t *testing.T) {
	queue := NewQueue(2)
	queue.Push(Frame{1})
	queue.Push(Frame{2})
	queue.Push(Frame{3})
	first := <-queue.Frames()
	second := <-queue.Frames()
	if first[0] != 2 || second[0] != 3 {
		t.Fatalf("expected newest frames, got %v and %v", first, second)
	}
}
