package worker

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/config"
	"github.com/hafizkrm/Monitoring/backend/internal/logger"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

// jobHeap implements heap.Interface for Job
type jobHeap []Job

func (h jobHeap) Len() int           { return len(h) }
func (h jobHeap) Less(i, j int) bool { return h[i].ScheduledAt.Before(h[j].ScheduledAt) }
func (h jobHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *jobHeap) Push(x any)        { *h = append(*h, x.(Job)) }
func (h *jobHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}

type Scheduler struct {
	config       config.Config
	db           DatabaseClient
	logger       logger.Logger
	jobQueue     chan<- Job
	stopChan     chan struct{}
	syncInterval time.Duration

	devices     map[int]*models.Device
	pendingJobs map[int]struct{}
	pq          jobHeap
	mu          sync.Mutex

	stopped bool
}

func NewScheduler(
	cfg config.Config,
	db DatabaseClient,
	log logger.Logger,
	jobQueue chan<- Job,
) *Scheduler {

	interval, _ := time.ParseDuration(cfg.Polling.HealthCheckInterval)
	if interval <= 0 {
		interval = 60 * time.Second // DB sync interval
	}

	return &Scheduler{
		config:       cfg,
		db:           db,
		logger:       log,
		jobQueue:     jobQueue,
		stopChan:     make(chan struct{}),
		syncInterval: interval,
		devices:      make(map[int]*models.Device),
		pendingJobs:  make(map[int]struct{}),
		pq:           make(jobHeap, 0),
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	// Initial sync
	s.syncDevices(ctx)

	syncTicker := time.NewTicker(s.syncInterval)
	defer syncTicker.Stop()

	dispatchTicker := time.NewTicker(1 * time.Second)
	defer dispatchTicker.Stop()

	s.logger.Info("scheduler started with min-heap priority queue", map[string]interface{}{
		"sync_interval": s.syncInterval.String(),
	})

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopChan:
			return
		case <-syncTicker.C:
			s.syncDevices(ctx)
		case <-dispatchTicker.C:
			s.dispatchDueDevices()
		}
	}
}

func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return
	}

	close(s.stopChan)
	s.stopped = true
}

func (s *Scheduler) syncDevices(ctx context.Context) {
	devices, err := s.db.GetAllEnabledDevices(ctx)
	if err != nil {
		s.logger.Error("failed load devices for scheduler sync", map[string]interface{}{
			"error": err,
		})
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	currentIDs := make(map[int]bool)
	now := time.Now()

	for i := range devices {
		device := &devices[i]
		currentIDs[device.ID] = true

		if _, exists := s.devices[device.ID]; !exists {
			// New device, add to map and push to heap
			s.devices[device.ID] = device

			nextPoll := now
			if device.LastPolledAt != nil {
				interval := time.Duration(device.PollingInterval) * time.Second
				if interval <= 0 {
					interval = s.config.GetPollingInterval()
				}
				nextPoll = device.LastPolledAt.Add(interval)
			}
			heap.Push(&s.pq, Job{Device: device, ScheduledAt: nextPoll})
		} else {
			// Existing device, just update the struct memory in case fields changed
			s.devices[device.ID] = device
		}
	}

	// Remove devices that are no longer enabled/exist
	for id := range s.devices {
		if !currentIDs[id] {
			delete(s.devices, id)
		}
	}
}

func (s *Scheduler) dispatchDueDevices() {
	now := time.Now()

	for {
		s.mu.Lock()
		if s.pq.Len() == 0 || s.pq[0].ScheduledAt.After(now) {
			s.mu.Unlock()
			break
		}

		// Pop the due device
		job := heap.Pop(&s.pq).(Job)

		// Verify it still exists and is enabled
		device, exists := s.devices[job.Device.ID]
		if !exists {
			s.mu.Unlock()
			continue
		}

		// Prevent concurrent enqueues of the same device
		if _, pending := s.pendingJobs[device.ID]; pending {
			s.mu.Unlock()
			continue
		}

		// Reserve
		s.pendingJobs[device.ID] = struct{}{}
		job.Device = device // use latest struct
		s.mu.Unlock()

		// Attempt to send to worker queue
		select {
		case s.jobQueue <- job:
			s.logger.Debug("device queued", map[string]interface{}{"device": device.Name})
		default:
			// Queue is full, rollback and try again soon
			s.mu.Lock()
			delete(s.pendingJobs, device.ID)
			job.ScheduledAt = time.Now().Add(5 * time.Second) // backoff
			heap.Push(&s.pq, job)
			s.mu.Unlock()

			s.logger.Warn("worker queue full, job delayed", map[string]interface{}{"device": device.Name})
			return // If queue is full, no point popping more
		}
	}
}

func (s *Scheduler) markDone(deviceID int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	delete(s.pendingJobs, deviceID)

	// Reschedule for next interval
	if device, exists := s.devices[deviceID]; exists {
		interval := time.Duration(device.PollingInterval) * time.Second
		if interval <= 0 {
			interval = s.config.GetPollingInterval()
		}

		nextPoll := time.Now().Add(interval)
		heap.Push(&s.pq, Job{Device: device, ScheduledAt: nextPoll})
	}
}
