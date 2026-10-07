package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hafizkrm/Monitoring/backend/internal/contracts"
	"github.com/hafizkrm/Monitoring/backend/internal/transport/eventbus"
)

func TestM5_EventBusLoad(t *testing.T) {
	bus := eventbus.NewInMemoryEventBus()

	// Profiling setup
	runtime.SetBlockProfileRate(1)
	fCpu, _ := os.Create("cpu.prof")
	pprof.StartCPUProfile(fCpu)
	defer pprof.StopCPUProfile()

	fBlock, _ := os.Create("block.prof")
	defer func() {
		pprof.Lookup("block").WriteTo(fBlock, 0)
		fBlock.Close()
	}()

	// Parameters (Simulate 100+ devices bursts)
	numPublishers := 150
	eventsPerPublisher := 500
	numFastSubscribers := 5
	numSlowSubscribers := 1

	var publishedCount int64
	var deliveredFastCount int64
	var deliveredSlowCount int64

	// Create subscribers
	fastSubs := make([]*eventbus.Subscriber, numFastSubscribers)
	for i := 0; i < numFastSubscribers; i++ {
		fastSubs[i] = bus.Subscribe(contracts.DomainEventMetricsUpdated, 1000)
		go func(sub *eventbus.Subscriber) {
			for range sub.Channel {
				atomic.AddInt64(&deliveredFastCount, 1)
			}
		}(fastSubs[i])
	}

	slowSubs := make([]*eventbus.Subscriber, numSlowSubscribers)
	for i := 0; i < numSlowSubscribers; i++ {
		slowSubs[i] = bus.Subscribe(contracts.DomainEventMetricsUpdated, 10) // small buffer, triggers drop
		go func(sub *eventbus.Subscriber) {
			for range sub.Channel {
				time.Sleep(5 * time.Millisecond) // slow processing
				atomic.AddInt64(&deliveredSlowCount, 1)
			}
		}(slowSubs[i])
	}

	startMem := getMemStats()
	startGoroutines := runtime.NumGoroutine()
	start := time.Now()

	var wg sync.WaitGroup
	var pubLatencyTotal int64
	var pubLatencyCount int64

	// Start Publishers
	for p := 0; p < numPublishers; p++ {
		wg.Add(1)
		go func(pubID int) {
			defer wg.Done()
			for e := 0; e < eventsPerPublisher; e++ {
				evt := contracts.NewDomainEvent(
					"m5-test-worker",
					contracts.DomainEventMetricsUpdated,
					fmt.Sprintf("dev-%d", pubID),
					"test-payload",
				)

				pubStart := time.Now()
				bus.Publish(evt)
				latency := time.Since(pubStart).Microseconds()

				atomic.AddInt64(&pubLatencyTotal, latency)
				atomic.AddInt64(&pubLatencyCount, 1)
				atomic.AddInt64(&publishedCount, 1)

				// simulate slight varied polling interval
				if e%10 == 0 {
					time.Sleep(1 * time.Millisecond)
				}
			}
		}(p)
	}

	wg.Wait()
	publishDuration := time.Since(start)

	// Wait a bit for delivery to finish catching up
	time.Sleep(2 * time.Second)
	bus.Shutdown() // closes channels

	time.Sleep(500 * time.Millisecond) // give goroutines time to exit after shutdown

	totalExpectedPerSub := int64(numPublishers * eventsPerPublisher)
	fastDroppedMath := (totalExpectedPerSub * int64(numFastSubscribers)) - deliveredFastCount
	slowDroppedMath := (totalExpectedPerSub * int64(numSlowSubscribers)) - deliveredSlowCount

	// Verify atomic dropped counters
	var fastDroppedAtomic uint64
	for _, sub := range fastSubs {
		fastDroppedAtomic += sub.DroppedCount()
	}
	var slowDroppedAtomic uint64
	for _, sub := range slowSubs {
		slowDroppedAtomic += sub.DroppedCount()
	}

	if fastDroppedAtomic != uint64(fastDroppedMath) {
		t.Errorf("Fast dropped atomic mismatch: got %d, expected %d", fastDroppedAtomic, fastDroppedMath)
	}
	if slowDroppedAtomic != uint64(slowDroppedMath) {
		t.Errorf("Slow dropped atomic mismatch: got %d, expected %d", slowDroppedAtomic, slowDroppedMath)
	}

	endMem := getMemStats()
	endGoroutines := runtime.NumGoroutine()

	avgPubLatency := float64(pubLatencyTotal) / float64(pubLatencyCount) / 1000.0 // in ms

	fmt.Printf("\n--- M6 VERIFICATION (M5 LOAD TEST) ---\n")
	fmt.Printf("Publishers: %d\n", numPublishers)
	fmt.Printf("Events/Publisher: %d (Total: %d)\n", eventsPerPublisher, publishedCount)
	fmt.Printf("Test Duration (Publishing): %v\n", publishDuration)
	fmt.Printf("Throughput: %.2f events/sec\n", float64(publishedCount)/publishDuration.Seconds())
	fmt.Printf("Average Publish Latency: %.4f ms\n", avgPubLatency)

	fmt.Printf("Fast Subscribers: %d\n", numFastSubscribers)
	fmt.Printf("  Delivered: %d\n", deliveredFastCount)
	fmt.Printf("  Dropped (Atomic): %d\n", fastDroppedAtomic)

	fmt.Printf("Slow Subscribers: %d\n", numSlowSubscribers)
	fmt.Printf("  Delivered: %d\n", deliveredSlowCount)
	fmt.Printf("  Dropped (Atomic): %d\n", slowDroppedAtomic)

	fmt.Printf("Goroutines Start: %d | End: %d\n", startGoroutines, endGoroutines)
	fmt.Printf("Mem Alloc Start: %d KB | End: %d KB\n", startMem.Alloc/1024, endMem.Alloc/1024)
	fmt.Printf("---------------------------------------\n")
}

func getMemStats() runtime.MemStats {
	runtime.GC() // Force GC to get stable baseline
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return m
}
