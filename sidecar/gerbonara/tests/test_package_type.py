"""Tests for coarse package-type classification driving component-spacing.

The classifier picks one of discrete | leaded | bga | through_hole from the
ODB++ PKG record (IPC name token + pin-grid geometry) plus the CMP mount type.
Cases mirror real packages seen on the Dalsa board (3/3 BGA recall, 0 false
positives during validation): standard BGA grids, a depopulated 4x2 micro-BGA,
QFN/SOP leaded parts, and 2xN strip connectors that must NOT read as BGA.
"""
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).parent.parent))
from parser_odb import _classify_package_type, _grid_dims, _parse_eda_packages


def _grid(rows, cols, pitch=1.0):
    """Generate a rows x cols grid of pin centers at the given pitch (mm)."""
    return [(c * pitch, r * pitch) for r in range(rows) for c in range(cols)]


def test_grid_dims_counts_rows_and_cols():
    assert _grid_dims(_grid(9, 12)) == (9, 12)
    assert _grid_dims([(0, 0), (1, 0)]) == (1, 2)


def test_mount_type_wins_for_through_hole():
    # Even a footprint that looks discrete is through_hole when press-fit/THMT.
    assert _classify_package_type("FUSE_1812", [(0, 0), (2, 0)], {"RC"}, "thmt") == "through_hole"
    assert _classify_package_type("TO127", _grid(7, 2), {"CR", "RC"}, "pressfit") == "through_hole"


def test_passive_name_token_is_discrete():
    for nm in ("CAPC1608X100N", "RESC1005X40N", "INDC3216X130N", "CAPMP7343X400N"):
        assert _classify_package_type(nm, [(0, 0), (2, 0)], {"RC"}, "smt") == "discrete", nm


def test_bga_grid_detected():
    # Full 108-ball array: round pads, near-square, fully populated.
    assert _classify_package_type("BGA108C127P1600", _grid(9, 12), {"CR"}, "smt") == "bga"


def test_depopulated_micro_bga_detected():
    # 8-ball 4x2 micro-BGA: aspect 2.0 (<=3), round, >=8 balls -> still BGA.
    assert _classify_package_type("BGA8C50P4X2", _grid(2, 4, pitch=0.5), {"CR"}, "smt") == "bga"


def test_strip_connector_not_bga():
    # 20x2 round-pad Samtec strip: aspect 10 rejects it -> leaded, not bga.
    assert _classify_package_type("SAMTEC_IPL1-120-01-SM-D-K", _grid(2, 20), {"CR"}, "smt") == "leaded"


def test_qfn_is_leaded_not_bga():
    # QFN perimeter pads are rectangular (RC), so the round-only gate excludes it.
    assert _classify_package_type("QFN50P400X400X80EP260-21N", _grid(7, 7), {"RC", "SQ"}, "smt") == "leaded"


def test_two_pin_unnamed_smt_is_leaded():
    # SOD/SOT diode: not RES/CAP/IND named, not a BGA grid -> leaded bucket.
    assert _classify_package_type("SOD1680X70N", [(0, 0), (1.7, 0)], {"RC"}, "smt") == "leaded"


def test_name_based_bga_fallback_without_geometry():
    # No EDA pins available; name token still routes BGA correctly.
    assert _classify_package_type("BGA256", [], set(), "smt") == "bga"


def test_unknown_with_no_signal_is_empty():
    assert _classify_package_type("", [], set(), "") == ""


def test_eda_packages_capture_pins_and_shapes(tmp_path):
    data = tmp_path / "data"
    data.write_text(
        "PKG BGA4C100 1.0 -1 -1 1 1;;ID=1\n"
        "PIN A1 S -0.5 0.5 0 U U ID=2\n"
        "CR -0.5 0.5 0.25\n"
        "PIN A2 S 0.5 0.5 0 U U ID=3\n"
        "CR 0.5 0.5 0.25\n"
        "PIN B1 S -0.5 -0.5 0 U U ID=4\n"
        "CR -0.5 -0.5 0.25\n"
        "PIN B2 S 0.5 -0.5 0 U U ID=5\n"
        "CR 0.5 -0.5 0.25\n"
        "PKG RESC1608 1.0 -0.8 -0.4 0.8 0.4;;ID=6\n"
        "PIN 1 S -0.7 0 0 U U ID=7\n"
        "RC -0.7 0 0.4 0.5\n"
    )
    pkgs = _parse_eda_packages(data, "MM")
    assert pkgs[0]["name"] == "BGA4C100"
    assert len(pkgs[0]["pins"]) == 4
    assert pkgs[0]["pad_shapes"] == {"CR"}
    assert pkgs[1]["name"] == "RESC1608"
    assert pkgs[1]["pad_shapes"] == {"RC"}
