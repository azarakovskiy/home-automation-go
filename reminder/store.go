package reminder

import (
	"encoding/json"
	"fmt"
	"log"

	"home-go/dryrun"

	ga "saml.dev/gome-assistant"
)

type jsonStore[T any] struct {
	service  *ga.Service
	state    ga.State
	entityID string
}

func newJSONStore[T any](service *ga.Service, state ga.State, entityID string) *jsonStore[T] {
	return &jsonStore[T]{service: service, state: state, entityID: entityID}
}

func (s *jsonStore[T]) Load() (T, error) {
	var zero T

	if s.state == nil {
		return zero, fmt.Errorf("state is not initialized for %s", s.entityID)
	}

	entityState, err := s.state.Get(s.entityID)
	if err != nil {
		log.Printf("WARNING: failed to read %s: %v (defaulting to empty state)", s.entityID, err)
		return zero, nil
	}

	if entityState.State == "" || entityState.State == "unknown" || entityState.State == "unavailable" {
		return zero, nil
	}

	var value T
	if err := json.Unmarshal([]byte(entityState.State), &value); err != nil {
		return zero, fmt.Errorf("failed to unmarshal %s: %w", s.entityID, err)
	}

	return value, nil
}

func (s *jsonStore[T]) Save(value T) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", s.entityID, err)
	}

	payload := string(bytes)
	if err := dryrun.CallWithData("InputText.Set", s.entityID, payload, func() error {
		return s.service.InputText.Set(s.entityID, payload)
	}); err != nil {
		return fmt.Errorf("failed to persist %s: %w", s.entityID, err)
	}

	log.Printf("Persisted %d bytes to %s", len(payload), s.entityID)
	return nil
}
