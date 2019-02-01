package mock

import (
	"context"
	"errors"
)

type Mock struct {
	IsBackendReachable bool
	IsQueryValid       bool
	IsContentFound     bool
}

func (m *Mock) Close(ctx context.Context) error {
	return nil
}

func (m *Mock) Healthcheck() (string, error) {
	return "mock", nil
}

func (m *Mock) checkForErrors() error {
	if m.IsBackendReachable != true {
		return errors.New("database unavailble - 500")
	}

	if m.IsQueryValid != true {
		return errors.New("invalid query - 400")
	}

	if m.IsContentFound != true {
		return errors.New("not found - 404")
	}

	return nil
}
