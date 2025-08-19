package utils

import (
	"fmt"
	"testing"

	"github.com/usace/cc-go-sdk"
	"github.com/usace/cloudcompute"
)

func TestBlockEventGenerator(t *testing.T) {
	event := generateTestEvent()
	eg, _ := NewRealzBlockEventGenerator(event, 2, 5, 10, 15)
	for {
		event, hasNext, _ := eg.NextEvent()
		if !hasNext {
			break
		}
		fmt.Printf("Event Identifier: %s, hasNext %v\n", event.EventIdentifier, hasNext)
	}
}

func generateTestEvent() cloudcompute.Event {
	return cloudcompute.Event{
		Manifests: []cloudcompute.ComputeManifest{
			{
				Inputs: cloudcompute.PluginInputs{
					PayloadAttributes: make(cc.PayloadAttributes),
				},
			},
		},
	}
}
