package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"

	. "github.com/usace-cloud-compute/cloudcompute"
)

/*
This code will be removed but is being left in for a short period of time.
DO NOT USE THIS STRUCT
*/

type StreamingBatchEventGenerator struct {
	event          Event
	scanner        *bufio.Scanner
	index          int
	mu             sync.Mutex
	perEventLooper *PerEventLooper
	hasNext        bool
	eventId        string
}

func NewStreamingEventGeneratorForReader2(event Event, perEventLoopData []map[string]string, reader io.Reader, delimiter string) (*StreamingBatchEventGenerator, error) {
	scanner := bufio.NewScanner(reader)
	scanner.Split(splitAt(delimiter))
	manifestCount := len(event.Manifests)
	for i := 0; i < manifestCount; i++ {
		err := event.Manifests[i].WritePayload()
		if err != nil {
			return nil, fmt.Errorf("failed to write payload for manifest %s: %s", event.Manifests[i].ManifestID, err)
		}
	}

	perEventLooper := NewPerEventLooper(perEventLoopData)

	seg := &StreamingBatchEventGenerator{
		event:          event,
		scanner:        scanner,
		perEventLooper: perEventLooper,
	}

	seg.hasNext = seg.scanner.Scan()
	seg.eventId = seg.scanner.Text()

	return seg, nil
}

func NewStreamingEventGenerator2(event Event, perEventLoopData []map[string]string, scanner *bufio.Scanner) (*StreamingBatchEventGenerator, error) {
	manifestCount := len(event.Manifests)
	for i := 0; i < manifestCount; i++ {
		err := event.Manifests[i].WritePayload()
		if err != nil {
			return nil, fmt.Errorf("failed to write payload for manifest %s: %s", event.Manifests[i].ManifestID, err)
		}
	}

	perEventLooper := NewPerEventLooper(perEventLoopData)

	seg := &StreamingBatchEventGenerator{
		event:          event,
		scanner:        scanner,
		perEventLooper: perEventLooper,
	}

	seg.hasNext = seg.scanner.Scan()
	seg.eventId = seg.scanner.Text()

	return seg, nil
}

func (seg *StreamingBatchEventGenerator) scanNext() {
	delimiter := ","
	batchsize := 10
	var eventIdentifier strings.Builder
	for i := 0; i < batchsize; i++ {
		seg.hasNext = seg.scanner.Scan()
		nextId := seg.scanner.Text()
		if i > 0 {
			eventIdentifier.WriteString(delimiter)
		}
		eventIdentifier.WriteString(nextId)
		seg.index++
		if !seg.hasNext {
			break
		}
	}
	seg.event.EventIdentifier = eventIdentifier.String()
}

func (seg *StreamingBatchEventGenerator) NextEvent() (Event, bool, error) {
	seg.mu.Lock()
	defer seg.mu.Unlock()

	event := seg.event
	if seg.eventId == "" {
		seg.scanNext()
		return event, seg.hasNext, fmt.Errorf("empty event identifier at index: %d", seg.index)
	} else {
		var additionalEnvVars map[string]string
		incrementEvent := true

		if seg.perEventLooper != nil {
			additionalEnvVars, incrementEvent = seg.perEventLooper.Next()
			event.AdditionalEventEnvVars = MapToKeyValuePairs(additionalEnvVars)
		}
		event.EventIdentifier = seg.eventId

		//read next if we have exhausted the perEventLoop arrray:
		if incrementEvent {
			seg.scanNext()
		}
		return event, seg.hasNext, nil
	}
}

func splitAt(substring string) func(data []byte, atEOF bool) (advance int, token []byte, err error) {
	searchBytes := []byte(substring)
	searchLen := len(searchBytes)
	return func(data []byte, atEOF bool) (advance int, token []byte, err error) {
		dataLen := len(data)

		// Return nothing if at end of file and no data passed
		if atEOF && dataLen == 0 {
			return 0, nil, nil
		}

		// Find next separator and return token
		if i := bytes.Index(data, searchBytes); i >= 0 {
			return i + searchLen, data[0:i], nil
		}

		// If we're at EOF, we have a final, non-terminated line. Return it.
		if atEOF {
			return dataLen, data, nil
		}

		// Request more data.
		return 0, nil, nil
	}
}
