package sync

import (
	"context"
	"time"
)

type LegacySyncState struct {
	IsRunning     bool
	IsPaused      bool
	CurrentPage   int
	TotalPages    int
	Processed     int
	Total         int
	SyncMode      string
	CurrentFolder int
	Folders       []map[string]interface{}
	FolderIndex   int
	APIRemaining  int
	AnonRemaining int
	LastBatch     *SyncBatch
	LastActivity  time.Time
}

type LegacyStateManager struct {
	manager *StateManager
}

func NewLegacyStateManager() *LegacyStateManager {
	return &LegacyStateManager{
		manager: DefaultManager,
	}
}

func (m *LegacyStateManager) GetState() LegacySyncState {
	state := m.manager.GetState()
	return LegacySyncState{
		IsRunning:     state.Status == SyncStatusRunning || state.Status == SyncStatusPaused,
		IsPaused:      state.Status == SyncStatusPaused,
		CurrentPage:   state.CurrentPage,
		TotalPages:    state.TotalPages,
		Processed:     state.Processed,
		Total:         state.Total,
		SyncMode:      state.SyncMode,
		CurrentFolder: state.CurrentFolder,
		Folders:       state.Folders,
		FolderIndex:   state.FolderIndex,
		APIRemaining:  state.APIRemaining,
		AnonRemaining: state.AnonRemaining,
		LastBatch:     state.LastBatch,
		LastActivity:  state.LastActivity,
	}
}

func (m *LegacyStateManager) UpdateState(fn func(*LegacySyncState)) {
	m.manager.UpdateState(func(s *SyncState) {
		legacy := LegacySyncState{
			IsRunning:     s.Status == SyncStatusRunning || s.Status == SyncStatusPaused,
			IsPaused:      s.Status == SyncStatusPaused,
			CurrentPage:   s.CurrentPage,
			TotalPages:    s.TotalPages,
			Processed:     s.Processed,
			Total:         s.Total,
			SyncMode:      s.SyncMode,
			CurrentFolder: s.CurrentFolder,
			Folders:       s.Folders,
			FolderIndex:   s.FolderIndex,
			APIRemaining:  s.APIRemaining,
			AnonRemaining: s.AnonRemaining,
			LastBatch:     s.LastBatch,
			LastActivity:  s.LastActivity,
		}
		fn(&legacy)
		if legacy.IsRunning && !legacy.IsPaused {
			s.Status = SyncStatusRunning
		} else if legacy.IsRunning && legacy.IsPaused {
			s.Status = SyncStatusPaused
		} else {
			s.Status = SyncStatusIdle
		}
		s.CurrentPage = legacy.CurrentPage
		s.TotalPages = legacy.TotalPages
		s.Processed = legacy.Processed
		s.Total = legacy.Total
		s.SyncMode = legacy.SyncMode
		s.CurrentFolder = legacy.CurrentFolder
		s.Folders = legacy.Folders
		s.FolderIndex = legacy.FolderIndex
		s.APIRemaining = legacy.APIRemaining
		s.AnonRemaining = legacy.AnonRemaining
		s.LastBatch = legacy.LastBatch
		s.LastActivity = legacy.LastActivity
	})
}

func (m *LegacyStateManager) Reset() {
	m.manager.Reset()
}

func (m *LegacyStateManager) RequestPause() bool {
	return m.manager.RequestPause()
}

func (m *LegacyStateManager) RequestResume() bool {
	return m.manager.RequestResume()
}

func (m *LegacyStateManager) RequestStop() bool {
	return m.manager.RequestStop()
}

func (m *LegacyStateManager) WaitForPause(ctx context.Context) error {
	return m.manager.WaitForPause(ctx)
}

func (m *LegacyStateManager) WaitForResume(ctx context.Context) error {
	return m.manager.WaitForResume(ctx)
}

func (m *LegacyStateManager) WaitForStop(ctx context.Context) error {
	return m.manager.WaitForStop(ctx)
}

func (m *LegacyStateManager) IsRunning() bool {
	state := m.manager.GetState()
	return state.Status == SyncStatusRunning || state.Status == SyncStatusPaused
}

func (m *LegacyStateManager) IsPaused() bool {
	state := m.manager.GetState()
	return state.Status == SyncStatusPaused
}

func (m *LegacyStateManager) IsActive() bool {
	state := m.manager.GetState()
	return state.Status == SyncStatusRunning || state.Status == SyncStatusPaused
}

var DefaultLegacyManager = NewLegacyStateManager()
