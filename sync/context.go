package sync

import (
	"context"
	"sync"
	"time"
)

type SyncContext struct {
	ctx    context.Context
	cancel context.CancelFunc
	state  *StateManager
	wg     sync.WaitGroup
	mu     sync.Mutex
	done   chan struct{}
}

func NewSyncContext(state *StateManager) *SyncContext {
	ctx, cancel := context.WithCancel(context.Background())
	return &SyncContext{
		ctx:    ctx,
		cancel: cancel,
		state:  state,
		done:   make(chan struct{}),
	}
}

func (sc *SyncContext) Context() context.Context {
	return sc.ctx
}

func (sc *SyncContext) Done() <-chan struct{} {
	return sc.done
}

func (sc *SyncContext) Cancel(reason string) {
	sc.state.SetStatus(SyncStatusStopping)
	sc.cancel()
}

func (sc *SyncContext) IsCancelled() bool {
	select {
	case <-sc.ctx.Done():
		return true
	default:
		return false
	}
}

func (sc *SyncContext) AwaitPause(ctx context.Context) error {
	return sc.state.WaitForPause(ctx)
}

func (sc *SyncContext) AwaitResume(ctx context.Context) error {
	return sc.state.WaitForResume(ctx)
}

func (sc *SyncContext) AwaitStop(ctx context.Context) error {
	return sc.state.WaitForStop(ctx)
}

func (sc *SyncContext) ShouldPause() bool {
	return sc.state.GetState().Status == SyncStatusPaused
}

func (sc *SyncContext) ShouldStop() bool {
	return sc.state.GetState().Status == SyncStatusStopping || sc.state.GetState().Status == SyncStatusIdle
}

func (sc *SyncContext) Checkpoint() error {
	sc.state.UpdateState(func(s *SyncState) {
		s.LastActivity = time.Now()
	})
	return nil
}

func (sc *SyncContext) WaitForStateChange(ctx context.Context, check func(SyncState) bool) error {
	stateCh := make(chan SyncState, 1)
	go func() {
		for {
			state := sc.state.GetState()
			if check(state) {
				select {
				case stateCh <- state:
				default:
				}
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-sc.done:
		return nil
	case state := <-stateCh:
		_ = state
		return nil
	}
}

func (sc *SyncContext) Close() {
	close(sc.done)
	sc.wg.Wait()
}

type SyncProcessor struct {
	ctx       *SyncContext
	state     *StateManager
	batchSize int
	syncMode  string
}

func NewSyncProcessor(state *StateManager, batchSize int, syncMode string) *SyncProcessor {
	return &SyncProcessor{
		ctx:       NewSyncContext(state),
		state:     state,
		batchSize: batchSize,
		syncMode:  syncMode,
	}
}

func (sp *SyncContext) Start() {
	sp.state.SetStatus(SyncStatusRunning)
}

func (sp *SyncContext) Pause() {
	sp.state.RequestPause()
}

func (sp *SyncContext) Resume() {
	sp.state.RequestResume()
}

func (sp *SyncContext) Stop() {
	sp.state.RequestStop()
}

func (sp *SyncContext) Status() SyncStatus {
	return sp.state.GetState().Status
}

type PauseSignal struct {
	Timestamp time.Time
	Requested bool
}

func NewPauseSignal() *PauseSignal {
	return &PauseSignal{
		Timestamp: time.Now(),
		Requested: false,
	}
}

type PauseManager struct {
	mu        sync.RWMutex
	paused    bool
	pauseCh   chan struct{}
	resumeCh  chan struct{}
	signals   []PauseSignal
	maxSignal int
}

func NewPauseManager() *PauseManager {
	return &PauseManager{
		pauseCh:   make(chan struct{}, 1),
		resumeCh:  make(chan struct{}, 1),
		signals:   make([]PauseSignal, 0),
		maxSignal: 100,
	}
}

func (pm *PauseManager) Pause() bool {
	pm.mu.Lock()
	canPause := !pm.paused
	if canPause {
		pm.paused = true
		pm.signals = append(pm.signals, PauseSignal{
			Timestamp: time.Now(),
			Requested: true,
		})
		if len(pm.signals) > pm.maxSignal {
			pm.signals = pm.signals[len(pm.signals)-pm.maxSignal:]
		}
		select {
		case pm.pauseCh <- struct{}{}:
		default:
		}
	}
	pm.mu.Unlock()
	return canPause
}

func (pm *PauseManager) Resume() bool {
	pm.mu.Lock()
	canResume := pm.paused
	if canResume {
		pm.paused = false
		pm.signals = append(pm.signals, PauseSignal{
			Timestamp: time.Now(),
			Requested: false,
		})
		if len(pm.signals) > pm.maxSignal {
			pm.signals = pm.signals[len(pm.signals)-pm.maxSignal:]
		}
		select {
		case pm.resumeCh <- struct{}{}:
		default:
		}
	}
	pm.mu.Unlock()
	return canResume
}

func (pm *PauseManager) IsPaused() bool {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	return pm.paused
}

func (pm *PauseManager) WaitForPause(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pm.pauseCh:
			return nil
		case <-pm.resumeCh:
			return nil
		}
	}
}

func (pm *PauseManager) WaitForResume(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-pm.resumeCh:
			return nil
		case <-pm.pauseCh:
			return nil
		}
	}
}

func (pm *PauseManager) Signals() []PauseSignal {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	result := make([]PauseSignal, len(pm.signals))
	copy(result, pm.signals)
	return result
}

type ContextAdapter struct {
	pauseManager *PauseManager
	stateManager *StateManager
}

func NewContextAdapter() *ContextAdapter {
	return &ContextAdapter{
		pauseManager: NewPauseManager(),
		stateManager: DefaultManager,
	}
}

func (ca *ContextAdapter) Pause() bool {
	return ca.pauseManager.Pause()
}

func (ca *ContextAdapter) Resume() bool {
	return ca.pauseManager.Resume()
}

func (ca *ContextAdapter) IsPaused() bool {
	return ca.pauseManager.IsPaused()
}

func (ca *ContextAdapter) Status() SyncStatus {
	return ca.stateManager.GetState().Status
}

func (ca *ContextAdapter) WaitForPause(ctx context.Context) error {
	return ca.pauseManager.WaitForPause(ctx)
}

func (ca *ContextAdapter) WaitForResume(ctx context.Context) error {
	return ca.pauseManager.WaitForResume(ctx)
}

func (ca *ContextAdapter) Checkpoint() {
	ca.stateManager.UpdateState(func(s *SyncState) {
		s.LastActivity = time.Now()
	})
}

func (ca *ContextAdapter) ShouldPause() bool {
	return ca.pauseManager.IsPaused()
}

func (ca *ContextAdapter) ShouldStop() bool {
	status := ca.stateManager.GetState().Status
	return status == SyncStatusStopping || status == SyncStatusIdle
}
