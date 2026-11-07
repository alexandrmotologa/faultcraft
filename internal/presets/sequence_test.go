package presets

import (
	"net"
	"testing"
	"time"

	"github.com/alexandrmotologa/faultcraft/internal/proxy"
)

func TestSequenceRunner(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed starting listener: %v", err)
	}
	defer ln.Close()

	p := proxy.NewProxy("test_seq_proxy", "127.0.0.1:0", ln.Addr().String())
	if err := p.Start(); err != nil {
		t.Fatalf("failed starting proxy: %v", err)
	}
	defer p.Stop()

	runner := &SequenceRunner{}
	if err := runner.Run(p, "db_failover_drill"); err != nil {
		t.Fatalf("failed starting sequence: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	status := runner.GetStatus()
	if !status.Running {
		t.Errorf("expected sequence to be running")
	}
	if status.SequenceID != "db_failover_drill" {
		t.Errorf("expected sequence ID db_failover_drill, got %s", status.SequenceID)
	}

	runner.Stop(p)
	time.Sleep(20 * time.Millisecond)
	stoppedStatus := runner.GetStatus()
	if stoppedStatus.Running {
		t.Errorf("expected sequence to stop")
	}
}
