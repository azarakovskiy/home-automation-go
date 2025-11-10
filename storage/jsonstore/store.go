package jsonstore

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"

	"home-go/dryrun"

	ga "saml.dev/gome-assistant"
)

const chunkSize = 240

// Store persists arbitrary JSON serializable data into one or more Home Assistant input_text entities.
type Store[T any] struct {
	service   *ga.Service
	state     ga.State
	entityIDs []string
}

// New returns a new Store bound to one or more input_text entities.
func New[T any](service *ga.Service, state ga.State, entityIDs ...string) *Store[T] {
	if len(entityIDs) == 0 {
		panic("jsonstore: at least one entity ID is required")
	}
	return &Store[T]{
		service:   service,
		state:     state,
		entityIDs: entityIDs,
	}
}

// Load deserializes the current entity value.
func (s *Store[T]) Load() (T, error) {
	var zero T
	if s.state == nil {
		return zero, fmt.Errorf("state is not initialized for %v", s.entityIDs)
	}

	var builder strings.Builder
	for _, entityID := range s.entityIDs {
		entityState, err := s.state.Get(entityID)
		if err != nil {
			log.Printf("WARNING: failed to read %s: %v (continuing)", entityID, err)
			continue
		}

		if entityState.State == "" || entityState.State == "unknown" || entityState.State == "unavailable" {
			continue
		}

		builder.WriteString(entityState.State)
	}

	serialized := strings.TrimSpace(builder.String())
	if serialized == "" {
		return zero, nil
	}

	payload, err := decodePayload(serialized)
	if err != nil {
		return zero, fmt.Errorf("failed to decode payload: %w", err)
	}

	var value T
	if err := json.Unmarshal(payload, &value); err != nil {
		return zero, fmt.Errorf("failed to unmarshal %v: %w", s.entityIDs, err)
	}

	return value, nil
}

// Save serializes and writes the value to Home Assistant.
func (s *Store[T]) Save(value T) error {
	bytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	payload, err := encodePayload(bytes)
	if err != nil {
		return fmt.Errorf("failed to encode payload: %w", err)
	}

	chunks := chunkString(payload, chunkSize)
	if len(chunks) > len(s.entityIDs) {
		return fmt.Errorf("payload (%d bytes) exceeds chunk capacity (%d chunks)", len(payload), len(s.entityIDs))
	}

	for i, entityID := range s.entityIDs {
		chunk := ""
		if i < len(chunks) {
			chunk = chunks[i]
		}

		if err := dryrun.CallWithData("InputText.Set", entityID, chunk, func() error {
			return s.service.InputText.Set(entityID, chunk)
		}); err != nil {
			return fmt.Errorf("failed to persist %s chunk %d: %w", entityID, i, err)
		}
	}

	log.Printf("Persisted %d bytes across %d chunk(s)", len(payload), len(chunks))
	return nil
}

func chunkString(value string, size int) []string {
	if size <= 0 {
		return []string{value}
	}

	var chunks []string
	for start := 0; start < len(value); start += size {
		end := start + size
		if end > len(value) {
			end = len(value)
		}
		chunks = append(chunks, value[start:end])
	}
	return chunks
}

func encodePayload(data []byte) (string, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return "", err
	}
	if err := gz.Close(); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func decodePayload(data string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		// Legacy plain-text payloads (pre-compression)
		return []byte(data), nil
	}

	reader, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	defer reader.Close()

	decoded, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return decoded, nil
}
