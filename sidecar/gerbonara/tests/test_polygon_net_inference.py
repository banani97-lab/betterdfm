"""Tests for _infer_polygon_nets — the four-pass net backfill that rescues
copper pour polygons whose ODB++ `S` record has no `.net=` attribute.

Without this inference, the clearance rule's same-net skip never matches
pour-edge pseudo-traces to their adjacent thermal-relief catch-pads, and
every via on a plane layer fires a false positive.
"""
import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).parent.parent))

from parser_odb import _infer_polygon_nets, _point_in_polygon, _point_in_ring
from models import Polygon, Pad, Point, Trace, Via


def _square(cx, cy, half, layer="L02_GND", net=""):
    return Polygon(
        layer=layer,
        points=[
            Point(x=cx - half, y=cy - half),
            Point(x=cx + half, y=cy - half),
            Point(x=cx + half, y=cy + half),
            Point(x=cx - half, y=cy + half),
        ],
        netName=net,
    )


def test_point_in_ring_basic():
    ring = [Point(x=0, y=0), Point(x=10, y=0), Point(x=10, y=10), Point(x=0, y=10)]
    assert _point_in_ring(5, 5, ring)
    assert not _point_in_ring(-1, 5, ring)
    assert not _point_in_ring(11, 5, ring)


def test_point_in_polygon_respects_holes():
    outer = [Point(x=0, y=0), Point(x=10, y=0), Point(x=10, y=10), Point(x=0, y=10)]
    hole  = [Point(x=3, y=3), Point(x=7, y=3), Point(x=7, y=7), Point(x=3, y=7)]
    poly = Polygon(layer="L02_GND", points=outer, holes=[hole], netName="")
    assert _point_in_polygon(1, 1, poly)        # inside outer, outside hole
    assert not _point_in_polygon(5, 5, poly)    # inside hole → excluded


def test_pass1_pads_inside_plane():
    # A big GND plane with three thermal-relief pads inside, one on another net.
    poly = _square(50, 50, 40)   # 80×80 mm pour
    pads = [
        Pad(layer="L02_GND", x=30, y=30, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="GND_D", refDes=""),
        Pad(layer="L02_GND", x=50, y=50, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="GND_D", refDes=""),
        Pad(layer="L02_GND", x=70, y=70, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="GND_D", refDes=""),
        Pad(layer="L02_GND", x=60, y=40, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="VCC_3V3", refDes=""),
    ]
    _infer_polygon_nets([poly], pads, [], [], [], warnings=None)
    assert poly.netName == "GND_D"   # 3 GND_D beats 1 VCC_3V3


def test_pass1_ignores_pads_inside_holes():
    # Plane with an anti-pad hole; a non-GND via catch-pad sits inside the hole.
    outer = [Point(x=0, y=0), Point(x=10, y=0), Point(x=10, y=10), Point(x=0, y=10)]
    hole  = [Point(x=4, y=4), Point(x=6, y=4), Point(x=6, y=6), Point(x=4, y=6)]
    poly = Polygon(layer="L02_GND", points=outer, holes=[hole], netName="")
    pads = [
        # Non-GND via catch-pad inside the anti-pad hole — must be ignored.
        Pad(layer="L02_GND", x=5, y=5, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="VCC_3V3", refDes=""),
        # Two GND thermal-relief pads in the pour itself.
        Pad(layer="L02_GND", x=1, y=1, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="GND_D", refDes=""),
        Pad(layer="L02_GND", x=9, y=9, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="GND_D", refDes=""),
    ]
    _infer_polygon_nets([poly], pads, [], [], [], warnings=None)
    assert poly.netName == "GND_D"


def test_pass2_traces_inside_island_with_no_pads():
    # Small island on a signal layer — no pads inside, but a trace passes through.
    poly = _square(50, 50, 2, layer="L01_TOP")
    traces = [
        Trace(layer="L01_TOP", widthMM=0.15,
              startX=49, startY=49, endX=51, endY=51, netName="VCC_1V8"),
    ]
    _infer_polygon_nets([poly], [], traces, [], [], warnings=None)
    assert poly.netName == "VCC_1V8"


