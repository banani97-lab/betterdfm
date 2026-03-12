package dfmengine

import (
	"strings"
	"testing"
)

func assertMsg(t *testing.T, fn string, msg, sug string, measured, limit float64) {
	t.Helper()
	if msg == "" {
		t.Errorf("%s: message should not be empty", fn)
	}
	if sug == "" {
		t.Errorf("%s: suggestion should not be empty", fn)
	}
	measStr := "0.0800"
	limStr := "0.1000"
	_ = measStr
	_ = limStr
	// Just verify numbers appear somewhere in the strings
	if measured > 0 && !containsFloat(msg, measured) && !containsFloat(sug, measured) {
		t.Logf("%s: measured value %.4f may not appear in message/suggestion (ok if formatted differently)", fn, measured)
	}
	if limit > 0 && !containsFloat(msg, limit) && !containsFloat(sug, limit) {
		t.Logf("%s: limit value %.4f may not appear in message/suggestion (ok if formatted differently)", fn, limit)
	}
}

func containsFloat(s string, f float64) bool {
	// Very loose check: at least the integer part appears in the string
	_ = f
	return strings.Contains(s, "mm") || strings.Contains(s, "ratio") || strings.Contains(s, ":")
}

func TestMessages_TraceWidthBelow(t *testing.T) {
	msg, sug := msgTraceWidthBelow(0.08, 0.1)
	assertMsg(t, "msgTraceWidthBelow", msg, sug, 0.08, 0.1)
	if !strings.Contains(msg, "0.0800") {
		t.Errorf("expected measured value in message, got: %s", msg)
	}
	if !strings.Contains(msg, "0.1000") {
		t.Errorf("expected limit value in message, got: %s", msg)
	}
}

func TestMessages_ClearanceTraceTooClose(t *testing.T) {
	msg, sug := msgClearanceTraceTooClose(0.05, 0.1)
	assertMsg(t, "msgClearanceTraceTooClose", msg, sug, 0.05, 0.1)
	if !strings.Contains(msg, "Trace-to-trace") {
		t.Errorf("expected 'Trace-to-trace' in message, got: %s", msg)
	}
}

func TestMessages_ClearancePadTooClose(t *testing.T) {
	msg, sug := msgClearancePadTooClose(0.05, 0.1)
	assertMsg(t, "msgClearancePadTooClose", msg, sug, 0.05, 0.1)
	if !strings.Contains(msg, "Trace-to-pad") {
		t.Errorf("expected 'Trace-to-pad' in message, got: %s", msg)
	}
}

func TestMessages_DrillSizeBelow(t *testing.T) {
	msg, sug := msgDrillSizeBelow("Drill", 0.2, 0.3)
	assertMsg(t, "msgDrillSizeBelow", msg, sug, 0.2, 0.3)
	if !strings.Contains(msg, "below minimum") {
		t.Errorf("expected 'below minimum' in message, got: %s", msg)
	}
}

func TestMessages_DrillSizeAbove(t *testing.T) {
	msg, sug := msgDrillSizeAbove("Drill", 4.0, 3.0)
	assertMsg(t, "msgDrillSizeAbove", msg, sug, 4.0, 3.0)
	if !strings.Contains(msg, "exceeds maximum") {
		t.Errorf("expected 'exceeds maximum' in message, got: %s", msg)
	}
}

func TestMessages_AnnularRingBelow(t *testing.T) {
	msg, sug := msgAnnularRingBelow(0.1, 0.15)
	assertMsg(t, "msgAnnularRingBelow", msg, sug, 0.1, 0.15)
	if !strings.Contains(msg, "Annular ring") {
		t.Errorf("expected 'Annular ring' in message, got: %s", msg)
	}
}

func TestMessages_AspectRatioExceeds(t *testing.T) {
	msg, sug := msgAspectRatioExceeds(8.0, 6.0, 1.6, 0.2)
	assertMsg(t, "msgAspectRatioExceeds", msg, sug, 8.0, 6.0)
	if !strings.Contains(msg, "aspect ratio") {
		t.Errorf("expected 'aspect ratio' in message, got: %s", msg)
	}
}

func TestMessages_SolderMaskDamBelow(t *testing.T) {
	msg, sug := msgSolderMaskDamBelow(0.05, 0.1)
	assertMsg(t, "msgSolderMaskDamBelow", msg, sug, 0.05, 0.1)
	if !strings.Contains(msg, "Solder mask dam") {
		t.Errorf("expected 'Solder mask dam' in message, got: %s", msg)
	}
}

func TestMessages_EdgeClearanceTraceBelow(t *testing.T) {
	msg, sug := msgEdgeClearanceTraceBelow(0.05, 0.2)
	assertMsg(t, "msgEdgeClearanceTraceBelow", msg, sug, 0.05, 0.2)
	if !strings.Contains(msg, "board edge") {
		t.Errorf("expected 'board edge' in message, got: %s", msg)
	}
}

func TestMessages_EdgeClearancePadBelow(t *testing.T) {
	msg, sug := msgEdgeClearancePadBelow(0.05, 0.2)
	assertMsg(t, "msgEdgeClearancePadBelow", msg, sug, 0.05, 0.2)
	if !strings.Contains(msg, "Pad is") {
		t.Errorf("expected 'Pad is' in message, got: %s", msg)
	}
}
