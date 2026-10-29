// DigiPanel Backup Scheduling (modules/backup/schedule.go)
// Persistent backup schedules with an explicit frequency, hour, and retention
// policy. NextRun is computed in the operator's local clock and stored with the
// schedule so a missed window is visible rather than silently skipped.
// Zero emojis, strictly production grade.

package backup

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const scheduleFile = "schedules.json"

var (
	ErrInvalidFrequency = errors.New("frequency must be daily, weekly, or monthly")
	ErrInvalidHour      = errors.New("hour must be between 0 and 23")
	ErrInvalidRetention = errors.New("retention must be between 1 and 365")
	ErrScheduleExists   = errors.New("a schedule for this account already exists")
	ErrScheduleNotFound = errors.New("backup schedule not found")
)

// Schedule describes one recurring backup.
type Schedule struct {
	ID        string     `json:"id"`
	Account   string     `json:"account"`
	Label     string     `json:"label"`
	Frequency string     `json:"frequency"`
	Hour      int        `json:"hour"`
	Retention int        `json:"retention"`
	Enabled   bool       `json:"enabled"`
	CreatedAt time.Time  `json:"createdAt"`
	LastRun   *time.Time `json:"lastRun,omitempty"`
	NextRun   time.Time  `json:"nextRun"`
}

// ValidateFrequency checks the allowed recurrence values.
func ValidateFrequency(frequency string) error {
	switch strings.ToLower(strings.TrimSpace(frequency)) {
	case "daily", "weekly", "monthly":
		return nil
	default:
		return ErrInvalidFrequency
	}
}

// ComputeNextRun returns the first window strictly after from.
func ComputeNextRun(frequency string, hour int, from time.Time) (time.Time, error) {
	if err := ValidateFrequency(frequency); err != nil {
		return time.Time{}, err
	}
	if hour < 0 || hour > 23 {
		return time.Time{}, ErrInvalidHour
	}
	base := time.Date(from.Year(), from.Month(), from.Day(), hour, 0, 0, 0, from.Location())
	switch strings.ToLower(strings.TrimSpace(frequency)) {
	case "daily":
		if !base.After(from) {
			base = base.AddDate(0, 0, 1)
		}
	case "weekly":
		// Weekly windows land on Monday.
		for base.Weekday() != time.Monday || !base.After(from) {
			base = base.AddDate(0, 0, 1)
		}
	case "monthly":
		base = time.Date(from.Year(), from.Month(), 1, hour, 0, 0, 0, from.Location())
		if !base.After(from) {
			base = base.AddDate(0, 1, 0)
		}
	}
	return base, nil
}

func (s Schedule) due(now time.Time) bool {
	return s.Enabled && !s.NextRun.After(now)
}

func (e *Engine) schedulePath() string {
	return filepath.Join(e.opts.Root, scheduleFile)
}

// Schedules returns every stored schedule ordered by next run.
func (e *Engine) Schedules() ([]Schedule, error) {
	content, err := os.ReadFile(e.schedulePath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Schedule{}, nil
		}
		return nil, err
	}
	var stored []Schedule
	if err := json.Unmarshal(content, &stored); err != nil {
		return nil, fmt.Errorf("parse backup schedules: %w", err)
	}
	out := make([]Schedule, len(stored))
	copy(out, stored)
	sort.Slice(out, func(i, j int) bool { return out[i].NextRun.Before(out[j].NextRun) })
	return out, nil
}

func (e *Engine) writeSchedules(items []Schedule) error {
	if err := os.MkdirAll(e.opts.Root, 0700); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(items, "", "  ")
	if err != nil {
		return err
	}
	path := e.schedulePath()
	temporary := path + ".partial"
	if err := os.WriteFile(temporary, append(encoded, '\n'), 0600); err != nil {
		return fmt.Errorf("write backup schedules: %w", err)
	}
	return os.Rename(temporary, path)
}

// SaveSchedule inserts or replaces the schedule for an account.
func (e *Engine) SaveSchedule(schedule Schedule) (*Schedule, error) {
	if !accountPattern.MatchString(schedule.Account) {
		return nil, ErrInvalidAccount
	}
	if err := ValidateFrequency(schedule.Frequency); err != nil {
		return nil, err
	}
	if schedule.Hour < 0 || schedule.Hour > 23 {
		return nil, ErrInvalidHour
	}
	if schedule.Retention < 1 || schedule.Retention > 365 {
		return nil, ErrInvalidRetention
	}
	if err := ValidateLabel(schedule.Label); err != nil {
		return nil, err
	}

	items, err := e.Schedules()
	if err != nil {
		return nil, err
	}

	now := e.now()
	schedule.Frequency = strings.ToLower(strings.TrimSpace(schedule.Frequency))
	next, err := ComputeNextRun(schedule.Frequency, schedule.Hour, now)
	if err != nil {
		return nil, err
	}
	schedule.NextRun = next
	if schedule.ID == "" {
		suffix := make([]byte, 4)
		if _, err := rand.Read(suffix); err != nil {
			return nil, err
		}
		schedule.ID = "sch-" + hex.EncodeToString(suffix)
		schedule.CreatedAt = now.UTC()
	}

	replaced := false
	for index, item := range items {
		if item.ID == schedule.ID || item.Account == schedule.Account {
			if schedule.CreatedAt.IsZero() {
				schedule.CreatedAt = item.CreatedAt
			}
			if item.ID != schedule.ID && schedule.ID != item.ID {
				// Keep one schedule per account: replace the existing record.
				schedule.ID = item.ID
			}
			items[index] = schedule
			replaced = true
			break
		}
	}
	if !replaced {
		items = append(items, schedule)
	}
	if err := e.writeSchedules(items); err != nil {
		return nil, err
	}
	copied := schedule
	return &copied, nil
}

// DeleteSchedule removes a schedule by identifier.
func (e *Engine) DeleteSchedule(id string) error {
	items, err := e.Schedules()
	if err != nil {
		return err
	}
	remaining := make([]Schedule, 0, len(items))
	found := false
	for _, item := range items {
		if item.ID == id {
			found = true
			continue
		}
		remaining = append(remaining, item)
	}
	if !found {
		return ErrScheduleNotFound
	}
	return e.writeSchedules(remaining)
}

// MarkScheduleRun records a completed run and advances the next window.
func (e *Engine) MarkScheduleRun(id string, at time.Time) (*Schedule, error) {
	items, err := e.Schedules()
	if err != nil {
		return nil, err
	}
	for index, item := range items {
		if item.ID != id {
			continue
		}
		stamp := at.UTC()
		item.LastRun = &stamp
		next, err := ComputeNextRun(item.Frequency, item.Hour, at)
		if err != nil {
			return nil, err
		}
		item.NextRun = next
		items[index] = item
		if err := e.writeSchedules(items); err != nil {
			return nil, err
		}
		copied := item
		return &copied, nil
	}
	return nil, ErrScheduleNotFound
}

// Due returns the enabled schedules whose window has arrived.
func (e *Engine) Due(now time.Time) ([]Schedule, error) {
	items, err := e.Schedules()
	if err != nil {
		return nil, err
	}
	due := make([]Schedule, 0, len(items))
	for _, item := range items {
		if item.due(now) {
			due = append(due, item)
		}
	}
	return due, nil
}
