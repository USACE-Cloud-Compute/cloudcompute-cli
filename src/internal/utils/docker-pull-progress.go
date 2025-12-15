package utils

import (
	"sync"

	cloudcompute "github.com/usace-cloud-compute/cloudcompute/providers/docker"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

type CliDockerPullProgress struct {
	mu   sync.Mutex
	p    *mpb.Progress
	bars map[string]*mpb.Bar
}

type CliDockerPullProgressFactory struct{}

func (ppf *CliDockerPullProgressFactory) New() cloudcompute.DockerPullProgress {
	p := mpb.New(
		mpb.WithWidth(64),
		mpb.WithWaitGroup(&sync.WaitGroup{}), // Use a WaitGroup for proper synchronization
	)

	bars := make(map[string]*mpb.Bar)

	return &CliDockerPullProgress{
		p:    p,
		bars: bars,
	}
}

func (dpp *CliDockerPullProgress) Close() {
	dpp.p.Wait()
}

func (dpp *CliDockerPullProgress) Update(msg cloudcompute.DockerEvent) {
	// We only care about messages with a layer ID
	if msg.ID == "" {
		return
	}

	dpp.mu.Lock()
	bar, ok := dpp.bars[msg.ID]
	if !ok && msg.ProgressDetail.Total > 0 {
		bar = dpp.p.New(
			msg.ProgressDetail.Total,
			mpb.BarStyle().Lbound("[").Filler("=").Tip(">").Padding("-").Rbound("]"),
			mpb.PrependDecorators(
				decor.Name("Layer: "+msg.ID, decor.WC{W: 22, C: decor.DindentRight}),
				decor.CountersKibiByte(
					"%8.2f / %8.2f",
					decor.WC{W: 19, C: decor.DindentRight}),
			),
			mpb.AppendDecorators(
				decor.Percentage(),
			),
		)
		dpp.bars[msg.ID] = bar
	} else if ok {
		if msg.ProgressDetail.Total > 0 {
			bar.SetTotal(msg.ProgressDetail.Total, false)
		}

		if msg.ProgressDetail.Current > 0 {
			bar.SetCurrent(msg.ProgressDetail.Current)
		}
	}
	dpp.mu.Unlock()
}
