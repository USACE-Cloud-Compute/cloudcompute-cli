package utils

import (
	"fmt"
	"sync"

	"github.com/usace/cloudcompute"
)

type RealzBlockEventGenerator struct {
	event      cloudcompute.Event
	realzPos   int64
	endRealz   int64
	startBlock int64
	blockPos   int64
	endBlock   int64
	numevent   int64
	mu         sync.Mutex
}

func NewRealzBlockEventGenerator(
	event cloudcompute.Event,
	startRealz int64,
	endRealz int64,
	startBlock int64,
	endBlock int64,
) (*RealzBlockEventGenerator, error) {
	return &RealzBlockEventGenerator{
		event:      event,
		realzPos:   startRealz,
		endRealz:   endRealz,
		startBlock: startBlock,
		blockPos:   startBlock,
		endBlock:   endBlock,
	}, nil
}

func (rbe *RealzBlockEventGenerator) NextEvent() (cloudcompute.Event, bool, error) {
	rbe.mu.Lock()
	defer rbe.mu.Unlock()
	rbe.numevent++
	event := rbe.event
	event.EventIdentifier = fmt.Sprintf("%d::%d::%d", rbe.numevent, rbe.realzPos, rbe.blockPos)
	hasNext := true

	if rbe.realzPos > rbe.endRealz { //last realz and block. we are done
		hasNext = false
	} else if rbe.blockPos >= rbe.endBlock { //at the end of the blocks, increment realz and set blocks to start
		rbe.realzPos++
		rbe.blockPos = rbe.startBlock
	} else {
		rbe.blockPos++
	}
	return event, hasNext, nil
}
