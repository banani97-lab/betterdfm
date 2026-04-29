import sys
from pathlib import Path
import pytest

sys.path.insert(0, str(Path(__file__).parent.parent))
from parser_odb import _parse_sym, _parse_symbol_table, _scan_custom_symbol, _load_custom_symbols

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


# ── Custom (special_*) symbol geometry ────────────────────────────────────────

def test_scan_custom_symbol_uses_bbox():
    """A custom symbol's features file → RECT with the union bbox of its surfaces.

    Without this fallback, names like `special_*_domekey_outer_*` lose their
    encoded geometry and shrink to a 0.1mm circle in the renderer.
    """
    shape = _scan_custom_symbol(FIXTURES / "custom_symbol_features.txt", "MM")
    assert shape is not None
    assert shape["shape"] == "RECT"
    assert shape["w"] == pytest.approx(5.0, abs=0.01)
    assert shape["h"] == pytest.approx(3.0, abs=0.01)


def test_load_custom_symbols_returns_empty_for_missing_dir(tmp_path):
    """Missing symbols/ directory → empty dict, no error."""
    out = _load_custom_symbols(tmp_path / "no_such_dir", "MM")
    assert out == {}


def test_parse_sym_uses_custom_syms_for_unrecognized_name():
    """`_parse_sym` prefers a custom_syms entry over the 0.1mm fallback."""
    custom = {"special_widget_q42": {"shape": "RECT", "w": 4.5, "h": 2.7, "inner": 0.0}}
    out = _parse_sym("special_widget_q42", "MM", custom_syms=custom)
    assert out["shape"] == "RECT"
    assert out["w"] == pytest.approx(4.5)
    assert out["h"] == pytest.approx(2.7)


def test_parse_sym_falls_back_when_custom_syms_misses():
    """Heuristic still wins for names the symbol scanner didn't see."""
    out = _parse_sym("r10", "INCH", custom_syms={})
    assert out["shape"] == "CIRCLE"
    assert out["w"] == pytest.approx(0.254, abs=0.001)


def test_load_custom_symbols_scans_dir_and_lowercases(tmp_path):
    """End-to-end: directory of `<name>/features` → registry keyed by name + lower."""
    sym_dir = tmp_path / "MyWidget"
    sym_dir.mkdir(parents=True)
    (sym_dir / "features").write_text(
        "UNITS=MM\nS P 0 ;;ID=1\nOB 0 0 I\nOS 4 0\nOS 4 2\nOS 0 2\nOE\nSE\n"
    )
    out = _load_custom_symbols(tmp_path, "MM")
    assert "MyWidget" in out
    assert "mywidget" in out  # lowercased so _parse_sym's lower() lookup succeeds
    assert out["MyWidget"]["shape"] == "RECT"
    assert out["MyWidget"]["w"] == pytest.approx(4.0)
    assert out["MyWidget"]["h"] == pytest.approx(2.0)
