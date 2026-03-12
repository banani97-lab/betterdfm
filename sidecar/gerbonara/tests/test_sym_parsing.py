import sys
from pathlib import Path
import pytest

sys.path.insert(0, str(Path(__file__).parent.parent))
from parser_odb import _parse_sym, _parse_symbol_table

FIXTURES = Path(__file__).parent / "fixtures"


@pytest.mark.parametrize("sym,units,expected_shape,expected_w,tol", [
    # CIRCLE (round pad)
    ("r304.8",             "MM",   "CIRCLE", 0.3048, 0.001),
    ("r304.8",             "INCH", "CIRCLE", 7.742,  0.05),
    ("r10",                "INCH", "CIRCLE", 0.254,  0.001),
    # RECT (square pad s<n>)
    ("s700",               "INCH", "RECT",   17.78,  0.1),
    ("s700",               "MM",   "RECT",   0.7,    0.005),
    # OVAL
    ("oval550x1650",       "INCH", "OVAL",   13.97,  0.1),
    ("oval550x1650 M",     "INCH", "OVAL",   13.97,  0.1),   # trailing modifier
    ("oval550x1650",       "MM",   "OVAL",   0.55,   0.005),
    # RECT (rectangular pad)
    ("rect2720x1230",      "INCH", "RECT",   69.09,  0.5),
    ("rect550x800xr49.5",  "INCH", "RECT",   13.97,  0.1),   # corner radius stripped
    # DONUT (via)
    ("donut_r78.74x27.559","INCH", "DONUT",  2.0,    0.05),
    # moire / thermal → circle placeholder
    ("moire",              "INCH", "CIRCLE", 1.0,    0.001),
    ("thermal",            "INCH", "CIRCLE", 1.0,    0.001),
    # unknown symbol → fallback
    ("sc_join0201_hd",     "INCH", "CIRCLE", 0.1,    0.001),
    ("",                   "INCH", "CIRCLE", 0.1,    0.001),  # empty string → fallback
])
def test_parse_sym_shape_and_width(sym, units, expected_shape, expected_w, tol):
    result = _parse_sym(sym, units)
    assert result["shape"] == expected_shape, f"sym={sym!r} units={units}: expected shape {expected_shape}, got {result['shape']}"
    assert abs(result["w"] - expected_w) <= tol, (
        f"sym={sym!r} units={units}: expected w≈{expected_w}, got {result['w']:.4f}"
    )


def test_parse_sym_donut_inner_less_than_outer():
    result = _parse_sym("donut_r78.74x27.559", "INCH")
    assert result["inner"] < result["w"], "donut inner must be less than outer"
    assert result["inner"] > 0, "donut inner must be positive"


def test_parse_sym_oval_height():
    result = _parse_sym("oval550x1650", "INCH")
    assert result["shape"] == "OVAL"
    # height should be larger than width
    assert result["h"] > result["w"]


def test_parse_sym_rect_dimensions():
    result = _parse_sym("rect2720x1230", "INCH")
    assert result["shape"] == "RECT"
    assert result["w"] > result["h"]  # 2720 > 1230


def test_parse_symbol_table_inch(tmp_path):
    """Symbol table parsed from fixture file should yield correct shapes and sizes."""
    sym_file = FIXTURES / "sym_table_inch.txt"
    if not sym_file.exists():
        pytest.skip("fixture not yet created (Phase 2a)")
    lines = sym_file.read_text().splitlines()
    table = _parse_symbol_table(lines, "INCH")
    # $0 r254 → CIRCLE ~6.45mm
    assert 0 in table
    assert table[0]["shape"] == "CIRCLE"
    assert abs(table[0]["w"] - 6.45) < 0.1
    # $1 s700 → RECT ~17.78mm
    assert 1 in table
    assert table[1]["shape"] == "RECT"


def test_parse_symbol_table_mm(tmp_path):
    """Symbol table for MM unit board: r304.8 → ~0.3048mm (not 7.7mm)."""
    sym_file = FIXTURES / "sym_table_mm.txt"
    if not sym_file.exists():
        pytest.skip("fixture not yet created (Phase 2a)")
    lines = sym_file.read_text().splitlines()
    table = _parse_symbol_table(lines, "MM")
    assert 0 in table
    assert table[0]["shape"] == "CIRCLE"
    assert table[0]["w"] < 1.0, f"MM pad should be sub-mm, got {table[0]['w']:.3f}mm"
    assert table[0]["w"] > 0.2, f"MM pad too small: {table[0]['w']:.3f}mm"
