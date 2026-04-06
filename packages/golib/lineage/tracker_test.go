package lineage

import (
	"sync"
	"testing"
	"time"
)

func TestRecordAndGetBySubject(t *testing.T) {
	tr := New()

	tr.Record(DataFlow{
		DataSubject: "user-1",
		DataType:    "nik",
		Source:      "garudainfo",
		Destination: "dukcapil",
		Purpose:     "identity verification",
		LegalBasis:  "consent",
		ConsentID:   "c-1",
	})
	tr.Record(DataFlow{
		DataSubject: "user-2",
		DataType:    "name",
		Source:      "garudainfo",
		Destination: "bff",
		Purpose:     "display",
		LegalBasis:  "consent",
	})

	flows := tr.GetBySubject("user-1")
	if len(flows) != 1 {
		t.Fatalf("expected 1 flow for user-1, got %d", len(flows))
	}
	if flows[0].DataType != "nik" {
		t.Errorf("expected data_type nik, got %s", flows[0].DataType)
	}
	if flows[0].ID == "" {
		t.Error("expected flow ID to be assigned")
	}
}

func TestGetByDataType(t *testing.T) {
	tr := New()

	tr.Record(DataFlow{
		DataSubject: "user-1", DataType: "nik",
		Source: "garudainfo", Destination: "dukcapil",
	})
	tr.Record(DataFlow{
		DataSubject: "user-2", DataType: "nik",
		Source: "garudainfo", Destination: "dukcapil",
	})
	tr.Record(DataFlow{
		DataSubject: "user-1", DataType: "name",
		Source: "garudainfo", Destination: "bff",
	})

	nikFlows := tr.GetByDataType("nik")
	if len(nikFlows) != 2 {
		t.Errorf("expected 2 nik flows, got %d", len(nikFlows))
	}

	nameFlows := tr.GetByDataType("name")
	if len(nameFlows) != 1 {
		t.Errorf("expected 1 name flow, got %d", len(nameFlows))
	}

	dobFlows := tr.GetByDataType("dob")
	if len(dobFlows) != 0 {
		t.Errorf("expected 0 dob flows, got %d", len(dobFlows))
	}
}

func TestGetByService(t *testing.T) {
	tr := New()

	tr.Record(DataFlow{
		DataSubject: "user-1", DataType: "nik",
		Source: "garudainfo", Destination: "dukcapil",
	})
	tr.Record(DataFlow{
		DataSubject: "user-1", DataType: "name",
		Source: "dukcapil", Destination: "garudainfo",
	})
	tr.Record(DataFlow{
		DataSubject: "user-2", DataType: "address",
		Source: "bff", Destination: "garudainfo",
	})

	// dukcapil appears as both source and destination
	dukcapilFlows := tr.GetByService("dukcapil")
	if len(dukcapilFlows) != 2 {
		t.Errorf("expected 2 flows involving dukcapil, got %d", len(dukcapilFlows))
	}

	bffFlows := tr.GetByService("bff")
	if len(bffFlows) != 1 {
		t.Errorf("expected 1 flow involving bff, got %d", len(bffFlows))
	}
}

func TestSummary(t *testing.T) {
	tr := New()

	tr.Record(DataFlow{
		DataSubject: "u1", DataType: "nik",
		Source: "garudainfo", Destination: "dukcapil",
	})
	tr.Record(DataFlow{
		DataSubject: "u2", DataType: "nik",
		Source: "garudainfo", Destination: "dukcapil",
	})
	tr.Record(DataFlow{
		DataSubject: "u1", DataType: "name",
		Source: "garudainfo", Destination: "dukcapil",
	})
	tr.Record(DataFlow{
		DataSubject: "u1", DataType: "address",
		Source: "garudainfo", Destination: "bff",
	})

	summaries := tr.Summary()
	if len(summaries) != 2 {
		t.Fatalf("expected 2 summary pairs, got %d", len(summaries))
	}

	// Sorted: garudainfo->bff first, garudainfo->dukcapil second
	if summaries[0].Source != "garudainfo" || summaries[0].Destination != "bff" {
		t.Errorf("expected garudainfo->bff first, got %s->%s", summaries[0].Source, summaries[0].Destination)
	}
	if summaries[0].FlowCount != 1 {
		t.Errorf("expected flow_count 1, got %d", summaries[0].FlowCount)
	}
	if len(summaries[0].DataTypes) != 1 || summaries[0].DataTypes[0] != "address" {
		t.Errorf("expected data_types [address], got %v", summaries[0].DataTypes)
	}

	if summaries[1].FlowCount != 3 {
		t.Errorf("expected flow_count 3, got %d", summaries[1].FlowCount)
	}
	if len(summaries[1].DataTypes) != 2 {
		t.Errorf("expected 2 data types, got %d", len(summaries[1].DataTypes))
	}
}

func TestEmptyTracker(t *testing.T) {
	tr := New()

	if flows := tr.GetBySubject("nobody"); len(flows) != 0 {
		t.Errorf("expected empty, got %d", len(flows))
	}
	if flows := tr.GetByDataType("nik"); len(flows) != 0 {
		t.Errorf("expected empty, got %d", len(flows))
	}
	if flows := tr.GetByService("any"); len(flows) != 0 {
		t.Errorf("expected empty, got %d", len(flows))
	}
	if summaries := tr.Summary(); len(summaries) != 0 {
		t.Errorf("expected empty summary, got %d", len(summaries))
	}
}

func TestMultipleFlowsSameSubject(t *testing.T) {
	tr := New()

	tr.Record(DataFlow{
		DataSubject: "user-1", DataType: "nik",
		Source: "garudainfo", Destination: "dukcapil",
	})
	tr.Record(DataFlow{
		DataSubject: "user-1", DataType: "name",
		Source: "garudainfo", Destination: "bff",
	})
	tr.Record(DataFlow{
		DataSubject: "user-1", DataType: "dob",
		Source: "bff", Destination: "citizen-portal",
	})

	flows := tr.GetBySubject("user-1")
	if len(flows) != 3 {
		t.Errorf("expected 3 flows for user-1, got %d", len(flows))
	}
}

func TestConcurrentRecording(t *testing.T) {
	tr := New()
	var wg sync.WaitGroup
	n := 100

	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			tr.Record(DataFlow{
				DataSubject: "user-concurrent",
				DataType:    "nik",
				Source:      "src",
				Destination: "dst",
				Purpose:     "test",
				LegalBasis:  "consent",
				Timestamp:   time.Now(),
			})
		}()
	}
	wg.Wait()

	flows := tr.GetBySubject("user-concurrent")
	if len(flows) != n {
		t.Errorf("expected %d flows, got %d", n, len(flows))
	}

	// Verify all IDs are unique
	ids := make(map[string]bool)
	for _, f := range flows {
		if ids[f.ID] {
			t.Errorf("duplicate flow ID: %s", f.ID)
		}
		ids[f.ID] = true
	}
}
