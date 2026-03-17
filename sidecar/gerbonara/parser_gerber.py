from __future__ import annotations

import logging
import tempfile
from pathlib import Path

from models import BoardData, Layer, Trace, Pad, Via, Drill, Point

logger = logging.getLogger(__name__)


def _layer_type(name: str) -> str:
    n = name.lower()
    if "copper" in n or "gtl" in n or "gbl" in n or "signal" in n or "inner" in n:
        return "COPPER"
    if "paste" in n or "gtp" in n or "gbp" in n:
        return "SOLDER_PASTE"
    if "mask" in n or "gts" in n or "gbs" in n:
        return "SOLDER_MASK"
    if "silk" in n or "gto" in n or "gbo" in n or "legend" in n:
        return "SILK"
    if "drill" in n or "drl" in n or "exc" in n:
        return "DRILL"
    if "outline" in n or "edge" in n or "gko" in n or "gm1" in n or "mechanical" in n:
        return "OUTLINE"
    return "COPPER"


# File extensions recognised as Gerber graphic layers / Excellon drill files
_GERBER_EXTS = frozenset({
    ".gbr", ".ger", ".gtl", ".gbl", ".gts", ".gbs", ".gto", ".gbo",
    ".gko", ".gm1", ".gtp", ".gbp", ".g2l", ".g3l", ".gl2", ".gl3",
    ".cmp", ".sol", ".plc", ".pls", ".stc", ".sts", ".art", ".pho",
})
_DRILL_EXTS = frozenset({".drl", ".xln", ".exc", ".ncd"})


def _ap_width_mm(ap) -> float:
    """Return the effective width of an aperture in mm."""
    try:
        return float(ap.equivalent_width("mm"))
    except Exception:
        pass
    for attr in ("diameter", "w"):
        try:
            val = getattr(ap, attr, None)
            if val is not None:
                unit = getattr(ap, "unit", None)
                if unit and str(unit) in ("in", "inch"):
                    return float(val) * 25.4
                return float(val)
        except Exception:
            pass
    return 0.1


def _ap_dims_mm(ap) -> tuple[float, float, str]:
    """Return (widthMM, heightMM, shape) for a pad aperture."""
    from gerbonara.apertures import CircleAperture, RectangleAperture, ObroundAperture

    def _cv(v: float) -> float:
        """Convert aperture-unit value to mm."""
        unit = getattr(ap, "unit", None)
        if unit and str(unit) in ("in", "inch"):
            return float(v) * 25.4
        return float(v)

    if isinstance(ap, CircleAperture):
        d = _ap_width_mm(ap)
        return d, d, "CIRCLE"
    if isinstance(ap, RectangleAperture):
        return _cv(ap.w), _cv(ap.h), "RECT"
    if isinstance(ap, ObroundAperture):
        return _cv(ap.w), _cv(ap.h), "OVAL"
    d = _ap_width_mm(ap)
    return d, d, "CIRCLE"


def _extract_graphic_layer(layer_name: str, layer_file, ltype: str,
                            traces: list, pads: list, outline: list) -> None:
    """Extract geometry from one GerberFile into traces/pads/outline lists."""
    from gerbonara.graphic_objects import Line, Arc, Flash
    from gerbonara.utils import MM

    for obj in layer_file.objects:
        if not getattr(obj, "polarity_dark", True):
            continue
        try:
            c = obj.converted(MM)  # returns a copy with all coords in mm
        except Exception:
            continue

        if isinstance(c, (Line, Arc)):
            if ltype == "OUTLINE":
                outline.append(Point(x=c.x1, y=c.y1))
                outline.append(Point(x=c.x2, y=c.y2))
            elif ltype == "COPPER":
                w = _ap_width_mm(c.aperture) if c.aperture else 0.1
                traces.append(Trace(
                    layer=layer_name, widthMM=max(0.01, w),
                    startX=c.x1, startY=c.y1, endX=c.x2, endY=c.y2,
                ))
        elif isinstance(c, Flash) and ltype == "COPPER":
            w, h, shape = _ap_dims_mm(c.aperture) if c.aperture else (1.0, 1.0, "CIRCLE")
            pads.append(Pad(layer=layer_name, x=c.x, y=c.y,
                            widthMM=max(0.01, w), heightMM=max(0.01, h), shape=shape))


