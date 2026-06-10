"""Contour pads: custom-symbol polygon extraction and P-record placement."""

import math
import sys
from pathlib import Path

import pytest

sys.path.insert(0, str(Path(__file__).parent.parent))
from parser_odb import (  # noqa: E402
    _MAX_CONTOUR_POINTS,
    _decimate_ring,
    _load_custom_symbols,
    _parse_features,
    _place_contour,
    _scan_custom_symbol,
)

FIXTURES = Path(__file__).parent / "fixtures"


def _write_symbol(tmp_path: Path, name: str, body: str) -> Path:
    d = tmp_path / name
    d.mkdir(parents=True)
    (d / "features").write_text(body)
    return tmp_path


# ── _scan_custom_symbol ───────────────────────────────────────────────────────

L_SHAPE = (
    "UNITS=MM\n"
    "S P 0 ;;ID=1\n"
    "OB 0 0 I\n"
    "OS 4 0\nOS 4 1\nOS 1 1\nOS 1 3\nOS 0 3\n"
    "OE\nSE\n"
)


def test_l_shaped_symbol_contour(tmp_path):
    """An L-shaped surface keeps its concave contour (a hull would not)."""
    root = _write_symbol(tmp_path, "lshape", L_SHAPE)
    shape = _scan_custom_symbol(root / "lshape" / "features", "MM")
    assert shape["shape"] == "POLYGON"
    assert shape["w"] == pytest.approx(4.0)
    assert shape["h"] == pytest.approx(3.0)
    pts = {(round(x, 3), round(y, 3)) for x, y in shape["contour"]}
    assert pts == {(0, 0), (4, 0), (4, 1), (1, 1), (1, 3), (0, 3)}


def test_multi_island_symbol_keeps_largest_ring(tmp_path):
    """Two boundary islands → largest-area ring wins, multi_ring flagged,
    bbox still spans both islands."""
    body = (
        "UNITS=MM\n"
        "S P 0 ;;ID=1\n"
        "OB 0 0 I\nOS 1 0\nOS 1 1\nOS 0 1\nOE\n"   # small 1x1
        "OB 2 0 I\nOS 6 0\nOS 6 3\nOS 2 3\nOE\n"   # large 4x3
        "SE\n"
    )
    root = _write_symbol(tmp_path, "twin", body)
    shape = _scan_custom_symbol(root / "twin" / "features", "MM")
    assert shape["shape"] == "POLYGON"
    assert shape["multi_ring"] is True
    assert shape["w"] == pytest.approx(6.0)  # union bbox of both islands
    xs = [x for x, _ in shape["contour"]]
    assert min(xs) == pytest.approx(2.0)  # contour is the larger island only

    warnings: list[str] = []
    _load_custom_symbols(root, "MM", warnings=warnings)
    assert len(warnings) == 1
    assert "twin" in warnings[0]


def test_hole_rings_do_not_become_contour(tmp_path):
    """H rings feed the bbox but are never selected as the outer contour."""
    body = (
        "UNITS=MM\n"
        "S P 0 ;;ID=1\n"
        "OB 0 0 I\nOS 4 0\nOS 4 4\nOS 0 4\nOE\n"
        "OB 1 1 H\nOS 3 1\nOS 3 3\nOS 1 3\nOE\n"
        "SE\n"
    )
    root = _write_symbol(tmp_path, "holed", body)
    shape = _scan_custom_symbol(root / "holed" / "features", "MM")
    assert shape["shape"] == "POLYGON"
    assert shape["multi_ring"] is False
    assert len(shape["contour"]) == 4
    assert shape["w"] == pytest.approx(4.0)


def test_contour_is_decimated(tmp_path):
    """A finely tessellated boundary is capped at _MAX_CONTOUR_POINTS."""
    pts = [
        (5 + 5 * math.cos(2 * math.pi * i / 256),
         5 + 5 * math.sin(2 * math.pi * i / 256))
        for i in range(256)
    ]
    body = "UNITS=MM\nS P 0 ;;ID=1\n" + f"OB {pts[0][0]:.4f} {pts[0][1]:.4f} I\n"
    body += "".join(f"OS {x:.4f} {y:.4f}\n" for x, y in pts[1:]) + "OE\nSE\n"
    root = _write_symbol(tmp_path, "circle", body)
    shape = _scan_custom_symbol(root / "circle" / "features", "MM")
    assert shape["shape"] == "POLYGON"
    assert len(shape["contour"]) <= _MAX_CONTOUR_POINTS
    # Decimation must not visibly shrink the shape.
    xs = [x for x, _ in shape["contour"]]
    assert max(xs) - min(xs) == pytest.approx(10.0, abs=0.05)


