package opjournal

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/kiddingbaby/agx/internal/adapters/fileutil"
	"github.com/kiddingbaby/agx/internal/ports"
	"gopkg.in/yaml.v3"
)

var _ ports.OperationJournal = (*Journal)(nil)

type Journal struct {
	path string
}

func New(path string) *Journal {
	return &Journal{path: path}
}

func (j *Journal) Current() (*ports.OperationRecord, error) {
	record, exists, err := j.load()
	if err != nil || !exists {
		return nil, err
	}
	return &record, nil
}

func (j *Journal) Begin(record ports.OperationRecord) error {
	current, exists, err := j.load()
	if err != nil {
		return err
	}
	if exists && strings.TrimSpace(current.ID) != "" && current.ID != record.ID {
		return fmt.Errorf("unfinished AGX operation %s detected; run `agx doctor` before changing configs again", current.ID)
	}
	return j.write(record)
}

func (j *Journal) Update(record ports.OperationRecord) error {
	return j.write(record)
}

func (j *Journal) Clear(id string) error {
	current, exists, err := j.load()
	if err != nil {
		return err
	}
	if !exists {
		return nil
	}
	if strings.TrimSpace(id) != "" && current.ID != id {
		return fmt.Errorf("operation journal mismatch: current=%s clear=%s", current.ID, id)
	}
	if err := os.Remove(j.path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (j *Journal) load() (ports.OperationRecord, bool, error) {
	data, err := os.ReadFile(j.path)
	if err != nil {
		if os.IsNotExist(err) {
			return ports.OperationRecord{}, false, nil
		}
		return ports.OperationRecord{}, false, err
	}

	var record ports.OperationRecord
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if err := decoder.Decode(&record); err != nil {
		return ports.OperationRecord{}, false, err
	}
	return record, true, nil
}

func (j *Journal) write(record ports.OperationRecord) error {
	data, err := yaml.Marshal(record)
	if err != nil {
		return err
	}
	return fileutil.AtomicWriteFile(j.path, data, 0600)
}
