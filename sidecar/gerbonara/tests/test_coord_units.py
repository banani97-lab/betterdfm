import sys
from pathlib import Path
import pytest

sys.path.insert(0, str(Path(__file__).parent.parent))
from main import _coord_to_mm, _sym_to_mm


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
