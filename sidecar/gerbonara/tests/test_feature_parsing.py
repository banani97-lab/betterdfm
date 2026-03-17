import sys
from pathlib import Path
import pytest
sys.path.insert(0, str(Path(__file__).parent.parent))
from parser_odb import _parse_features
from models import Trace, Pad, Via

FIXTURES = Path(__file__).parent / "fixtures"


def _run_parse(fixture_name, ltype="COPPER", units="INCH"):
    feat = FIXTURES / fixture_name
    if not feat.exists():
        pytest.skip(f"fixture {fixture_name!r} not yet created (Phase 2a)")
    traces, pads, vias, polygons = [], [], [], []
    _parse_features(feat, "test_layer", ltype, units, traces, pads, vias, polygons=polygons)
    return traces, pads, vias, polygons


def test_l_record_produces_trace():
    traces, pads, vias, _ = _run_parse("features_inch.txt")
    assert len(traces) >= 1, "L record should produce at least one trace"
    t = traces[0]
    assert t.layer == "test_layer"
    assert t.widthMM > 0


def test_p_record_produces_pad():
    traces, pads, vias, _ = _run_parse("features_inch.txt")
    assert len(pads) >= 1, "P record should produce at least one pad"
    p = pads[0]
    assert p.widthMM > 0
    assert p.heightMM > 0


def test_surface_polygon_produces_polygon():
    traces, pads, vias, polygons = _run_parse("surface_polygon.txt", ltype="COPPER")
    # Surface polygon OB+OS+OE+SE block should produce a Polygon, not traces
    assert len(polygons) >= 1, f"Expected ≥1 polygon from surface block, got {len(polygons)}"
    assert len(polygons[0].points) >= 4, (
        f"Expected polygon with ≥4 points, got {len(polygons[0].points)}"
    )


def test_eof_surface_polygon_flushed():
    """Polygon missing SE at EOF should still be emitted (EOF flush)."""
    traces, pads, vias, polygons = _run_parse("surface_polygon.txt", ltype="COPPER")
    # The fixture has two polygon blocks: one complete, one truncated at EOF.
    # Both should be flushed into the polygons list.
    assert len(polygons) >= 2, f"Expected both polygons flushed, got {len(polygons)} polygons"


def test_non_copper_layer_skips_l_records():
    """L records on SOLDER_MASK layer should not produce traces."""
    feat = FIXTURES / "features_inch.txt"
    if not feat.exists():
        pytest.skip("fixture not yet created")
    traces, pads, vias, polygons = [], [], [], []
    _parse_features(feat, "mask", "SOLDER_MASK", "INCH", traces, pads, vias, polygons=polygons)
    assert len(traces) == 0, "SOLDER_MASK layer should not produce traces from L records"
