package output

import (
	"context"
	"testing"
)

func TestNoOpTracer_Start(t *testing.T) {
	var tr NoOpTracer
	ctx, span := tr.Start(context.Background(), "op")
	if ctx == nil {
		t.Error("context must not be nil")
	}
	span.End()
}

func TestNoOpSpan_AllMethods(_ *testing.T) {
	var tr NoOpTracer
	_, span := tr.Start(context.Background(), "op",
		WithAttributes(Attribute{Key: "k", Value: "v"}),
	)
	span.SetAttributes(Attribute{Key: "k2", Value: true})
	span.AddEvent("e", Attribute{Key: "k3", Value: 1})
	span.RecordError(nil)
	span.SetStatus(StatusOK, "ok")
	span.SetStatus(StatusError, "err")
	span.SetStatus(StatusUnset, "")
	span.End()
}

func TestStartSpanConfig_Attributes(t *testing.T) {
	attrs := []Attribute{
		{Key: "a", Value: "x"},
		{Key: "b", Value: 42},
	}
	opt := WithAttributes(attrs...)
	var cfg StartSpanConfig
	opt(&cfg)
	got := cfg.Attributes()
	if len(got) != len(attrs) {
		t.Fatalf("got %d attributes, want %d", len(got), len(attrs))
	}
	for i, a := range attrs {
		if got[i].Key != a.Key {
			t.Errorf("attr[%d].Key = %q, want %q", i, got[i].Key, a.Key)
		}
	}
}
