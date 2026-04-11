"""Tests for side-aware refdes spatial lookup.

Regression: top-side chip pins sitting directly above bottom-side passives
(or vice versa) used to be attributed to whichever component the spatial
lookup happened to find first, because the index didn't track which side
of the board each component was on. That caused pad-size-for-package to
flag top-side QFN pins as "undersized 0603 pads" on a C-reference. The
fix: the index now filters by side, and the pad's side is derived from
its layer name.
"""
import sys
from pathlib import Path
sys.path.insert(0, str(Path(__file__).parent.parent))
from parser_odb import _RefdesIndex, _layer_side, _parse_components


def test_layer_side_classification():
    assert _layer_side("L01_TOP") == "top"
    assert _layer_side("L06_BOTTOM") == "bot"
    assert _layer_side("TOP_PASTE") == "top"
    assert _layer_side("BOTTOM_SOLDER") == "bot"
    assert _layer_side("F.Cu") == "top"
    assert _layer_side("B.Cu") == "bot"
    # Internal planes don't belong to either outer stack
    assert _layer_side("L02_GND") is None
    assert _layer_side("L03_SIGNAL_PWR") is None


def test_refdes_index_filters_by_side():
    # A QFN chip (U1) on top at (10, 10) and a 0603 cap (C1) on bottom
    # at the exact same XY — the chip sits directly above the cap.
    components = [
        (10.0, 10.0, "U1", "QFN32", "top"),
        (10.0, 10.0, "C1", "0603", "bot"),
    ]
    idx = _RefdesIndex(components)

    # A pad on the top signal layer must resolve to U1, not C1.
    name, pkg = idx.lookup(10.0, 10.0, side="top")
    assert name == "U1"

    # A pad on the bottom signal layer must resolve to C1, not U1.
    name, pkg = idx.lookup(10.0, 10.0, side="bot")
    assert name == "C1"
    assert pkg == "0603"


def test_refdes_index_no_side_matches_any():
    # Backward-compat path: callers that don't know the pad's side should
    # still get a match (picks the nearest within tolerance).
    components = [(10.0, 10.0, "C1", "0603", "bot")]
    idx = _RefdesIndex(components)
    name, _ = idx.lookup(10.0, 10.0, side=None)
    assert name == "C1"


def test_refdes_index_unknown_component_side_is_wildcard():
    # If the component's side couldn't be recovered (empty string), it
    # should still match lookups of any side — preserves legacy behavior
    # for boards where the CMP mirror flag is absent.
    components = [(10.0, 10.0, "X1", "0603", "")]
    idx = _RefdesIndex(components)
    assert idx.lookup(10.0, 10.0, side="top")[0] == "X1"
    assert idx.lookup(10.0, 10.0, side="bot")[0] == "X1"


def test_parse_components_reads_mirror_flag(tmp_path):
    cmp_file = tmp_path / "components"
    cmp_file.write_text(
        "UNITS=MM\n"
        "CMP 0 10.0 10.0 0 N U1 QFN32 ;\n"
        "CMP 1 10.0 10.0 0 M C1 0603 ;\n"
    )
    comps = _parse_components(cmp_file, "MM")
    sides = {c[2]: c[4] for c in comps}
    assert sides == {"U1": "top", "C1": "bot"}


def test_parse_components_side_hint_wins(tmp_path):
    # Directory-derived hint (e.g. components/top) should override the
    # mirror flag in the file — the path is the most reliable signal.
    cmp_file = tmp_path / "components"
    cmp_file.write_text(
        "UNITS=MM\n"
        "CMP 0 10.0 10.0 0 M C1 0603 ;\n"  # mirror=M → would be bot
    )
    comps = _parse_components(cmp_file, "MM", side_hint="top")
    assert comps[0][4] == "top"
