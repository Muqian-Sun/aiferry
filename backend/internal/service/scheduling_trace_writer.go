package service

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

// 调度轨迹写入的参数。队列长度与批量是拍的：按现在每天几百个请求留足余量，量大了看丢弃计数再调。
const (
	schedulingTraceQueueSize     = 2000
	schedulingTraceBatchSize     = 200
	schedulingTraceFlushInterval = time.Second
	// SchedulingTraceRetention 轨迹保留期（muqian 2026-10-18 定 7 天）。
	SchedulingTraceRetention     = 7 * 24 * time.Hour
	schedulingTracePurgeInterval = time.Hour
	schedulingTraceDropLogEvery  = time.Minute
)

// SchedulingTraceFilter 管理站查轨迹的条件；Outcome 额外接受 switched（换过渠道）。
type SchedulingTraceFilter struct {
	Since           time.Time
	CatalogEntryID  int64
	InboundProtocol string
	Outcome         string
	AccountID       int64
	Limit           int
	Offset          int
}

// SchedulingTraceCount 按条目 × 入站协议 × 最终渠道 × 结果的请求数；没选到渠道时 AccountID 为 0。
type SchedulingTraceCount struct {
	CatalogEntryID  int64
	InboundProtocol string
	AccountID       int64
	Outcome         string
	Requests        int64
}

// SchedulingTraceRepository 调度轨迹的存储。
type SchedulingTraceRepository interface {
	InsertSchedulingTraces(ctx context.Context, traces []SchedulingTrace) error
	DeleteSchedulingTracesBefore(ctx context.Context, cutoff time.Time) (int64, error)
	ListSchedulingTraces(ctx context.Context, filter SchedulingTraceFilter) ([]SchedulingTrace, int64, error)
	CountSchedulingTraces(ctx context.Context, since time.Time) ([]SchedulingTraceCount, error)
}

// SchedulingTraceWriter 把请求结束时交来的轨迹异步批量写库，并定期清理过期的。
// 队列满时丢弃并计数、按分钟打 warn；写库失败只打日志，不影响请求。
type SchedulingTraceWriter struct {
	repo    SchedulingTraceRepository
	queue   chan SchedulingTrace
	stop    chan struct{}
	done    chan struct{}
	once    sync.Once
	dropped atomic.Int64
	lastLog atomic.Int64
}

// NewSchedulingTraceWriter 创建并启动写入协程。
func NewSchedulingTraceWriter(repo SchedulingTraceRepository) *SchedulingTraceWriter {
	w := &SchedulingTraceWriter{
		repo:  repo,
		queue: make(chan SchedulingTrace, schedulingTraceQueueSize),
		stop:  make(chan struct{}),
		done:  make(chan struct{}),
	}
	go w.run()
	return w
}

// Enqueue 交一条轨迹，不阻塞请求。
func (w *SchedulingTraceWriter) Enqueue(trace SchedulingTrace) {
	select {
	case w.queue <- trace:
	default:
		total := w.dropped.Add(1)
		now := time.Now().UnixNano()
		last := w.lastLog.Load()
		if now-last >= int64(schedulingTraceDropLogEvery) && w.lastLog.CompareAndSwap(last, now) {
			slog.Warn("scheduling_trace.queue_full_dropped", "dropped_total", total, "queue_size", schedulingTraceQueueSize)
		}
	}
}

// Dropped 累计丢弃的轨迹数。
func (w *SchedulingTraceWriter) Dropped() int64 { return w.dropped.Load() }

// Stop 停止写入：先把队列里剩下的写完。
func (w *SchedulingTraceWriter) Stop() {
	w.once.Do(func() { close(w.stop) })
	<-w.done
}

func (w *SchedulingTraceWriter) run() {
	defer close(w.done)
	flush := time.NewTicker(schedulingTraceFlushInterval)
	defer flush.Stop()
	purge := time.NewTicker(schedulingTracePurgeInterval)
	defer purge.Stop()
	w.purge()

	batch := make([]SchedulingTrace, 0, schedulingTraceBatchSize)
	write := func() {
		if len(batch) == 0 {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		if err := w.repo.InsertSchedulingTraces(ctx, batch); err != nil {
			slog.Warn("scheduling_trace.insert_failed", "count", len(batch), "error", err)
		}
		cancel()
		batch = batch[:0]
	}
	for {
		select {
		case trace := <-w.queue:
			batch = append(batch, trace)
			if len(batch) >= schedulingTraceBatchSize {
				write()
			}
		case <-flush.C:
			write()
		case <-purge.C:
			w.purge()
		case <-w.stop:
			for {
				select {
				case trace := <-w.queue:
					batch = append(batch, trace)
					if len(batch) >= schedulingTraceBatchSize {
						write()
					}
				default:
					write()
					return
				}
			}
		}
	}
}

func (w *SchedulingTraceWriter) purge() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := w.repo.DeleteSchedulingTracesBefore(ctx, time.Now().Add(-SchedulingTraceRetention)); err != nil {
		slog.Warn("scheduling_trace.purge_failed", "error", err)
	}
}
