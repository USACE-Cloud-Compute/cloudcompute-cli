package utils

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/google/uuid"
	. "github.com/usace-cloud-compute/cloudcompute"
)

type ArrayEventGenerator2 struct {
	event                  Event
	end                    int64
	position               int64
	mu                     sync.Mutex
	pel                    *PerEventLooper
	AdditionalEnvResources map[string][]string
}

type aerScanner struct {
	File    *os.File
	Scanner *bufio.Scanner
}

type ArrayEventGeneratorInput struct {
	Event                  Event
	PerEventLoopData       []map[string]string
	StartIndex             int64
	EndIndex               int64
	AdditionalEnvResources map[string]any
}

func NewArrayEventGenerator2(input ArrayEventGeneratorInput) (*ArrayEventGenerator2, error) {
	manifestCount := len(input.Event.Manifests)

	//order the set of manifests
	//@TODO...need to order all event generator manifest sets!!!
	if manifestCount > 1 {
		orderedIds, err := input.Event.TopoSort()
		if err != nil {
			log.Printf("Unable to order event %s: %s\n", input.Event.ID, err)
		}
		orderedManifests := make([]ComputeManifest, len(input.Event.Manifests))
		for i, oid := range orderedIds {
			orderedManifests[i], err = getManifest(input.Event.Manifests, oid)
			if err != nil {
				log.Printf("Unable to order event %s: %s\n", input.Event.ID, err)
			}
		}
		input.Event.Manifests = orderedManifests
	}

	for i := 0; i < manifestCount; i++ {
		err := input.Event.Manifests[i].WritePayload()
		if err != nil {
			return nil, fmt.Errorf("failed to write payload for manifest %s: %s", input.Event.Manifests[i].ManifestID, err)
		}
	}

	perEventLooper := NewPerEventLooper(input.PerEventLoopData)
	aers := make(map[string][]string)
	for k, v := range input.AdditionalEnvResources {
		err := func() error {
			file, err := os.Open(v.(string))
			if err != nil {
				return err
			}
			defer file.Close()
			scanner := bufio.NewScanner(file)
			lines := []string{}
			for scanner.Scan() {
				lines = append(lines, scanner.Text())
			}
			aers[k] = lines
			return nil
		}()
		if err != nil {
			return nil, err
		}

	}

	return &ArrayEventGenerator2{
		event:                  input.Event,
		position:               input.StartIndex,
		end:                    input.EndIndex,
		pel:                    perEventLooper,
		AdditionalEnvResources: aers,
	}, nil
}

func (aeg *ArrayEventGenerator2) NextEvent() (Event, bool, error) {
	aeg.mu.Lock()
	defer aeg.mu.Unlock()
	event := aeg.event
	var additionalEnvVars map[string]string
	incrementEvent := true
	if aeg.pel != nil {
		additionalEnvVars, incrementEvent = aeg.pel.Next()
	}

	//add any additional vars from files
	for k, valset := range aeg.AdditionalEnvResources {
		additionalEnvVars[k] = valset[aeg.position-1]
	}
	event.AdditionalEventEnvVars = MapToKeyValuePairs(additionalEnvVars)
	event.EventIdentifier = strconv.Itoa(int(aeg.position))
	hasNext := true
	if incrementEvent {
		hasNext = aeg.position < aeg.end
		aeg.position++
	}
	return event, hasNext, nil

}

func getManifest(manifests []ComputeManifest, id uuid.UUID) (ComputeManifest, error) {
	for _, m := range manifests {
		if m.ManifestID == id {
			return m, nil
		}
	}
	return ComputeManifest{}, fmt.Errorf("unable to find manifest %s in list", id.String())
}
