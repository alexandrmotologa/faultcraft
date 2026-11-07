package toxics

func init() {
	Register("hang", NewHangToxic)
}

// HangToxic simulates an unresponsive half-open socket by stalling data transmission indefinitely.
type HangToxic struct {
	BaseToxic
}

// NewHangToxic constructs a HangToxic.
func NewHangToxic(name string, dir Direction, toxicity float64, attrs map[string]interface{}) (Toxic, error) {
	return &HangToxic{
		BaseToxic: BaseToxic{
			ToxicName: name,
			ToxicType: "hang",
			ToxicDir:  dir,
			Prob:      toxicity,
			Attrs:     attrs,
		},
	}, nil
}

func (t *HangToxic) SetAttributes(attrs map[string]interface{}) error {
	t.Attrs = attrs
	return nil
}

func (t *HangToxic) Clone() Toxic {
	return &HangToxic{
		BaseToxic: BaseToxic{
			ToxicName: t.ToxicName,
			ToxicType: t.ToxicType,
			ToxicDir:  t.ToxicDir,
			Prob:      t.Prob,
			Attrs:     t.Attrs,
		},
	}
}

func (t *HangToxic) Pipe(link *Link, in <-chan *Chunk, out chan<- *Chunk) {
	defer close(out)

	if !t.ShouldApply() {
		for chunk := range in {
			if link.IsClosed() {
				return
			}
			out <- chunk
		}
		return
	}

	link.Stats.RecordFault()
	// Hang indefinitely until link is closed by context or peer
	<-link.Done()
}
