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
