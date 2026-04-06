package lineage

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// DataFlow represents a data movement between services.
type DataFlow struct {
	ID          string    `json:"id"`
	DataSubject string    `json:"data_subject"`
	DataType    string    `json:"data_type"`
	Source      string    `json:"source"`
	Destination string    `json:"destination"`
	Purpose     string    `json:"purpose"`
	LegalBasis  string    `json:"legal_basis"`
	ConsentID   string    `json:"consent_id,omitempty"`
	Timestamp   time.Time `json:"timestamp"`
}

// DataFlowSummary aggregates flows between a service pair.
type DataFlowSummary struct {
	Source      string   `json:"source"`
	Destination string   `json:"destination"`
	DataTypes   []string `json:"data_types"`
	FlowCount   int      `json:"flow_count"`
}

// Tracker records and queries data lineage.
type Tracker struct {
	flows []DataFlow
	mu    sync.RWMutex
	seq   int
}

// New creates a new Tracker.
func New() *Tracker {
	return &Tracker{
		flows: make([]DataFlow, 0),
	}
}

// Record records a data flow event and returns the assigned flow ID.
func (t *Tracker) Record(flow DataFlow) string {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.seq++
	flow.ID = fmt.Sprintf("flow-%d", t.seq)
	if flow.Timestamp.IsZero() {
		flow.Timestamp = time.Now()
	}

	t.flows = append(t.flows, flow)
	return flow.ID
}

// GetBySubject returns all data flows for a data subject (user).
func (t *Tracker) GetBySubject(subjectID string) []DataFlow {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []DataFlow
	for _, f := range t.flows {
		if f.DataSubject == subjectID {
			result = append(result, f)
		}
	}
	return result
}

// GetByDataType returns all flows of a specific data type.
func (t *Tracker) GetByDataType(dataType string) []DataFlow {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []DataFlow
	for _, f := range t.flows {
		if f.DataType == dataType {
			result = append(result, f)
		}
	}
	return result
}

// GetByService returns all flows involving a service (as source or destination).
func (t *Tracker) GetByService(serviceName string) []DataFlow {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []DataFlow
	for _, f := range t.flows {
		if f.Source == serviceName || f.Destination == serviceName {
			result = append(result, f)
		}
	}
	return result
}

// Summary returns a data map: which data types flow between which services.
func (t *Tracker) Summary() []DataFlowSummary {
	t.mu.RLock()
	defer t.mu.RUnlock()

	type pairKey struct {
		Source      string
		Destination string
	}

	pairData := make(map[pairKey]map[string]bool)
	pairCount := make(map[pairKey]int)

	for _, f := range t.flows {
		key := pairKey{Source: f.Source, Destination: f.Destination}
		if pairData[key] == nil {
			pairData[key] = make(map[string]bool)
		}
		pairData[key][f.DataType] = true
		pairCount[key]++
	}

	var summaries []DataFlowSummary
	for key, types := range pairData {
		var dataTypes []string
		for dt := range types {
			dataTypes = append(dataTypes, dt)
		}
		sort.Strings(dataTypes)

		summaries = append(summaries, DataFlowSummary{
			Source:      key.Source,
			Destination: key.Destination,
			DataTypes:   dataTypes,
			FlowCount:   pairCount[key],
		})
	}

	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].Source != summaries[j].Source {
			return summaries[i].Source < summaries[j].Source
		}
		return summaries[i].Destination < summaries[j].Destination
	})

	return summaries
}
