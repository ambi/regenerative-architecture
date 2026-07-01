package support

import (
	"context"
	"errors"
	"time"
)

const (
	defaultOperationTimeout          = 5 * time.Second
	defaultDetachedCompletionTimeout = 2 * time.Second
)

// HTTPAbortKind is the metric label used to keep client aborts out of server
// error accounting while still making cancellation visible.
type HTTPAbortKind string

const (
	HTTPAbortClientAborted   HTTPAbortKind = "client_aborted"
	HTTPAbortServerTimeout   HTTPAbortKind = "server_timeout"
	HTTPAbortUpstreamTimeout HTTPAbortKind = "upstream_timeout"
)

type HTTPAbortMetrics interface {
	IncHTTPAbort(kind HTTPAbortKind)
	IncDetachedCompletionFailure()
}

func (d Deps) OperationContext(requestCtx context.Context) (context.Context, context.CancelFunc) {
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	timeout := d.OperationTimeout
	if timeout <= 0 {
		timeout = defaultOperationTimeout
	}
	return context.WithTimeout(context.WithoutCancel(requestCtx), timeout)
}

func (d Deps) DetachedCompletionContext(requestCtx context.Context) (context.Context, context.CancelFunc) {
	if requestCtx == nil {
		requestCtx = context.Background()
	}
	timeout := d.DetachedCompletionTimeout
	if timeout <= 0 {
		timeout = defaultDetachedCompletionTimeout
	}
	return context.WithTimeout(context.WithoutCancel(requestCtx), timeout)
}

func (d Deps) RecordDetachedCompletionFailure() {
	if d.AbortMetrics != nil {
		d.AbortMetrics.IncDetachedCompletionFailure()
	}
}

func (d Deps) ClassifyAndRecordCancel(requestCtx context.Context, err error) (HTTPAbortKind, bool) {
	kind, ok := ClassifyCancel(requestCtx, err)
	if ok && d.AbortMetrics != nil {
		d.AbortMetrics.IncHTTPAbort(kind)
	}
	return kind, ok
}

func ClassifyCancel(requestCtx context.Context, err error) (HTTPAbortKind, bool) {
	if err == nil {
		return "", false
	}
	if errors.Is(err, context.Canceled) {
		if requestCtx != nil && errors.Is(requestCtx.Err(), context.Canceled) {
			return HTTPAbortClientAborted, true
		}
		return HTTPAbortUpstreamTimeout, true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		if requestCtx != nil && errors.Is(requestCtx.Err(), context.DeadlineExceeded) {
			return HTTPAbortServerTimeout, true
		}
		return HTTPAbortUpstreamTimeout, true
	}
	return "", false
}