def test_pass3_netlist_points_inside_island():
    # Island with no same-layer features, but a netlist point lies inside.
    poly = _square(50, 50, 1, layer="L03_SIGNAL_PWR")
    net_points = [(50.1, 50.1, "VCC_CORE")]
    _infer_polygon_nets([poly], [], [], [], net_points, warnings=None)
    assert poly.netName == "VCC_CORE"


def test_pass4_nearest_via_for_sliver_polygons():
    # A thermal-relief sliver adjacent to a signal via. The via sits 0.37 mm
    # from the polygon's centroid — outside the polygon itself — but well
    # within the 1 mm proximity tolerance.
    poly = _square(10.0, 10.0, 0.2, layer="L03_SIGNAL_PWR")
    vias = [
        Via(x=10.0, y=10.37, outerDiamMM=0.76, drillDiamMM=0.4, netName="VS_TCK_1V8"),
    ]
    _infer_polygon_nets([poly], [], [], vias, [], warnings=None)
    assert poly.netName == "VS_TCK_1V8"


def test_pass4_respects_tolerance():
    # A via 2 mm away must NOT be adopted — too far to be "adjacent."
    poly = _square(10.0, 10.0, 0.2, layer="L03_SIGNAL_PWR")
    vias = [
        Via(x=10.0, y=12.0, outerDiamMM=0.76, drillDiamMM=0.4, netName="UNRELATED"),
    ]
    _infer_polygon_nets([poly], [], [], vias, [], warnings=None)
    assert poly.netName == ""


def test_already_labeled_polygon_is_untouched():
    poly = _square(10.0, 10.0, 5, net="PREEXISTING")
    # Put a conflicting pad inside — shouldn't matter.
    pads = [
        Pad(layer="L02_GND", x=10, y=10, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="GND_D", refDes=""),
    ]
    _infer_polygon_nets([poly], pads, [], [], [], warnings=None)
    assert poly.netName == "PREEXISTING"


def test_pass_priority_pads_before_nearest_via():
    # If a pad is inside the polygon AND a via is nearby on a different net,
    # the pad pass wins (it's the more rigorous signal).
    poly = _square(10, 10, 1)
    pads = [
        Pad(layer="L02_GND", x=10, y=10, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="GND_D", refDes=""),
    ]
    vias = [
        Via(x=10.5, y=10.5, outerDiamMM=0.76, drillDiamMM=0.4, netName="DECOY"),
    ]
    _infer_polygon_nets([poly], pads, [], vias, [], warnings=None)
    assert poly.netName == "GND_D"


def test_split_plane_sub_polygons_get_distinct_nets():
    # Two non-overlapping sub-polygons on the same layer; each should be
    # labeled from pads inside its own boundary, not from a layer-wide vote.
    p1 = _square(10, 10, 5)  # GND sub-pour
    p2 = _square(50, 10, 5)  # VCC sub-pour
    pads = [
        Pad(layer="L02_GND", x=10, y=10, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="GND_D", refDes=""),
        Pad(layer="L02_GND", x=11, y=11, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="GND_D", refDes=""),
        Pad(layer="L02_GND", x=50, y=10, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="VCC_3V3", refDes=""),
        Pad(layer="L02_GND", x=51, y=11, widthMM=0.6, heightMM=0.6,
            shape="CIRCLE", netName="VCC_3V3", refDes=""),
    ]
    _infer_polygon_nets([p1, p2], pads, [], [], [], warnings=None)
    assert p1.netName == "GND_D"
    assert p2.netName == "VCC_3V3"


def test_empty_inputs_are_harmless():
    _infer_polygon_nets([], [], [], [], [], warnings=None)
    poly = _square(0, 0, 1)
    _infer_polygon_nets([poly], [], [], [], [], warnings=None)
    assert poly.netName == ""
