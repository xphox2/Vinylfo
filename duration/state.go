package duration

import (
	"sync"
)

// ResolverStatus represents the current state of the duration resolver
type ResolverStatus string

const (
	ResolverStatusIdle      ResolverStatus = "idle"
	ResolverStatusRunning   ResolverStatus = "running"
	ResolverStatusPaused    ResolverStatus = "paused"
	ResolverStatusStopping  ResolverStatus = "stopping"
	ResolverStatusCompleted ResolverStatus = "completed"
	ResolverStatusFailed    ResolverStatus = "failed"
)

// ResolverState holds the current state of the duration resolver
type ResolverState struct {
	Status           ResolverStatus `json:"status"`
	TotalTracks      int            `json:"total_tracks"`
	ProcessedTracks  int            `json:"processed_tracks"`
	ResolvedCount    int            `json:"resolved_count"`
	NeedsReviewCount int            `json:"needs_review_count"`
	FailedCount      int            `json:"failed_count"`
	SkippedCount     int            `json:"skipped_count"`
	CurrentTrackID   uint           `json:"current_track_id"`
	CurrentTrack     string         `json:"current_track"` // "Artist - Title" for display
	LastError        string         `json:"last_error"`
}

// StateManager provides thread-safe access to resolver state
type StateManager struct {
	mu       sync.RWMutex
	state    ResolverState
	pauseCh  chan struct{}
	resumeCh chan struct{}
	stopCh   chan struct{}
}

// NewStateManager creates a new state manager
func NewStateManager() *StateManager {
	return &StateManager{
		state: ResolverState{
			Status: ResolverStatusIdle,
		},
		pauseCh:  make(chan struct{}),
		resumeCh: make(chan struct{}),
		stopCh:   make(chan struct{}),
	}
}

// GetState returns a copy of the current state
func (m *StateManager) GetState() ResolverState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// UpdateState safely updates the state using a callback
func (m *StateManager) UpdateState(fn func(*ResolverState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn(&m.state)
}

// SetStatus updates just the status
func (m *StateManager) SetStatus(status ResolverStatus) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.Status = status
}

// IsRunning returns true if resolver is actively processing
func (m *StateManager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.Status == ResolverStatusRunning
}

// IsPaused returns true if resolver is paused
func (m *StateManager) IsPaused() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.Status == ResolverStatusPaused
}

// RequestPause signals the worker to pause
func (m *StateManager) RequestPause() {
	m.SetStatus(ResolverStatusPaused)
}

// RequestResume signals the worker to resume
func (m *StateManager) RequestResume() {
	m.SetStatus(ResolverStatusRunning)
	select {
	case m.resumeCh <- struct{}{}:
	default:
	}
}

// RequestStop signals the worker to stop
func (m *StateManager) RequestStop() {
	m.SetStatus(ResolverStatusStopping)
	select {
	case m.stopCh <- struct{}{}:
	default:
	}
}

// WaitForResume blocks until resume is signaled (used by worker)
func (m *StateManager) WaitForResume() {
	<-m.resumeCh
}

// StopChan returns the stop channel for select statements
func (m *StateManager) StopChan() <-chan struct{} {
	return m.stopCh
}

// Reset resets the state manager for a new run
func (m *StateManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = ResolverState{
		Status: ResolverStatusIdle,
	}
	// Recreate channels
	m.pauseCh = make(chan struct{})
	m.resumeCh = make(chan struct{})
	m.stopCh = make(chan struct{})
}
