package video

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

const (
	DefaultMaxConcurrent = 2
	RenderTimeout        = 5 * time.Second
)

type RenderRequest struct {
	Position time.Duration
	Width    int
	Height   int
	Quality  QualityPreset
	SeqNum   uint64
}

type RenderResult struct {
	Frame string
	Error error
}

type RenderWorker struct {
	maxConcurrent int
	semaphore     chan struct{}
	latestSeq     uint64
	mu            sync.Mutex
	cancelFunc    context.CancelFunc
	cancelMu      sync.Mutex
}

func NewRenderWorker(maxConcurrent int) *RenderWorker {
	if maxConcurrent <= 0 {
		maxConcurrent = DefaultMaxConcurrent
	}
	return &RenderWorker{
		maxConcurrent: maxConcurrent,
		semaphore:     make(chan struct{}, maxConcurrent),
	}
}

// NextSeq returns the next sequence number for tracking request order
func (w *RenderWorker) NextSeq() uint64 {
	return atomic.AddUint64(&w.latestSeq, 1)
}

// IsStale checks if a request is outdated (newer request has been submitted)
func (w *RenderWorker) IsStale(seqNum uint64) bool {
	return seqNum < atomic.LoadUint64(&w.latestSeq)
}

// Submit submits a render job with cancellation support for stale requests
// The callback is called with the result only if the request is not stale
func (w *RenderWorker) Submit(req RenderRequest, renderFunc func(context.Context) (string, error), callback func(RenderResult)) {
	// Cancel any previous pending render
	w.cancelMu.Lock()
	if w.cancelFunc != nil {
		w.cancelFunc()
	}
	ctx, cancel := context.WithTimeout(context.Background(), RenderTimeout)
	w.cancelFunc = cancel
	w.cancelMu.Unlock()

	go func() {
		defer cancel()

		// Try to acquire semaphore
		select {
		case w.semaphore <- struct{}{}:
			defer func() { <-w.semaphore }()
		case <-ctx.Done():
			return
		}

		// Check if stale before starting work
		if w.IsStale(req.SeqNum) {
			return
		}

		// Execute the render function
		frame, err := renderFunc(ctx)

		// Check if stale after completing work
		if w.IsStale(req.SeqNum) {
			return
		}

		// Only callback if not stale and context not cancelled
		select {
		case <-ctx.Done():
			return
		default:
			callback(RenderResult{Frame: frame, Error: err})
		}
	}()
}

// SubmitSync submits a render job and waits for the result
func (w *RenderWorker) SubmitSync(req RenderRequest, renderFunc func(context.Context) (string, error)) RenderResult {
	resultChan := make(chan RenderResult, 1)

	w.Submit(req, renderFunc, func(result RenderResult) {
		select {
		case resultChan <- result:
		default:
		}
	})

	select {
	case result := <-resultChan:
		return result
	case <-time.After(RenderTimeout):
		return RenderResult{Error: context.DeadlineExceeded}
	}
}

// ActiveCount returns the number of currently active workers
func (w *RenderWorker) ActiveCount() int {
	return len(w.semaphore)
}

// CancelAll cancels any pending render operations
func (w *RenderWorker) CancelAll() {
	w.cancelMu.Lock()
	if w.cancelFunc != nil {
		w.cancelFunc()
		w.cancelFunc = nil
	}
	w.cancelMu.Unlock()
}
