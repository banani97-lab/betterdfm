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


# ── Via emission via .padstack_id cross-reference ─────────────────────────────

def test_via_from_padstack_id_cross_reference():
    """Copper parsed first populates the map; drill lookup emits a Via."""
    from parser_odb import _parse_features
    traces, pads, vias, polygons, drills = [], [], [], [], []
    padstack_outer_mm: dict = {}

    # Pass 1: copper layer populates padstack 335 → 0.508mm and 336 → 0.508mm
    _parse_features(FIXTURES / "copper_via_catchpad.txt", "art01", "COPPER",
                    "MM", traces, pads, vias, drills=drills,
                    polygons=polygons, padstack_outer_mm=padstack_outer_mm)
    assert padstack_outer_mm[335] == pytest.approx(0.508)
    assert padstack_outer_mm[336] == pytest.approx(0.508)
    assert len(vias) == 0  # copper pass emits pads, not vias

    # Pass 2: drill layer with padstack_id attrs but no regex-matchable geometry
    _parse_features(FIXTURES / "drill_padstack_id.txt", "drill", "DRILL",
                    "MM", traces, pads, vias, drills=drills,
                    polygons=polygons, padstack_outer_mm=padstack_outer_mm)

    assert len(vias) == 2, f"expected 2 vias from padstack cross-ref, got {len(vias)}"
    for v in vias:
        assert v.outerDiamMM == pytest.approx(0.508)
        assert v.drillDiamMM == pytest.approx(0.254, abs=0.001)


def test_via_padstack_id_picks_min_outer_across_layers():
    """Inner-layer catch-pad (r406 → 0.406) must beat top-layer (r508 → 0.508)."""
    from parser_odb import _parse_features
    traces, pads, vias, polygons, drills = [], [], [], [], []
    padstack_outer_mm: dict = {}

    _parse_features(FIXTURES / "copper_via_catchpad.txt", "art01", "COPPER",
                    "MM", traces, pads, vias, drills=drills,
                    polygons=polygons, padstack_outer_mm=padstack_outer_mm)
    _parse_features(FIXTURES / "copper_via_catchpad_inner.txt", "art07", "COPPER",
                    "MM", traces, pads, vias, drills=drills,
                    polygons=polygons, padstack_outer_mm=padstack_outer_mm)
    assert padstack_outer_mm[335] == pytest.approx(0.406)  # inner wins

    _parse_features(FIXTURES / "drill_padstack_id.txt", "drill", "DRILL",
                    "MM", traces, pads, vias, drills=drills,
                    polygons=polygons, padstack_outer_mm=padstack_outer_mm)
    via_335 = [v for v in vias if abs(v.outerDiamMM - 0.406) < 0.001]
    assert len(via_335) == 1, "padstack 335 via must use the smaller inner OD"


def test_via_regex_D_H_still_works():
    """Regression: the D...H... regex path still emits a Via. Values are
    scaled via _sym_to_mm: microns for MM files, mils for INCH files."""
    from parser_odb import _via_geometry_mm
    outer, hole = _via_geometry_mm("0=0", {0: "D500H250"}, "MM")   # 500µm / 250µm
    assert outer == pytest.approx(0.5)
    assert hole == pytest.approx(0.25)
    outer, hole = _via_geometry_mm("0=0", {0: "D20H10"}, "INCH")    # 20mil / 10mil
    assert outer == pytest.approx(0.508)
    assert hole == pytest.approx(0.254)


def test_via_hole_round_p_regex_still_works():
    """Regression: the hole0.1_round0.25_p regex path still emits a Via."""
    traces, pads, vias, polygons, drills = [], [], [], [], []
    _parse_features(FIXTURES / "drill_hole_round_p.txt", "drill", "DRILL",
                    "MM", traces, pads, vias, drills=drills,
                    polygons=polygons)
    assert len(vias) == 1
    assert vias[0].outerDiamMM == pytest.approx(0.5)
    assert vias[0].drillDiamMM == pytest.approx(0.25)


def test_via_altium_vxh_regex():
    """New regex: Altium-style 'VIA 600x300' in an MM file → 0.6 / 0.3 mm."""
    from parser_odb import _via_geometry_mm
    outer, hole = _via_geometry_mm("0=0", {0: "VIA 600x300"}, "MM")
    assert outer == pytest.approx(0.6)
    assert hole == pytest.approx(0.3)


def test_no_padstack_map_does_not_crash():
    """Passing no padstack_outer_mm falls back to existing behavior
    (drill record added, no Via when regex can't match)."""
    from parser_odb import _parse_features
    traces, pads, vias, polygons, drills = [], [], [], [], []
    _parse_features(FIXTURES / "drill_padstack_id.txt", "drill", "DRILL",
                    "MM", traces, pads, vias, drills=drills,
                    polygons=polygons, padstack_outer_mm=None)
    assert len(drills) == 2, "drill records still emitted"
    assert len(vias) == 0, "no vias without padstack map and no regex match"
