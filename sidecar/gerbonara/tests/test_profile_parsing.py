"""Tests for _parse_profile: outer boundary vs inner hole extraction."""
import sys
from pathlib import Path
import pytest

sys.path.insert(0, str(Path(__file__).parent.parent))

from parser_odb import _parse_profile

FIXTURES = Path(__file__).parent / "fixtures"


def test_profile_outer_boundary_only():
    """A profile with only an I-flagged ring returns boundary points, no holes."""
    boundary, holes = _parse_profile(FIXTURES / "profile_with_hole.txt", "INCH")
    assert len(boundary) >= 4, f"Expected ≥4 boundary points, got {len(boundary)}"
    assert len(holes) == 1, f"Expected 1 hole ring, got {len(holes)}"


def test_profile_hole_ring_extracted():
    """The H-flagged ring must appear in holes, not in boundary."""
    boundary, holes = _parse_profile(FIXTURES / "profile_with_hole.txt", "INCH")
    assert len(holes) == 1
    hole = holes[0]
    assert len(hole) >= 3, f"Hole ring should have ≥3 points, got {len(hole)}"


def test_profile_boundary_coords_converted_from_inch():
    """Coordinates should be converted from inches to mm (×25.4)."""
    boundary, _ = _parse_profile(FIXTURES / "profile_with_hole.txt", "INCH")
    xs = [p.x for p in boundary]
    # Outer ring spans 0–1 inch = 0–25.4mm
    assert max(xs) > 20, f"Expected mm coords ~25.4, got max x={max(xs)}"


def test_profile_missing_file_returns_empty():
    """A non-existent profile file returns empty boundary and holes without raising."""
    boundary, holes = _parse_profile(FIXTURES / "no_such_profile.txt", "INCH")
    assert boundary == []
    assert holes == []


def test_profile_boundary_only_no_hole_flag():
    """Profile with no H-flagged blocks returns an empty holes list."""
    # features_inch.txt is not a profile, but we can create an inline fixture
    import tempfile
    content = "OB 0 0 I\nOS 1 0\nOS 1 1\nOS 0 1\nOE\n"
    with tempfile.NamedTemporaryFile(mode="w", suffix=".txt", delete=False) as f:
        f.write(content)
        path = Path(f.name)
    try:
        boundary, holes = _parse_profile(path, "INCH")
        assert len(holes) == 0, f"No H-flagged rings → holes should be empty, got {holes}"
        assert len(boundary) >= 4
    finally:
        path.unlink()


def test_profile_arc_tessellated_not_chord():
    """OC arc edges must tessellate, not collapse to a chord.

    Fixture profile is a square with a semicircle bulging out the right side,
    centered at (1, 0.5) with radius 0.5 inch (12.7 mm). Without tessellation
    the polygon collapses to a 1×1 inch square (max x = 25.4mm); with proper
    tessellation it bulges to max x ≈ 38.1 mm.
    """
    boundary, _ = _parse_profile(FIXTURES / "profile_with_arc.txt", "INCH")
    xs = [p.x for p in boundary]
    assert max(xs) > 36, (
        f"Arc not tessellated — max x = {max(xs):.2f}mm; "
        f"expected ≈38.1mm if the arc bulges out, "
        f"≈25.4mm if it collapsed to a chord"
    )
    # And the polygon must have intermediate arc samples, not just 4 corners
    assert len(boundary) > 6, (
        f"Expected arc tessellation samples, got {len(boundary)} points"
    )


def test_profile_self_resolves_u_mm_declaration(tmp_path):
    """Profile files exported by Mentor / Cadence-derived tools declare units
    as `U MM` directly in the profile file, even when stephdr has no UNITS line
    at all. The parser must self-resolve from the file header, not just take
    the caller's step-level units; otherwise a 100mm board outline reads as
    100" = 2540mm and the actual features render as a smudge in one corner.
    """
    p = tmp_path / "profile"
    p.write_text("#\n#Units\n#\nU MM\n\nOB 0 0 I\nOS 100 0\nOS 100 80\nOS 0 80\nOE\n")
    # Caller passes "INCH" (the stephdr default); the file's own `U MM`
    # declaration must override.
    boundary, _ = _parse_profile(p, "INCH")
    xs = [pt.x for pt in boundary]
    ys = [pt.y for pt in boundary]
    assert max(xs) - min(xs) == pytest.approx(100.0, abs=0.01), (
        f"expected ~100mm wide outline, got {max(xs) - min(xs):.2f}mm — "
        "profile parser ignored the file's own U MM declaration"
    )
    assert max(ys) - min(ys) == pytest.approx(80.0, abs=0.01)