def test_decimate_ring_preserves_sharp_corners():
    """The perpendicular-distance filter drops collinear points first."""
    ring = [(0, 0), (1, 0), (2, 0), (3, 0), (4, 0), (4, 3), (0, 3)]
    out = _decimate_ring(ring, 4)
    assert (4, 3) in out and (0, 3) in out
    assert (0, 0) in out and (4, 0) in out


# ── _place_contour ────────────────────────────────────────────────────────────

def test_place_contour_translates():
    out = _place_contour([(1.0, 2.0)], 10.0, 20.0, False, 0.0)
    assert out[0] == pytest.approx((11.0, 22.0))


def test_place_contour_rotates_clockwise():
    # ODB++ rotations are clockwise: +X axis rotates onto -Y at 90°.
    out = _place_contour([(1.0, 0.0)], 0.0, 0.0, False, 90.0)
    assert out[0][0] == pytest.approx(0.0, abs=1e-9)
    assert out[0][1] == pytest.approx(-1.0)


def test_place_contour_mirrors_then_rotates():
    # Mirror negates x first, then the clockwise rotation applies.
    out = _place_contour([(1.0, 0.0)], 0.0, 0.0, True, 90.0)
    assert out[0][0] == pytest.approx(0.0, abs=1e-9)
    assert out[0][1] == pytest.approx(1.0)


# ── P-record placement end-to-end ─────────────────────────────────────────────

def _parse_pads(tmp_path: Path, feature_lines: str):
    feat = tmp_path / "features"
    feat.write_text(feature_lines)
    traces, pads, vias = [], [], []
    custom = {"lpad": {
        "shape": "POLYGON", "w": 4.0, "h": 3.0, "inner": 0.0,
        "contour": [(0, 0), (4, 0), (4, 1), (1, 1), (1, 3), (0, 3)],
        "multi_ring": False,
    }}
    _parse_features(feat, "top", "COPPER", "MM", traces, pads, vias,
                    custom_syms=custom)
    return pads


def test_p_record_places_contour_at_xy(tmp_path):
    pads = _parse_pads(tmp_path, "UNITS=MM\n$0 lpad\nP 10 20 0 P 0 8 0\n")
    assert len(pads) == 1
    p = pads[0]
    assert p.shape == "POLYGON"
    assert len(p.contour) == 6
    xs = [pt.x for pt in p.contour]
    ys = [pt.y for pt in p.contour]
    assert min(xs) == pytest.approx(10.0)
    assert max(xs) == pytest.approx(14.0)
    assert min(ys) == pytest.approx(20.0)
    # Pad x/y is the placed contour's bbox center, w/h its bbox dims.
    assert p.x == pytest.approx(12.0)
    assert p.y == pytest.approx(21.5)
    assert p.widthMM == pytest.approx(4.0)
    assert p.heightMM == pytest.approx(3.0)


def test_p_record_rotates_contour_arbitrary_angle(tmp_path):
    """45° used to be silently treated as 0°; contours rotate exactly."""
    pads = _parse_pads(tmp_path, "UNITS=MM\n$0 lpad\nP 0 0 0 P 0 8 45\n")
    p = pads[0]
    # The (4, 0) corner rotated 45° CW lands at (4cos45, -4sin45).
    corner = min(p.contour, key=lambda pt: pt.y)
    assert corner.x == pytest.approx(4 * math.cos(math.radians(45)), abs=1e-6)
    assert corner.y == pytest.approx(-4 * math.sin(math.radians(45)), abs=1e-6)
    # bbox of the L rotated 45° CW: x' = (x+y)/sqrt2 spans 0 .. (4+1)/sqrt2.
    assert p.widthMM == pytest.approx(5 / math.sqrt(2), abs=1e-6)


def test_p_record_short_orient_code_rotates_rect(tmp_path):
    """Short-form orient codes (no angle field) rotate parametric pads:
    `P x y sym P 0 1` is 90° CW, so a RECT's w/h swap."""
    feat = tmp_path / "features"
    # MM-file symbol dims are microns: rect2000x1000 = 2.0 x 1.0 mm.
    feat.write_text("UNITS=MM\n$0 rect2000x1000\nP 5 5 0 P 0 1\n")
    traces, pads, vias = [], [], []
    _parse_features(feat, "top", "COPPER", "MM", traces, pads, vias)
    assert len(pads) == 1
    assert pads[0].widthMM == pytest.approx(1.0)
    assert pads[0].heightMM == pytest.approx(2.0)
