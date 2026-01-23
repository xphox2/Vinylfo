package services

import (
	"context"
	"log"
	"time"

	"vinylfo/duration"
	"vinylfo/models"

	"gorm.io/gorm"
)

type DurationWorker struct {
	db              *gorm.DB
	resolverService *DurationResolverService
	progressService *DurationProgressService
	stateManager    *duration.StateManager
	ctx             context.Context
	cancel          context.CancelFunc
}

func NewDurationWorker(
	db *gorm.DB,
	resolverService *DurationResolverService,
	stateManager *duration.StateManager,
	ctx context.Context,
	cancel context.CancelFunc,
) *DurationWorker {
	return &DurationWorker{
		db:              db,
		resolverService: resolverService,
		progressService: NewDurationProgressService(db),
		stateManager:    stateManager,
		ctx:             ctx,
		cancel:          cancel,
	}
}

func (w *DurationWorker) Run() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("DurationWorker panic: %v", r)
			w.stateManager.SetStatus(duration.ResolverStatusFailed)
			w.stateManager.UpdateState(func(s *duration.ResolverState) {
				s.LastError = "Worker panic"
			})
		}
	}()

	tracks, err := w.resolverService.GetTracksNeedingResolution()
	if err != nil {
		log.Printf("Failed to get tracks: %v", err)
		w.stateManager.SetStatus(duration.ResolverStatusFailed)
		return
	}

	if len(tracks) == 0 {
		log.Println("No tracks need resolution")
		w.stateManager.SetStatus(duration.ResolverStatusCompleted)
		return
	}

	w.stateManager.UpdateState(func(s *duration.ResolverState) {
		s.Status = duration.ResolverStatusRunning
		s.TotalTracks = len(tracks)
		s.ProcessedTracks = 0
	})

	savedProgress, _ := w.progressService.Load()
	startIndex := 0
	if savedProgress != nil && savedProgress.LastProcessedID > 0 {
		for i, t := range tracks {
			if t.ID == savedProgress.LastProcessedID {
				startIndex = i + 1
				w.stateManager.UpdateState(func(s *duration.ResolverState) {
					s.ProcessedTracks = savedProgress.ProcessedTracks
					s.ResolvedCount = savedProgress.ResolvedCount
					s.NeedsReviewCount = savedProgress.NeedsReviewCount
					s.FailedCount = savedProgress.FailedCount
				})
				break
			}
		}
	}

	for i := startIndex; i < len(tracks); i++ {
		select {
		case <-w.ctx.Done():
			w.stateManager.SetStatus(duration.ResolverStatusIdle)
			return
		case <-w.stateManager.StopChan():
			w.stateManager.SetStatus(duration.ResolverStatusIdle)
			return
		default:
		}

		if w.stateManager.IsPaused() {
			w.progressService.Save(w.stateManager.GetState())
			log.Println("Worker paused, waiting for resume...")
			w.stateManager.WaitForResume()
			log.Println("Worker resumed")
		}

		track := tracks[i]
		w.processTrack(&track)

		if i%10 == 0 {
			state := w.stateManager.GetState()
			w.progressService.Save(state)
		}

		// Small delay for context switching - rate limiters handle API pacing
		time.Sleep(100 * time.Millisecond)
	}

	w.stateManager.SetStatus(duration.ResolverStatusCompleted)
	w.progressService.MarkComplete()
	log.Println("Bulk resolution completed")
}

func (w *DurationWorker) processTrack(track *models.Track) {
	var album models.Album
	w.db.First(&album, track.AlbumID)

	w.stateManager.UpdateState(func(s *duration.ResolverState) {
		s.CurrentTrackID = track.ID
		s.CurrentTrack = album.Artist + " - " + track.Title
	})

	resolution, err := w.resolverService.ResolveTrackDuration(w.ctx, *track)

	w.stateManager.UpdateState(func(s *duration.ResolverState) {
		s.ProcessedTracks++

		if err != nil {
			s.FailedCount++
			s.LastError = err.Error()
			return
		}

		switch resolution.Status {
		case "resolved":
			s.ResolvedCount++
		case "needs_review":
			s.NeedsReviewCount++
		case "failed":
			s.FailedCount++
		}
	})
}
