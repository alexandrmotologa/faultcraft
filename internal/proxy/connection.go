package proxy

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/alexandrmotologa/faultcraft/internal/telemetry"
	"github.com/alexandrmotologa/faultcraft/internal/toxics"
)

// PipeConnection handles bidirectional data streaming between client and upstream,
// executing any active toxics along each directional stream.
func PipeConnection(
	clientConn net.Conn,
	upstreamAddr string,
	stats *telemetry.Stats,
	upstreamToxics []toxics.Toxic,
	downstreamToxics []toxics.Toxic,
	dialTimeout time.Duration,
) error {
	if dialTimeout <= 0 {
		dialTimeout = 5 * time.Second
	}

	upstreamConn, err := net.DialTimeout("tcp", upstreamAddr, dialTimeout)
	if err != nil {
		stats.RecordError()
		_ = clientConn.Close()
		return err
	}

	stats.ConnOpened()
	defer stats.ConnClosed()

	upstreamLink := toxics.NewLink(clientConn, upstreamConn, toxics.Upstream, stats)
	downstreamLink := toxics.NewLink(upstreamConn, clientConn, toxics.Downstream, stats)

	defer upstreamLink.Close()
	defer downstreamLink.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	// Stream 1: Client -> Upstream Target
	go func() {
		defer wg.Done()
		defer downstreamLink.Close() // Cascade close to the opposing stream
		runStreamPipeline(upstreamLink, upstreamToxics)
	}()

	// Stream 2: Upstream Target -> Client
	go func() {
		defer wg.Done()
		defer upstreamLink.Close() // Cascade close to the opposing stream
		runStreamPipeline(downstreamLink, downstreamToxics)
	}()

	wg.Wait()
	return nil
}

// runStreamPipeline initializes the reader, intermediate toxic chain, and destination writer.
func runStreamPipeline(link *toxics.Link, chain []toxics.Toxic) {
	inChan := make(chan *toxics.Chunk, 64)

	// Start reader goroutine from source
	go readLoop(link, inChan)

	// Thread data through toxic pipeline
	currentChan := inChan
	for _, t := range chain {
		if t == nil {
			continue
		}
		// Clone toxic so state is isolated per connection
		toxicInstance := t.Clone()
		nextChan := make(chan *toxics.Chunk, 64)
		go toxicInstance.Pipe(link, currentChan, nextChan)
		currentChan = nextChan
	}

	// Write processed chunks to target
	writeLoop(link, currentChan)
}

// readLoop reads raw byte streams from source socket and enqueues chunks.
func readLoop(link *toxics.Link, out chan<- *toxics.Chunk) {
	defer close(out)

	bufPtr := toxics.BufferPool.Get().(*[]byte)
	defer toxics.BufferPool.Put(bufPtr)
	buf := *bufPtr

	for {
		if link.IsClosed() {
			return
		}

		n, err := link.Source.Read(buf)
		if n > 0 {
			// Allocate exact slice for the chunk
			chunkData := make([]byte, n)
			copy(chunkData, buf[:n])

			select {
			case <-link.Done():
				return
			case out <- &toxics.Chunk{
				Data:      chunkData,
				Timestamp: time.Now(),
			}:
			}
		}

		if err != nil {
			if err != io.EOF && !link.IsClosed() {
				link.Stats.RecordError()
			}
			return
		}
	}
}

// writeLoop consumes chunks from the end of the toxic pipeline and writes to target.
func writeLoop(link *toxics.Link, in <-chan *toxics.Chunk) {
	for chunk := range in {
		if link.IsClosed() {
			return
		}

		total := len(chunk.Data)
		written := 0

		for written < total {
			n, err := link.Target.Write(chunk.Data[written:])
			if n > 0 {
				written += n
				if link.Dir == toxics.Upstream {
					link.Stats.AddBytesUpstream(int64(n))
				} else {
					link.Stats.AddBytesDownstream(int64(n))
				}
			}
			if err != nil {
				if !link.IsClosed() {
					link.Stats.RecordError()
				}
				link.Close()
				return
			}
		}

		latency := time.Since(chunk.Timestamp)
		link.Stats.RecordLatency(latency)
	}
}
