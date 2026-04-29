import sys
from pathlib import Path
import pytest

sys.path.insert(0, str(Path(__file__).parent.parent))
from units import _coord_to_mm, _sym_to_mm


def test_coord_inch_to_mm():
    assert abs(_coord_to_mm(1.0, "INCH") - 25.4) < 0.001


def test_coord_mm_identity():
    assert _coord_to_mm(1.0, "MM") == 1.0
    assert _coord_to_mm(25.4, "MM") == 25.4


def test_coord_case_insensitive():
    assert _coord_to_mm(1.0, "inch") == _coord_to_mm(1.0, "INCH")
    assert _coord_to_mm(1.0, "mm") == _coord_to_mm(1.0, "MM")


def test_sym_to_mm_inch():
    # 1000 mils = 1 inch = 25.4 mm
    assert abs(_sym_to_mm(1000, "INCH") - 25.4) < 0.001


def test_sym_to_mm_mm():
    # 1000 microns = 1 mm
    assert abs(_sym_to_mm(1000, "MM") - 1.0) < 0.001


def test_sym_to_mm_r304_8_mm():
    # r304.8 in MM file → 304.8 µm = 0.3048 mm
    assert abs(_sym_to_mm(304.8, "MM") - 0.3048) < 0.001


def test_sym_to_mm_r304_8_inch():
    # r304.8 in INCH file → 304.8 mils = 7.742 mm (very different!)
    assert abs(_sym_to_mm(304.8, "INCH") - 7.742) < 0.05


def test_sym_to_mm_never_negative():
    # Negative raw values should stay negative (not our concern to clamp)
    # but make sure zero works
    assert _sym_to_mm(0, "INCH") == 0.0
    assert _sym_to_mm(0, "MM") == 0.0


def test_feature_file_units_override_drill_inch():
    """A drill feature file declaring UNITS=INCH must override an MM step header.

    Regression: on the Dalsa Sycamore board, stephdr was UNITS=MM but the
    drill feature file was UNITS=INCH. Using the step-level units for the
    drill file squashed every drill coordinate ~25× and mis-scaled symbol
    diameters so the sub-50µm marker filter silently dropped ~99% of drills.
    """
    from pathlib import Path
    from parser_odb import _parse_features

    feat = Path(__file__).parent / "fixtures" / "features_inch_drill.txt"
    traces, pads, vias, polygons = [], [], [], []
    drills: list = []
    # Caller passes the step-level units ("MM"); per-file override must win.
    _parse_features(feat, "drill", "DRILL", "MM",
                    traces, pads, vias, drills=drills, polygons=polygons)

    assert len(drills) == 3, f"expected all 3 drill records parsed, got {len(drills)}"

    # Coords must be inch-scaled: 0.5" → 12.7 mm, 5" → 127 mm.
    xs = sorted(d.x for d in drills)
    assert abs(xs[0] - 12.7) < 0.01
    assert abs(xs[-1] - 127.0) < 0.01

    # Symbols must be mil-scaled: r10 → 0.254 mm, not 10 µm (which would
    # have been filtered out by the sub-50µm drill marker guard).
    diams = sorted(round(d.diamMM, 4) for d in drills)
    assert abs(diams[0] - 0.254) < 0.001    # r10 as mils
    assert abs(diams[1] - 0.508) < 0.001    # r20 as mils
    assert abs(diams[2] - 3.500) < 0.01     # r137.795 as mils


# ── `U MM` / `U INCH` syntax (Mentor / Cadence variant) ──────────────────────

def test_read_units_u_form_stephdr(tmp_path):
    """`_read_units` must accept both UNITS=MM and the `U MM` Mentor variant.

    Without this, boards exported by tools that use `U MM` fall through to
    the INCH default and every drill diameter is 25.4× too big — `r240`
    reads as 240 mils (6.096mm) instead of 240µm (0.240mm).
    """
    from parser_odb import _read_units
    p = tmp_path / "stephdr"
    p.write_text("X_DATUM=0\nU MM\nX_ORIGIN=0\n")
    assert _read_units(p) == "MM"

    p2 = tmp_path / "stephdr2"
    p2.write_text("U INCH\n")
    assert _read_units(p2) == "INCH"


def test_read_units_units_form_still_works(tmp_path):
    from parser_odb import _read_units
    p = tmp_path / "stephdr"
    p.write_text("UNITS=MM\n")
    assert _read_units(p) == "MM"


def test_read_units_default_inch(tmp_path):
    """No declaration at all → INCH default (preserves old behaviour)."""
    from parser_odb import _read_units
    p = tmp_path / "stephdr"
    p.write_text("X_DATUM=0\nX_ORIGIN=0\n")
    assert _read_units(p) == "INCH"


def test_features_file_units_u_form_overrides_step():
    """`_features_file_units` must recognise `U MM` as a per-file override."""
    from parser_odb import _features_file_units
    lines = ["#", "#Units", "#", "U MM", "", "$0 r240"]
    warnings: list[str] = []
    assert _features_file_units(lines, "INCH", "drill", warnings) == "MM"
    assert any("UNITS=MM" in w for w in warnings)


def test_features_file_units_u_form_matches_step_no_warning():
    """If `U MM` matches step-level MM, no override warning is emitted."""
    from parser_odb import _features_file_units
    lines = ["U MM"]
    warnings: list[str] = []
    assert _features_file_units(lines, "MM", "drill", warnings) == "MM"
    assert warnings == []
