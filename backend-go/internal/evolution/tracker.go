package evolution

// Tracker records execution traces associated with prompts.
type Tracker struct {
	Store *Store
}

// NewTracker creates a new execution tracker.
func NewTracker(store *Store) *Tracker {
	return &Tracker{Store: store}
}

// RecordTrace records an execution trace.
func (t *Tracker) RecordTrace(trace *ExecutionTrace) (int64, error) {
	return t.Store.RecordTrace(trace)
}

// GetRecentTraces returns the most recent N traces.
func (t *Tracker) GetRecentTraces(limit int) ([]*ExecutionTrace, error) {
	return t.Store.GetRecentTraces(limit)
}

// GetUnanalyzedTraces returns traces that haven't been analyzed yet.
func (t *Tracker) GetUnanalyzedTraces(limit int) ([]*ExecutionTrace, error) {
	return t.Store.GetUnanalyzedTraces(limit)
}

// MarkAnalyzed marks a batch of traces as analyzed.
func (t *Tracker) MarkAnalyzed(ids []int64) error {
	return t.Store.MarkTracesAnalyzed(ids)
}
