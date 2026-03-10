import math, sys
from pathlib import Path
import pytest
sys.path.insert(0, str(Path(__file__).parent.parent))
from main import _arc_segments

def test_full_circle_segment_count():
    # Full circle: start == end → 8 segments (default n=8)
    segs = _arc_segments(1.0, 0.0, 1.0, 0.0, 0.0, 0.0, cw=False)
    assert len(segs) == 8

def test_full_circle_closes():
    # Last endpoint == first startpoint
    segs = _arc_segments(1.0, 0.0, 1.0, 0.0, 0.0, 0.0, cw=False)
    assert abs(segs[-1][2] - segs[0][0]) < 1e-6
    assert abs(segs[-1][3] - segs[0][1]) < 1e-6

def test_quarter_arc_produces_segments():
    # Quarter circle arc from (1,0) to (0,1) CCW, center (0,0)
    segs = _arc_segments(1.0, 0.0, 0.0, 1.0, 0.0, 0.0, cw=False)
    assert len(segs) >= 1

def test_quarter_arc_endpoints():
    segs = _arc_segments(1.0, 0.0, 0.0, 1.0, 0.0, 0.0, cw=False)
    # First segment starts at (1, 0)
    assert abs(segs[0][0] - 1.0) < 1e-6
    assert abs(segs[0][1] - 0.0) < 1e-6
    # Last segment ends at (0, 1)
    assert abs(segs[-1][2] - 0.0) < 1e-6
    assert abs(segs[-1][3] - 1.0) < 1e-6

def test_zero_radius_returns_empty():
    segs = _arc_segments(0.0, 0.0, 0.0, 0.0, 0.0, 0.0, cw=False)
    assert segs == []

def test_all_points_on_circle():
    # All segment endpoints should lie on the circle of radius r
    r = 3.0
    segs = _arc_segments(r, 0.0, r, 0.0, 0.0, 0.0, cw=False)
    for x1, y1, x2, y2 in segs:
        assert abs(math.sqrt(x1**2 + y1**2) - r) < 1e-6
        assert abs(math.sqrt(x2**2 + y2**2) - r) < 1e-6
