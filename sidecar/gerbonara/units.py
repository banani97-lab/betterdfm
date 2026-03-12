from __future__ import annotations


def _coord_to_mm(v: float, units: str) -> float:
    """Convert coordinate from file units to mm."""
    return v * 25.4 if units.upper() == "INCH" else v


def _sym_to_mm(v: float, units: str) -> float:
    """Convert symbol dimension to mm using the correct scale for the file's unit system.

    ODB++ symbol dimensions are always in 1/1000 of the design unit:
    - INCH files: dims in mils (1/1000 inch) → multiply by 0.0254
    - MM files:   dims in microns (1/1000 mm) → multiply by 0.001
    """
    return v * 0.001 if units.upper() == "MM" else v * 0.0254
