package elevation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Store struct {
	root string
}

func NewStore(root string) Store {
	return Store{root: root}
}

func (s Store) SaveRequest(request Request) error {
	if err := request.Validate(time.Now().UTC()); err != nil {
		return err
	}
	return s.writeJSON(s.requestPath(request.ID), request)
}

func (s Store) LoadRequest(id string) (Request, error) {
	var request Request
	if err := s.readJSON(s.requestPath(id), &request); err != nil {
		return Request{}, err
	}
	if request.ID != id {
		return Request{}, errors.New("elevation request id does not match its file")
	}
	return request, nil
}

func (s Store) SaveStatus(status OperationStatus) error {
	if err := validateID(status.ID); err != nil {
		return err
	}
	return s.writeJSON(s.resultPath(status.ID), status)
}

func (s Store) LoadStatus(id string) (OperationStatus, error) {
	var status OperationStatus
	if err := s.readJSON(s.resultPath(id), &status); err != nil {
		return OperationStatus{}, err
	}
	if status.ID != id {
		return OperationStatus{}, errors.New("elevation result id does not match its file")
	}
	return status, nil
}

func (s Store) RequestCancellation(id string) error {
	if err := validateID(id); err != nil {
		return err
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create elevation directory: %w", err)
	}
	return os.WriteFile(s.cancelPath(id), []byte("cancel"), 0o600)
}

func (s Store) CancellationRequested(id string) (bool, error) {
	if err := validateID(id); err != nil {
		return false, err
	}
	_, err := os.Stat(s.cancelPath(id))
	if err == nil {
		return true, nil
	}
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	return false, fmt.Errorf("read elevation cancellation marker: %w", err)
}

func (s Store) requestPath(id string) string { return s.path(id, ".request.json") }
func (s Store) resultPath(id string) string  { return s.path(id, ".result.json") }
func (s Store) cancelPath(id string) string  { return s.path(id, ".cancel") }

func (s Store) path(id, suffix string) string {
	return filepath.Join(s.root, id+suffix)
}

func (s Store) writeJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode elevation data: %w", err)
	}
	if err := os.MkdirAll(s.root, 0o700); err != nil {
		return fmt.Errorf("create elevation directory: %w", err)
	}
	temporary := path + ".tmp"
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return fmt.Errorf("write elevation data: %w", err)
	}
	if err := os.Rename(temporary, path); err != nil {
		return fmt.Errorf("publish elevation data: %w", err)
	}
	return nil
}

func (s Store) readJSON(path string, destination any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return fmt.Errorf("decode elevation data: %w", err)
	}
	return nil
}

func validateID(id string) error {
	if len(id) != 32 || strings.Trim(id, "0123456789abcdef") != "" {
		return errors.New("invalid elevation request id")
	}
	return nil
}
