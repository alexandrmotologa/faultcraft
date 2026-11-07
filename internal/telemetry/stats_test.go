package telemetry

import (
	"testing"
	"time"
)

func TestStatsSnapshot(t *testing.T) {
	s := NewStats()

	s.ConnOpened()
	s.ConnOpened()
	s.ConnClosed()

	s.AddBytesUpstream(1024)
	s.AddBytesDownstream(2048)
	s.RecordError()
	s.RecordFault()

	s.RecordLatency(10 * time.Millisecond)
	s.RecordLatency(20 * time.Millisecond)
	s.RecordLatency(50 * time.Millisecond)
	s.RecordLatency(100 * time.Millisecond)

	snap := s.Snapshot()

	if snap.ActiveConnections != 1 {
		t.Errorf("expected 1 active connection, got %d", snap.ActiveConnections)
	}
	if snap.TotalConnections != 2 {
		t.Errorf("expected 2 total connections, got %d", snap.TotalConnections)
	}
	if snap.BytesUpstream != 1024 {
		t.Errorf("expected 1024 bytes upstream, got %d", snap.BytesUpstream)
	}
	if snap.BytesDownstream != 2048 {
		t.Errorf("expected 2048 bytes downstream, got %d", snap.BytesDownstream)
	}
	if snap.ErrorCount != 1 {
		t.Errorf("expected 1 error count, got %d", snap.ErrorCount)
	}
	if snap.FaultCount != 1 {
		t.Errorf("expected 1 fault count, got %d", snap.FaultCount)
	}
	if snap.P50LatencyMs < 20 || snap.P50LatencyMs > 50 {
		t.Errorf("expected p50 around 20-50ms, got %f", snap.P50LatencyMs)
	}

	s.Reset()
	snap2 := s.Snapshot()
	if snap2.ActiveConnections != 0 || snap2.BytesUpstream != 0 {
		t.Errorf("expected reset stats to be zeroed, got %+v", snap2)
	}
}
