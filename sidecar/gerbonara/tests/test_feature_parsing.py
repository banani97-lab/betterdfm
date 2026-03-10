import sys
from pathlib import Path
import pytest
sys.path.insert(0, str(Path(__file__).parent.parent))
from main import _parse_features, Trace, Pad, Via

FIXTURES = Path(__file__).parent / "fixtures"


def _run_parse(fixture_name, ltype="COPPER", units="INCH"):
    feat = FIXTURES / fixture_name
    if not feat.exists():
        pytest.skip(f"fixture {fixture_name!r} not yet created (Phase 2a)")
    traces, pads, vias = [], [], []
    _parse_features(feat, "test_layer", ltype, units, traces, pads, vias)
    return traces, pads, vias


def test_l_record_produces_trace():
    traces, pads, vias = _run_parse("features_inch.txt")
    assert len(traces) >= 1, "L record should produce at least one trace"
    t = traces[0]
    assert t.layer == "test_layer"
    assert t.widthMM > 0


def test_p_record_produces_pad():
    traces, pads, vias = _run_parse("features_inch.txt")
    assert len(pads) >= 1, "P record should produce at least one pad"
    p = pads[0]
    assert p.widthMM > 0
    assert p.heightMM > 0


def test_surface_polygon_produces_traces():
    traces, pads, vias = _run_parse("surface_polygon.txt", ltype="COPPER")
    # A 4-point polygon (4 OS + OB) should produce 4 trace segments (3 sides + close)
    assert len(traces) >= 4, f"Expected ≥4 trace segments from polygon, got {len(traces)}"


def test_eof_surface_polygon_flushed():
    """Polygon missing SE at EOF should still be emitted (EOF flush)."""
    traces, pads, vias = _run_parse("surface_polygon.txt", ltype="COPPER")
    # The fixture has two polygon blocks: one complete, one truncated at EOF
    # Both should be flushed
    assert len(traces) >= 8, f"Expected both polygons flushed, got {len(traces)} traces"


def test_non_copper_layer_skips_l_records():
    """L records on SOLDER_MASK layer should not produce traces."""
    feat = FIXTURES / "features_inch.txt"
    if not feat.exists():
        pytest.skip("fixture not yet created")
    traces, pads, vias = [], [], []
    _parse_features(feat, "mask", "SOLDER_MASK", "INCH", traces, pads, vias)
    assert len(traces) == 0, "SOLDER_MASK layer should not produce traces from L records"
