package admin

import (
	"testing"
)

func TestNewMetrics(t *testing.T) {
	m := NewMetrics()
	if m == nil {
		t.Fatal("NewMetrics returned nil")
	}
	if m.RequestsTotal == nil {
		t.Error("RequestsTotal is nil")
	}
	if m.RequestDuration == nil {
		t.Error("RequestDuration is nil")
	}
	if m.BytesIn == nil {
		t.Error("BytesIn is nil")
	}
	if m.BytesOut == nil {
		t.Error("BytesOut is nil")
	}
}
