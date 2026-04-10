package scheduler

import (
	"context"
	"log"
	"sync"

	cron "github.com/robfig/cron/v3"

	"github.com/mountain-pass/lazy-tcp-proxy/internal/types"
)

// CronActions is implemented by the proxy server to execute scheduler-triggered
// start and stop operations. Defined here so the scheduler package does not
// need to import the proxy package.
type CronActions interface {
	CronStart(ctx context.Context, targetID, targetName string)
	CronStop(ctx context.Context, targetID, targetName string)
}

// Scheduler manages per-target cron jobs for start and stop schedules.
type Scheduler struct {
	cron    *cron.Cron
	mu      sync.Mutex
	entries map[string][]cron.EntryID // targetID → registered entry IDs
	actions CronActions
	ctx     context.Context
}

// New creates a new Scheduler backed by the given CronActions handler.
// Call Start() to begin processing schedules.
func New(ctx context.Context, actions CronActions) *Scheduler {
	return &Scheduler{
		cron:    cron.New(),
		entries: make(map[string][]cron.EntryID),
		actions: actions,
		ctx:     ctx,
	}
}

// Start begins the cron scheduler's tick loop.
func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop halts the cron scheduler, waiting for any running jobs to complete.
func (s *Scheduler) Stop() {
	s.cron.Stop()
}

// Register adds cron jobs for the start and/or stop schedules declared in info.
// If the target is already registered, its existing jobs are replaced.
// Targets with neither CronStart nor CronStop set are silently ignored.
func (s *Scheduler) Register(info types.TargetInfo) {
	if info.CronStart == "" && info.CronStop == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove any existing entries for this target before re-adding.
	s.unregisterLocked(info.ContainerID)

	var ids []cron.EntryID

	if info.CronStart != "" {
		// Capture loop variables for the closure.
		targetID := info.ContainerID
		targetName := info.ContainerName
		id, err := s.cron.AddFunc(info.CronStart, func() {
			s.actions.CronStart(s.ctx, targetID, targetName)
		})
		if err != nil {
			// Should not happen — expression was validated at parse time.
			log.Printf("scheduler: \033[33m%s\033[0m: failed to add cron-start job: %v", info.ContainerName, err)
		} else {
			ids = append(ids, id)
		}
	}

	if info.CronStop != "" {
		targetID := info.ContainerID
		targetName := info.ContainerName
		id, err := s.cron.AddFunc(info.CronStop, func() {
			s.actions.CronStop(s.ctx, targetID, targetName)
		})
		if err != nil {
			log.Printf("scheduler: \033[33m%s\033[0m: failed to add cron-stop job: %v", info.ContainerName, err)
		} else {
			ids = append(ids, id)
		}
	}

	if len(ids) > 0 {
		s.entries[info.ContainerID] = ids
		log.Printf("scheduler: registered \033[33m%s\033[0m (cron-start=%q cron-stop=%q)",
			info.ContainerName, info.CronStart, info.CronStop)
	}
}

// Unregister removes all cron jobs for the given target ID.
func (s *Scheduler) Unregister(targetID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unregisterLocked(targetID)
}

// unregisterLocked removes cron entries for targetID. Caller must hold s.mu.
func (s *Scheduler) unregisterLocked(targetID string) {
	ids, ok := s.entries[targetID]
	if !ok {
		return
	}
	for _, id := range ids {
		s.cron.Remove(id)
	}
	delete(s.entries, targetID)
}
