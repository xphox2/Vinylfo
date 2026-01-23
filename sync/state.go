package sync

import (
	"context"
	"sync"
	"time"
)

type SyncStatus string

const (
	SyncStatusIdle     SyncStatus = "idle"
	SyncStatusRunning  SyncStatus = "running"
	SyncStatusPaused   SyncStatus = "paused"
	SyncStatusStopping SyncStatus = "stopping"
)

type SyncBatch struct {
	ID          int                      `json:"id"`
	Albums      []map[string]interface{} `json:"albums"`
	ProcessedAt *time.Time               `json:"processed_at,omitempty"`
}

type SyncState struct {
	Status           SyncStatus               `json:"status"`
	CurrentPage      int                      `json:"current_page"`
	TotalPages       int                      `json:"total_pages"`
	Processed        int                      `json:"processed"`
	Total            int                      `json:"total"`
	SyncMode         string                   `json:"sync_mode"`
	CurrentFolder    int                      `json:"current_folder"`
	Folders          []map[string]interface{} `json:"folders,omitempty"`
	FolderIndex      int                      `json:"folder_index"`
	APIRemaining     int                      `json:"api_remaining"`
	AnonRemaining    int                      `json:"anon_remaining"`
	LastBatch        *SyncBatch               `json:"last_batch,omitempty"`
	LastReview       interface{}              `json:"last_review,omitempty"`
	LastActivity     time.Time                `json:"last_activity"`
	WorkerID         string                   `json:"worker_id"`
	RateLimitRetryAt *time.Time               `json:"rate_limit_retry_at,omitempty"`
	RateLimitMessage string                   `json:"rate_limit_message,omitempty"`
}

type StateManager struct {
	mu       sync.RWMutex
	state    SyncState
	statusCh chan SyncStatus
	pauseCh  chan struct{}
	resumeCh chan struct{}
	cancelCh chan struct{}
}

var (
	DefaultManager = &StateManager{
		state: SyncState{
			Status: SyncStatusIdle,
		},
		statusCh: make(chan SyncStatus, 1),
		pauseCh:  make(chan struct{}, 1),
		resumeCh: make(chan struct{}, 1),
		cancelCh: make(chan struct{}, 1),
	}
	activeWorkers   = make(map[string]*sync.WaitGroup)
	activeWorkersMu sync.RWMutex
)

// NewStateManager creates a new StateManager instance for testing or isolated use
func NewStateManager() *StateManager {
	return &StateManager{
		state: SyncState{
			Status: SyncStatusIdle,
		},
		statusCh: make(chan SyncStatus, 1),
		pauseCh:  make(chan struct{}, 1),
		resumeCh: make(chan struct{}, 1),
		cancelCh: make(chan struct{}, 1),
	}
}

func (m *StateManager) GetState() SyncState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

func (m *StateManager) UpdateState(fn func(*SyncState)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	fn(&m.state)
}

func (m *StateManager) SetStatus(status SyncStatus) {
	m.mu.Lock()
	oldStatus := m.state.Status
	m.state.Status = status
	m.mu.Unlock()

	select {
	case m.statusCh <- status:
	default:
	}

	if oldStatus == SyncStatusRunning && status == SyncStatusPaused {
		select {
		case m.pauseCh <- struct{}{}:
		default:
		}
	} else if oldStatus == SyncStatusPaused && status == SyncStatusRunning {
		select {
		case m.resumeCh <- struct{}{}:
		default:
		}
	}
}

func (m *StateManager) RequestPause() bool {
	m.mu.Lock()
	canPause := m.state.Status == SyncStatusRunning
	if canPause {
		m.state.Status = SyncStatusPaused
	}
	m.mu.Unlock()

	if canPause {
		select {
		case m.pauseCh <- struct{}{}:
		default:
		}
	}

	return canPause
}

func (m *StateManager) RequestResume() bool {
	m.mu.Lock()
	canResume := m.state.Status == SyncStatusPaused
	if canResume {
		m.state.Status = SyncStatusRunning
	}
	m.mu.Unlock()

	if canResume {
		select {
		case m.resumeCh <- struct{}{}:
		default:
		}
	}

	return canResume
}

func (m *StateManager) RequestStop() bool {
	m.mu.Lock()
	canStop := m.state.Status == SyncStatusRunning || m.state.Status == SyncStatusPaused
	if canStop {
		m.state.Status = SyncStatusStopping
	}
	m.mu.Unlock()

	if canStop {
		select {
		case m.cancelCh <- struct{}{}:
		default:
		}
	}
	return canStop
}

func (m *StateManager) WaitForPause(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Poll status - if not running, we're paused or stopped
			m.mu.RLock()
			currentStatus := m.state.Status
			m.mu.RUnlock()
			if currentStatus != SyncStatusRunning {
				return nil
			}
		}
	}
}

func (m *StateManager) WaitForResume(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			// Poll status - if not paused, we're resumed
			m.mu.RLock()
			currentStatus := m.state.Status
			m.mu.RUnlock()
			if currentStatus != SyncStatusPaused {
				return nil
			}
		}
	}
}

func (m *StateManager) WaitForStop(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-m.cancelCh:
			return nil
		case status := <-m.statusCh:
			if status == SyncStatusIdle || status == SyncStatusStopping {
				return nil
			}
		}
	}
}

func (m *StateManager) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = SyncState{
		Status: SyncStatusIdle,
	}
}

func (s *SyncState) IsRunning() bool {
	return s.Status == SyncStatusRunning
}

func (s *SyncState) IsPaused() bool {
	return s.Status == SyncStatusPaused
}

func (s *SyncState) IsActive() bool {
	return s.Status == SyncStatusRunning || s.Status == SyncStatusPaused
}

func (s *SyncState) IsIdle() bool {
	return s.Status == SyncStatusIdle
}

func RegisterWorker(workerID string) {
	activeWorkersMu.Lock()
	defer activeWorkersMu.Unlock()
	activeWorkers[workerID] = &sync.WaitGroup{}
	activeWorkers[workerID].Add(1)
}

func UnregisterWorker(workerID string) {
	activeWorkersMu.Lock()
	defer activeWorkersMu.Unlock()
	if wg, ok := activeWorkers[workerID]; ok {
		wg.Done()
		delete(activeWorkers, workerID)
	}
}

func IsWorkerRunning(workerID string) bool {
	activeWorkersMu.RLock()
	defer activeWorkersMu.RUnlock()
	_, ok := activeWorkers[workerID]
	return ok
}

func WaitForWorker(workerID string, timeout time.Duration) bool {
	activeWorkersMu.RLock()
	wg, ok := activeWorkers[workerID]
	activeWorkersMu.RUnlock()

	if !ok {
		return true
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return true
	case <-time.After(timeout):
		return false
	}
}

func TerminateAllWorkers() {
	activeWorkersMu.Lock()
	defer activeWorkersMu.Unlock()
	for workerID, wg := range activeWorkers {
		wg.Done()
		delete(activeWorkers, workerID)
	}
}

func RemoveFirstAlbumFromBatch(s *SyncState) {
	if s.LastBatch != nil && len(s.LastBatch.Albums) > 0 {
		s.LastBatch.Albums = s.LastBatch.Albums[1:]
		if len(s.LastBatch.Albums) == 0 {
			s.LastBatch = nil
		}
	}
}

func (m *StateManager) ClearRateLimitState() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.RateLimitRetryAt = nil
	m.state.RateLimitMessage = ""
}

func (m *StateManager) SetRateLimitState(retryAt time.Time, message string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state.RateLimitRetryAt = &retryAt
	m.state.RateLimitMessage = message
}
