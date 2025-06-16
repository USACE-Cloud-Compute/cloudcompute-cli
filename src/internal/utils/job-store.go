package utils

import (
	"fmt"
	"os"
	"sync"

	"github.com/google/uuid"
	. "github.com/usace/cloudcompute"
)

type CsvJobStore struct {
	f *os.File
	m *sync.Mutex
}

func NewCsvJobStore(filepath string) (*CsvJobStore, error) {
	f, err := os.Create(filepath)
	if err != nil {
		return nil, err
	}
	f.WriteString("\"Compute\",\"Event\",\"Job\",\"ComputeProviderJob\",\"Payload\",\"EventIdentifier\"\n")
	return &CsvJobStore{f, &sync.Mutex{}}, nil
}

func (cjs *CsvJobStore) SaveJob(computeId uuid.UUID, payloadId uuid.UUID, event string, job *Job) error {
	cjs.m.Lock()
	defer cjs.m.Unlock()
	_, err := cjs.f.WriteString(fmt.Sprintf("\"%s\",\"%s\",\"%s\",\"%s\",\"%s\",\"%s\"\n", computeId.String(), job.EventID.String(), job.ID, *job.SubmittedJob.JobId, payloadId.String(), event))
	return err
}
