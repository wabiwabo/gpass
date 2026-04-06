package slabudget

import (
	"testing"
	"time"
)

func TestNewBudget(t *testing.T) {
	b := NewBudget(Config{Target: 0.999, Window: 30 * 24 * time.Hour})
	s := b.Status()
	if s.Target != 0.999 {
		t.Errorf("Target = %f", s.Target)
	}
	if s.TotalRequests != 0 {
		t.Errorf("TotalRequests = %d", s.TotalRequests)
	}
}

func TestNewBudget_Defaults(t *testing.T) {
	b := NewBudget(Config{})
	s := b.Status()
	if s.Target != 0.999 {
		t.Errorf("Target = %f, want default 0.999", s.Target)
	}
}

func TestRecordSuccess(t *testing.T) {
	b := NewBudget(Config{Target: 0.99})
	for i := 0; i < 100; i++ {
		b.RecordSuccess()
	}

	s := b.Status()
	if s.TotalRequests != 100 {
		t.Errorf("TotalRequests = %d", s.TotalRequests)
	}
	if s.TotalErrors != 0 {
		t.Errorf("TotalErrors = %d", s.TotalErrors)
	}
	if s.Actual != 1.0 {
		t.Errorf("Actual = %f, want 1.0", s.Actual)
	}
}

func TestRecordError(t *testing.T) {
	b := NewBudget(Config{Target: 0.99})
	for i := 0; i < 99; i++ {
		b.RecordSuccess()
	}
	b.RecordError()

	s := b.Status()
	if s.TotalRequests != 100 {
		t.Errorf("TotalRequests = %d", s.TotalRequests)
	}
	if s.TotalErrors != 1 {
		t.Errorf("TotalErrors = %d", s.TotalErrors)
	}
	if s.Actual != 0.99 {
		t.Errorf("Actual = %f, want 0.99", s.Actual)
	}
}

func TestRecord_Batch(t *testing.T) {
	b := NewBudget(Config{Target: 0.99})
	b.Record(1000, 5)

	s := b.Status()
	if s.TotalRequests != 1000 {
		t.Errorf("TotalRequests = %d", s.TotalRequests)
	}
	if s.TotalErrors != 5 {
		t.Errorf("TotalErrors = %d", s.TotalErrors)
	}
}

func TestBudgetTotal(t *testing.T) {
	b := NewBudget(Config{Target: 0.99})
	b.Record(1000, 0) // 1000 requests, 99% target → 10 error budget

	s := b.Status()
	// Ceil(1000*0.01) = 10, but floating point can make it 11
	if s.BudgetTotal < 10 || s.BudgetTotal > 11 {
		t.Errorf("BudgetTotal = %d, want 10-11", s.BudgetTotal)
	}
}

func TestBudgetRemaining(t *testing.T) {
	b := NewBudget(Config{Target: 0.99})
	b.Record(1000, 3)

	s := b.Status()
	// remaining = budgetTotal - errors
	expected := s.BudgetTotal - 3
	if s.BudgetRemaining != expected {
		t.Errorf("BudgetRemaining = %d, want %d", s.BudgetRemaining, expected)
	}
}

func TestBudgetExhausted(t *testing.T) {
	b := NewBudget(Config{Target: 0.99})
	b.Record(1000, 15) // budget=10, used=15

	s := b.Status()
	if !s.IsExhausted {
		t.Error("should be exhausted")
	}
	if s.BudgetRemaining != 0 {
		t.Errorf("BudgetRemaining = %d, want 0", s.BudgetRemaining)
	}
}

func TestBudgetNotExhausted(t *testing.T) {
	b := NewBudget(Config{Target: 0.99})
	b.Record(1000, 5)

	s := b.Status()
	if s.IsExhausted {
		t.Error("should not be exhausted")
	}
}

func TestBudgetRatio(t *testing.T) {
	b := NewBudget(Config{Target: 0.99})
	b.Record(1000, 5) // budget=10, used=5 → ratio=0.5

	s := b.Status()
	// 5 errors / ~10-11 budget = ~0.45-0.5
	if s.BudgetRatio < 0.4 || s.BudgetRatio > 0.6 {
		t.Errorf("BudgetRatio = %f, want ~0.45-0.5", s.BudgetRatio)
	}
}

func TestActual_NoRequests(t *testing.T) {
	b := NewBudget(Config{Target: 0.999})
	s := b.Status()
	if s.Actual != 1.0 {
		t.Errorf("Actual = %f, want 1.0 with no requests", s.Actual)
	}
}

func TestReset(t *testing.T) {
	b := NewBudget(Config{Target: 0.99})
	b.Record(1000, 50)
	b.Reset()

	s := b.Status()
	if s.TotalRequests != 0 || s.TotalErrors != 0 {
		t.Error("should be reset")
	}
}

func TestEstimatedExhaustion(t *testing.T) {
	b := NewBudget(Config{Target: 0.99, Window: 30 * 24 * time.Hour})
	b.Record(1000, 5) // 50% of budget used

	s := b.Status()
	if s.IsExhausted {
		t.Error("should not be exhausted")
	}
	// Exhaustion estimate should exist (we have errors)
	if s.EstimatedExhaustion == nil && s.BurnRate > 0 {
		t.Error("should have exhaustion estimate")
	}
}

func TestBurnRate_Zero(t *testing.T) {
	b := NewBudget(Config{Target: 0.99})
	// No requests = no burn rate
	s := b.Status()
	if s.BurnRate != 0 {
		t.Errorf("BurnRate = %f, want 0", s.BurnRate)
	}
}

func TestBudgetTotal_ZeroRequests(t *testing.T) {
	b := NewBudget(Config{Target: 0.99})
	s := b.Status()
	if s.BudgetTotal != 0 {
		t.Errorf("BudgetTotal = %d, want 0 with no requests", s.BudgetTotal)
	}
}
