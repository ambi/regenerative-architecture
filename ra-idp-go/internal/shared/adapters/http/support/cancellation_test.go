package support

import (
	"context"
	"errors"
	"testing"
	"time"
)

type contextKey string

type abortMetricsSpy struct {
	aborts   []HTTPAbortKind
	detached int
}

func (m *abortMetricsSpy) IncHTTPAbort(kind HTTPAbortKind) {
	m.aborts = append(m.aborts, kind)
}

func (m *abortMetricsSpy) IncDetachedCompletionFailure() {
	m.detached++
}

func TestOperationContextKeepsValuesAndIgnoresRequestCancel(t *testing.T) {
	requestCtx, cancelRequest := context.WithCancel(context.WithValue(context.Background(), contextKey("tenant"), "acme"))
	deps := Deps{OperationTimeout: time.Minute}

	opCtx, cancelOp := deps.OperationContext(requestCtx)
	defer cancelOp()
	cancelRequest()

	if got := opCtx.Value(contextKey("tenant")); got != "acme" {
		t.Fatalf("operation context lost request value: %v", got)
	}
	select {
	case <-opCtx.Done():
		t.Fatalf("operation context was canceled by request abort: %v", opCtx.Err())
	default:
	}
}

func TestOperationContextAlwaysHasTimeout(t *testing.T) {
	opCtx, cancel := Deps{OperationTimeout: time.Millisecond}.OperationContext(context.Background())
	defer cancel()

	select {
	case <-opCtx.Done():
		if !errors.Is(opCtx.Err(), context.DeadlineExceeded) {
			t.Fatalf("operation context err = %v, want deadline exceeded", opCtx.Err())
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("operation context did not enforce timeout")
	}
}

func TestClassifyCancel(t *testing.T) {
	clientCtx, cancelClient := context.WithCancel(context.Background())
	cancelClient()
	if got, ok := ClassifyCancel(clientCtx, context.Canceled); !ok || got != HTTPAbortClientAborted {
		t.Fatalf("client cancel classified as (%q, %v)", got, ok)
	}

	serverCtx, cancelServer := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancelServer()
	<-serverCtx.Done()
	if got, ok := ClassifyCancel(serverCtx, context.DeadlineExceeded); !ok || got != HTTPAbortServerTimeout {
		t.Fatalf("server deadline classified as (%q, %v)", got, ok)
	}

	if got, ok := ClassifyCancel(context.Background(), context.DeadlineExceeded); !ok || got != HTTPAbortUpstreamTimeout {
		t.Fatalf("upstream deadline classified as (%q, %v)", got, ok)
	}
}

func TestClassifyAndRecordCancelRecordsMetrics(t *testing.T) {
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()
	metrics := &abortMetricsSpy{}
	deps := Deps{AbortMetrics: metrics}

	if got, ok := deps.ClassifyAndRecordCancel(requestCtx, context.Canceled); !ok || got != HTTPAbortClientAborted {
		t.Fatalf("recorded cancel classified as (%q, %v)", got, ok)
	}
	if len(metrics.aborts) != 1 || metrics.aborts[0] != HTTPAbortClientAborted {
		t.Fatalf("abort metrics = %+v", metrics.aborts)
	}

	deps.RecordDetachedCompletionFailure()
	if metrics.detached != 1 {
		t.Fatalf("detached completion failures = %d", metrics.detached)
	}
}
