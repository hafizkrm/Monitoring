package worker

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/logger"
	"github.com/hafizkrm/Monitoring/backend/internal/models"
)

type persistTask struct {
	deviceMetric *models.DeviceMetric
	ifaceMetrics []*models.InterfaceMetric
}

type AsyncMetricsRepo struct {
	repo     MetricsRepository
	queue    chan persistTask
	wg       sync.WaitGroup
	workers  int
	logger   logger.Logger
	isClosed bool
	mu       sync.RWMutex
}

func NewAsyncMetricsRepo(repo MetricsRepository, log logger.Logger, capacity int, workers int) *AsyncMetricsRepo {
	r := &AsyncMetricsRepo{
		repo:    repo,
		queue:   make(chan persistTask, capacity),
		workers: workers,
		logger:  log,
	}
	r.start()
	return r
}

func (r *AsyncMetricsRepo) start() {
	r.wg.Add(r.workers)
	for i := 0; i < r.workers; i++ {
		go func() {
			defer r.wg.Done()

			const batchSize = 100
			deviceBatch := make([]*models.DeviceMetric, 0, batchSize)

			flushDeviceMetrics := func() {
				if len(deviceBatch) == 0 {
					return
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				if err := r.repo.BatchInsertDeviceMetrics(ctx, deviceBatch); err != nil {
					r.logger.Error("Async BatchInsertDeviceMetrics failed", map[string]interface{}{"error": err, "count": len(deviceBatch)})
				}
				cancel()
				deviceBatch = deviceBatch[:0]
			}

			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()

			for {
				select {
				case task, ok := <-r.queue:
					if !ok {
						// Channel closed (shutdown)
						flushDeviceMetrics()
						return
					}

					if task.deviceMetric != nil {
						deviceBatch = append(deviceBatch, task.deviceMetric)
					}

					// interface_metrics are already accumulated per-device in the worker hot-path,
					// so we can insert them as their own batch immediately without cross-device batching.
					if len(task.ifaceMetrics) > 0 {
						ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
						if err := r.repo.BatchInsertInterfaceMetrics(ctx, task.ifaceMetrics); err != nil {
							r.logger.Error("Async BatchInsertInterfaceMetrics failed", map[string]interface{}{"error": err})
						}
						cancel()
					}

					if len(deviceBatch) >= batchSize {
						flushDeviceMetrics()
					}

				case <-ticker.C:
					flushDeviceMetrics()
				}
			}
		}()
	}
}

func (r *AsyncMetricsRepo) Close() {
	r.mu.Lock()
	if r.isClosed {
		r.mu.Unlock()
		return
	}
	r.isClosed = true
	r.mu.Unlock()
	close(r.queue)
	r.wg.Wait()
}

func (r *AsyncMetricsRepo) InsertDeviceMetric(ctx context.Context, metric *models.DeviceMetric) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.isClosed {
		return errors.New("async metric repo is closed")
	}

	select {
	case r.queue <- persistTask{deviceMetric: metric}:
		return nil
	default:
		r.logger.Warn("SQL persistence queue full, dropping device metric", map[string]interface{}{"device_id": metric.DeviceID})
		return nil
	}
}

func (r *AsyncMetricsRepo) BatchInsertDeviceMetrics(ctx context.Context, metrics []*models.DeviceMetric) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.isClosed {
		return errors.New("async metric repo is closed")
	}

	// For simplicity in the async queue, we just enqueue them one by one,
	// they will be batched again inside the worker.
	for _, m := range metrics {
		select {
		case r.queue <- persistTask{deviceMetric: m}:
		default:
			r.logger.Warn("SQL persistence queue full, dropping device metric in batch", map[string]interface{}{"device_id": m.DeviceID})
		}
	}
	return nil
}

func (r *AsyncMetricsRepo) BatchInsertInterfaceMetrics(ctx context.Context, metrics []*models.InterfaceMetric) error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.isClosed {
		return errors.New("async metric repo is closed")
	}

	select {
	case r.queue <- persistTask{ifaceMetrics: metrics}:
		return nil
	default:
		r.logger.Warn("SQL persistence queue full, dropping interface metrics", nil)
		return nil
	}
}
