package worker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/yourusername/viscod/internal/config"
	"github.com/yourusername/viscod/internal/models"
)

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestScheduler_SyncAndDispatch(t *testing.T) {
	ctx := context.Background()
	jobQueue := make(chan Job, 10)

	now := time.Now()
	devices := []models.Device{
		{ID: 1, Name: "Device 1", PollingInterval: 60, LastPolledAt: nil}, // Due now
		{ID: 2, Name: "Device 2", PollingInterval: 60, LastPolledAt: timePtr(now.Add(30 * time.Second))}, // Not due yet
		{ID: 3, Name: "Device 3", PollingInterval: 60, LastPolledAt: timePtr(now.Add(-90 * time.Second))}, // Overdue (due now)
	}

	mockDB := &MockDatabase{
		GetAllEnabledDevicesFunc: func(ctx context.Context) ([]models.Device, error) {
			return devices, nil
		},
	}

	s := NewScheduler(config.Config{}, mockDB, &MockLogger{}, jobQueue)

	// Set default interval if config is empty
	s.config.Polling.DefaultInterval = "60s"

	// Sync devices (loads from DB and pushes to Min-Heap)
	s.syncDevices(ctx)

	// Dispatch due devices
	s.dispatchDueDevices()

	// We expect Device 1 and Device 3 to be enqueued
	count := 0
Loop:
	for {
		select {
		case job := <-jobQueue:
			count++
			if job.Device.ID == 2 {
				t.Error("Device 2 should not have been enqueued")
			}
		default:
			break Loop
		}
	}

	if count != 2 {
		t.Errorf("expected 2 jobs enqueued, got %d", count)
	}
}

func TestScheduler_Concurrency(t *testing.T) {
	ctx := context.Background()
	// Large queue to not block
	jobQueue := make(chan Job, 1000)

	devices := make([]models.Device, 100)
	for i := 0; i < 100; i++ {
		devices[i] = models.Device{ID: i + 1, Name: "Device", PollingInterval: 10, LastPolledAt: nil}
	}

	mockDB := &MockDatabase{
		GetAllEnabledDevicesFunc: func(ctx context.Context) ([]models.Device, error) {
			return devices, nil
		},
	}

	s := NewScheduler(config.Config{}, mockDB, &MockLogger{}, jobQueue)
	s.syncDevices(ctx)

	var wg sync.WaitGroup
	// Concurrently run dispatch and markDone
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				s.dispatchDueDevices()
				time.Sleep(1 * time.Millisecond)
			}
		}()
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				// mark some random devices as done
				s.markDone((workerID * 10) + (j % 10) + 1)
				time.Sleep(1 * time.Millisecond)
			}
		}(i)
	}

	// Also simulate syncing devices concurrently
	wg.Add(1)
	go func() {
		defer wg.Done()
		for j := 0; j < 50; j++ {
			s.syncDevices(ctx)
			time.Sleep(2 * time.Millisecond)
		}
	}()

	wg.Wait()
	// If it doesn't panic or deadlock, it passes
}
