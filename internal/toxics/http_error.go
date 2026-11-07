package toxics

import (
	"bytes"
	"fmt"
	"net/http"
	"sync"
)

func init() {
	Register("http_error", NewHTTPErrorToxic)
}

// HTTPErrorToxic rewrites HTTP/1.1 responses to synthetic error codes (500, 502, 503, 504).
type HTTPErrorToxic struct {
	BaseToxic

	mu     sync.Mutex
	Status int    `json:"status"` // Target HTTP status code
	Body   string `json:"body"`   // Custom response body (optional)
}

// NewHTTPErrorToxic constructs an HTTPErrorToxic.
func NewHTTPErrorToxic(name string, dir Direction, toxicity float64, attrs map[string]interface{}) (Toxic, error) {
	t := &HTTPErrorToxic{
		BaseToxic: BaseToxic{
			ToxicName: name,
			ToxicType: "http_error",
			ToxicDir:  dir,
			Prob:      toxicity,
			Attrs:     attrs,
		},
		Status: 503,
	}

	if err := t.SetAttributes(attrs); err != nil {
		return nil, err
	}
	return t, nil
}

func (t *HTTPErrorToxic) SetAttributes(attrs map[string]interface{}) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.Attrs = attrs
	if status, ok := attrs["status"]; ok {
		t.Status = int(toInt64(status))
	}
	if body, ok := attrs["body"]; ok {
		if s, ok := body.(string); ok {
			t.Body = s
		}
	}
	if t.Status < 400 || t.Status > 599 {
		t.Status = 503
	}
	return nil
}

func (t *HTTPErrorToxic) Clone() Toxic {
	t.mu.Lock()
	defer t.mu.Unlock()

	return &HTTPErrorToxic{
		BaseToxic: BaseToxic{
			ToxicName: t.ToxicName,
			ToxicType: t.ToxicType,
			ToxicDir:  t.ToxicDir,
			Prob:      t.Prob,
			Attrs:     t.Attrs,
		},
		Status: t.Status,
		Body:   t.Body,
	}
}

func (t *HTTPErrorToxic) Pipe(link *Link, in <-chan *Chunk, out chan<- *Chunk) {
	defer close(out)

	for chunk := range in {
		if link.IsClosed() {
			return
		}

		if t.ShouldApply() && isHTTPResponse(chunk.Data) {
			t.mu.Lock()
			statusCode := t.Status
			customBody := t.Body
			t.mu.Unlock()

			link.Stats.RecordFault()

			statusText := http.StatusText(statusCode)
			if statusText == "" {
				statusText = "Error"
			}

			payload := customBody
			if payload == "" {
				payload = fmt.Sprintf("{\"error\":\"faultcraft_injected_fault\",\"status\":%d,\"message\":\"%s\"}\n", statusCode, statusText)
			}

			responseStr := fmt.Sprintf(
				"HTTP/1.1 %d %s\r\nContent-Type: application/json\r\nContent-Length: %d\r\nConnection: close\r\n\r\n%s",
				statusCode,
				statusText,
				len(payload),
				payload,
			)

			subChunk := &Chunk{
				Data:      []byte(responseStr),
				Timestamp: chunk.Timestamp,
			}

			select {
			case <-link.Done():
				return
			case out <- subChunk:
			}
			continue
		}

		select {
		case <-link.Done():
			return
		case out <- chunk:
		}
	}
}

func isHTTPResponse(data []byte) bool {
	return bytes.HasPrefix(data, []byte("HTTP/1.0 ")) || bytes.HasPrefix(data, []byte("HTTP/1.1 "))
}