def _extract_drill_file(drill_file, default_plated: bool,
                        label: str, layers_out: list, drills: list) -> None:
    """Extract drill hits from an ExcellonFile into drills list."""
    from gerbonara.graphic_objects import Flash
    from gerbonara.utils import MM

    if drill_file is None:
        return
    layers_out.append(Layer(name=label, type="DRILL"))
    for obj in drill_file.objects:
        if not isinstance(obj, Flash):
            continue
        try:
            c = obj.converted(MM)
        except Exception:
            continue
        ap = c.aperture
        d = _ap_width_mm(ap) if ap else 0.3
        is_plated = default_plated if obj.plated is None else bool(obj.plated)
        drills.append(Drill(x=c.x, y=c.y, diamMM=max(0.01, d), plated=is_plated))


def _parse_gerber_fallback(
    file_path: str,
    layers_out: list,
    traces: list,
    pads: list,
    outline: list,
    drills: list,
) -> None:
    """File-by-file fallback: unpack zip then open each layer with GerberFile/ExcellonFile."""
    import zipfile
    from gerbonara import GerberFile
    from gerbonara.excellon import ExcellonFile

    with tempfile.TemporaryDirectory() as tmpdir:
        tmp = Path(tmpdir)

        try:
            with zipfile.ZipFile(file_path, "r") as zf:
                zf.extractall(tmpdir)
        except Exception:
            src = Path(file_path)
            tmp = src.parent if src.is_file() else src

        for f in sorted(tmp.rglob("*")):
            if not f.is_file():
                continue
            suffix = f.suffix.lower()
            name = f.name
            ltype = _layer_type(name)

            if suffix in _DRILL_EXTS or ltype == "DRILL":
                try:
                    xf = ExcellonFile.open(str(f))
                    plated = "npth" not in name.lower() and "unplated" not in name.lower()
                    _extract_drill_file(xf, plated, name, layers_out, drills)
                    continue
                except Exception:
                    pass

            if suffix in _GERBER_EXTS or suffix in _DRILL_EXTS:
                try:
                    gf = GerberFile.open(str(f))
                    layers_out.append(Layer(name=name, type=ltype))
                    _extract_graphic_layer(name, gf, ltype, traces, pads, outline)
                except Exception:
                    pass


def parse_gerber(file_path: str) -> BoardData:
    """Parse a Gerber zip/directory using gerbonara 1.x API."""
    from gerbonara import LayerStack

    layers_out: list[Layer] = []
    traces: list[Trace] = []
    pads: list[Pad] = []
    vias: list[Via] = []
    drills: list[Drill] = []
    outline: list[Point] = []

    try:
        stack = LayerStack.open(str(file_path))

        for layer_key, layer_file in stack.graphic_layers.items():
            if layer_file is None:
                continue
            name = str(layer_key)
            ltype = _layer_type(name)
            layers_out.append(Layer(name=name, type=ltype))
            _extract_graphic_layer(name, layer_file, ltype, traces, pads, outline)

        _extract_drill_file(stack.drill_pth, True, "drill_pth", layers_out, drills)
        _extract_drill_file(stack.drill_npth, False, "drill_npth", layers_out, drills)
        for i, dl in enumerate(stack.drill_layers):
            _extract_drill_file(dl, True, f"drill_{i}", layers_out, drills)

    except Exception as e:
        logger.warning("LayerStack.open() failed (%s) — trying file-by-file fallback", e)
        try:
            _parse_gerber_fallback(file_path, layers_out, traces, pads, outline, drills)
        except Exception as e2:
            logger.warning("Gerber fallback also failed: %s", e2, exc_info=True)

    logger.info("Gerber done: %d layers, %d traces, %d pads, %d drills, %d outline pts",
                len(layers_out), len(traces), len(pads), len(drills), len(outline))
    return BoardData(
        layers=layers_out, traces=traces, pads=pads, vias=vias,
        drills=drills, outline=outline, boardThicknessMM=1.6,
        outlineHoles=[],
    )
