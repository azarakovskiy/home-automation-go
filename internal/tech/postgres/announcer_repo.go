package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

const announcerStateKey = "morning_summary"

// AnnouncerRepo persists announcer deduplication state to PostgreSQL.
// Implements priceannouncer.AnnouncerStateStore (duck typing — no import needed).
type AnnouncerRepo struct {
	db *sql.DB
}

// NewAnnouncerRepo constructs an AnnouncerRepo backed by the given *sql.DB.
func NewAnnouncerRepo(db *sql.DB) *AnnouncerRepo {
	return &AnnouncerRepo{db: db}
}

// LastAnnouncedDate returns the time of the last morning announcement.
func (r *AnnouncerRepo) LastAnnouncedDate(ctx context.Context) (time.Time, error) {
	var unix int64
	err := r.db.QueryRowContext(ctx,
		`SELECT announced_at FROM announcer_state WHERE key = $1`,
		announcerStateKey,
	).Scan(&unix)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("read announcer state: %w", err)
	}
	if unix == 0 {
		return time.Time{}, nil
	}
	return time.Unix(unix, 0).UTC(), nil
}

// SetLastAnnouncedDate records the time of the latest morning announcement.
func (r *AnnouncerRepo) SetLastAnnouncedDate(ctx context.Context, t time.Time) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO announcer_state (key, announced_at) VALUES ($1, $2)
		 ON CONFLICT (key) DO UPDATE SET announced_at = $2`,
		announcerStateKey, t.Unix(),
	)
	if err != nil {
		return fmt.Errorf("update announcer state: %w", err)
	}
	return nil
}
