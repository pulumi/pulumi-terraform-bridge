package tfbridge

import (
	"context"
	"runtime"
	"sync"

	"github.com/opentracing/opentracing-go"
)

type memStatCollector struct {
	maxStats runtime.MemStats
	mu       sync.Mutex
}

// Samples memory stats in the background at 1s intervals, and creates
// spans for the data. This is currently opt-in via
// `PULUMI_TRACING_MEMSTATS_POLL_INTERVAL=1s` or similar. Consider
// collecting this by default later whenever tracing is enabled as we
// calibrate that the overhead is low enough.
func (c *memStatCollector) collectMemStats(ctx context.Context, span opentracing.Span) {
	memStats := runtime.MemStats{}

	runtime.ReadMemStats(&memStats)

	c.mu.Lock()
	defer c.mu.Unlock()

	// report cumulative metrics as is
	span.SetTag("runtime.NumCgoCall", runtime.NumCgoCall())
	span.SetTag("MemStats.TotalAlloc", memStats.TotalAlloc)
	span.SetTag("MemStats.Mallocs", memStats.Mallocs)
	span.SetTag("MemStats.Frees", memStats.Frees)
	span.SetTag("MemStats.PauseTotalNs", memStats.PauseTotalNs)
	span.SetTag("MemStats.NumGC", memStats.NumGC)

	// for other metrics report the max alongside current

	if memStats.Sys > c.maxStats.Sys {
		c.maxStats.Sys = memStats.Sys
		span.SetTag("MemStats.Sys", memStats.Sys)
		span.SetTag("MemStats.Sys.Max", c.maxStats.Sys)
	}

	if memStats.HeapAlloc > c.maxStats.HeapAlloc {
		c.maxStats.HeapAlloc = memStats.HeapAlloc
		span.SetTag("MemStats.HeapAlloc", memStats.HeapAlloc)
		span.SetTag("MemStats.HeapAlloc.Max", c.maxStats.HeapAlloc)
	}

	if memStats.HeapSys > c.maxStats.HeapSys {
		c.maxStats.HeapSys = memStats.HeapSys
		span.SetTag("MemStats.HeapSys", memStats.HeapSys)
		span.SetTag("MemStats.HeapSys.Max", c.maxStats.HeapSys)
	}

	if memStats.HeapIdle > c.maxStats.HeapIdle {
		c.maxStats.HeapIdle = memStats.HeapIdle
		span.SetTag("MemStats.HeapIdle", memStats.HeapIdle)
		span.SetTag("MemStats.HeapIdle.Max", c.maxStats.HeapIdle)
	}

	if memStats.HeapInuse > c.maxStats.HeapInuse {
		c.maxStats.HeapInuse = memStats.HeapInuse
		span.SetTag("MemStats.HeapInuse", memStats.HeapInuse)
		span.SetTag("MemStats.HeapInuse.Max", c.maxStats.HeapInuse)
	}

	if memStats.HeapReleased > c.maxStats.HeapReleased {
		c.maxStats.HeapReleased = memStats.HeapReleased
		span.SetTag("MemStats.HeapReleased", memStats.HeapReleased)
		span.SetTag("MemStats.HeapReleased.Max", c.maxStats.HeapReleased)
	}

	if memStats.HeapObjects > c.maxStats.HeapObjects {
		c.maxStats.HeapObjects = memStats.HeapObjects
		span.SetTag("MemStats.HeapObjects", memStats.HeapObjects)
		span.SetTag("MemStats.HeapObjects.Max", c.maxStats.HeapObjects)
	}

	if memStats.StackInuse > c.maxStats.StackInuse {
		c.maxStats.StackInuse = memStats.StackInuse
		span.SetTag("MemStats.StackInuse", memStats.StackInuse)
		span.SetTag("MemStats.StackInuse.Max", c.maxStats.StackInuse)
	}

	if memStats.StackSys > c.maxStats.StackSys {
		c.maxStats.StackSys = memStats.StackSys
		span.SetTag("MemStats.StackSys", memStats.StackSys)
		span.SetTag("MemStats.StackSys.Max", c.maxStats.StackSys)
	}
}
