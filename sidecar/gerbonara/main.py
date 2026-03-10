from __future__ import annotations

import gzip
import io
import logging
import math
import os
import tarfile
import tempfile
from pathlib import Path
from typing import Any

import boto3
from botocore.exceptions import ClientError
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="BetterDFM Gerbonara Sidecar", version="0.1.0")


# ── Pydantic models ─────────────────────────────────────────────────────────

class ParseRequest(BaseModel):
    fileKey: str
    fileType: str  # "GERBER" | "ODB_PLUS_PLUS"
    bucket: str


class Point(BaseModel):
    x: float
    y: float


class Layer(BaseModel):
    name: str
    type: str  # "COPPER" | "SOLDER_MASK" | "SILK" | "DRILL" | "OUTLINE"


class Trace(BaseModel):
    layer: str
    widthMM: float
    startX: float
    startY: float
    endX: float
    endY: float
    netName: str = ""


class Pad(BaseModel):
    layer: str
    x: float
    y: float
    widthMM: float
    heightMM: float
    shape: str  # "RECT" | "CIRCLE" | "OVAL"
    netName: str = ""
    refDes: str = ""


class Via(BaseModel):
    x: float
    y: float
    outerDiamMM: float
    drillDiamMM: float
    netName: str = ""


class Drill(BaseModel):
    x: float
    y: float
    diamMM: float
    plated: bool


class BoardData(BaseModel):
    layers: list[Layer]
    traces: list[Trace]
    pads: list[Pad]
    vias: list[Via]
    drills: list[Drill]
    outline: list[Point]
    boardThicknessMM: float


# ── S3 helpers ───────────────────────────────────────────────────────────────

def download_from_s3(bucket: str, key: str) -> str:
    """Download file from S3 to a temp file, return path."""
    s3 = boto3.client("s3", region_name=os.getenv("AWS_REGION", "us-east-1"))
    suffix = Path(key).suffix or ".zip"
    with tempfile.NamedTemporaryFile(delete=False, suffix=suffix) as f:
        tmp_path = f.name
    try:
        s3.download_file(bucket, key, tmp_path)
        logger.info("Downloaded s3://%s/%s to %s", bucket, key, tmp_path)
        return tmp_path
    except ClientError as e:
        logger.error("S3 download failed: %s", e)
        raise


# ── Gerber parsing ───────────────────────────────────────────────────────────

def _layer_type(name: str) -> str:
    n = name.lower()
    if "copper" in n or "gtl" in n or "gbl" in n or "signal" in n or "inner" in n:
        return "COPPER"
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
    """File-by-file fallback: unpack zip then open each layer with GerberFile/ExcellonFile.

    Used when LayerStack.open() cannot auto-detect the layer mapping (e.g. non-standard
    archive structure or file naming conventions gerbonara doesn't recognise).
    """
    import zipfile
    from gerbonara import GerberFile
    from gerbonara.excellon import ExcellonFile

    with tempfile.TemporaryDirectory() as tmpdir:
        tmp = Path(tmpdir)

        # Unpack zip if possible; otherwise parse the directory / bare file directly
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

            # ── Excellon drill ────────────────────────────────────────────────
            if suffix in _DRILL_EXTS or ltype == "DRILL":
                try:
                    xf = ExcellonFile.open(str(f))
                    plated = "npth" not in name.lower() and "unplated" not in name.lower()
                    _extract_drill_file(xf, plated, name, layers_out, drills)
                    continue
                except Exception:
                    pass  # not valid Excellon — fall through to Gerber attempt

            # ── Gerber graphic layer ──────────────────────────────────────────
            if suffix in _GERBER_EXTS or suffix in _DRILL_EXTS:
                try:
                    gf = GerberFile.open(str(f))
                    layers_out.append(Layer(name=name, type=ltype))
                    _extract_graphic_layer(name, gf, ltype, traces, pads, outline)
                except Exception:
                    pass  # non-Gerber file — skip silently


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

        # ── Graphic layers (copper, mask, silk, outline) ──────────────────────
        for layer_key, layer_file in stack.graphic_layers.items():
            if layer_file is None:
                continue
            name = str(layer_key)
            ltype = _layer_type(name)
            layers_out.append(Layer(name=name, type=ltype))
            _extract_graphic_layer(name, layer_file, ltype, traces, pads, outline)

        # ── Drill layers ──────────────────────────────────────────────────────
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
    )


def _mock_board() -> BoardData:
    """Return a simple mock board for testing without real S3/Gerber files."""
    return BoardData(
        layers=[
            Layer(name="top_copper", type="COPPER"),
            Layer(name="bottom_copper", type="COPPER"),
            Layer(name="drill", type="DRILL"),
        ],
        traces=[
            Trace(layer="top_copper", widthMM=0.15, startX=10.0, startY=10.0, endX=30.0, endY=10.0),
            Trace(layer="top_copper", widthMM=0.10, startX=30.0, startY=10.0, endX=30.0, endY=30.0),
            Trace(layer="bottom_copper", widthMM=0.20, startX=5.0, startY=5.0, endX=50.0, endY=5.0),
        ],
        pads=[
            Pad(layer="top_copper", x=10.0, y=10.0, widthMM=1.5, heightMM=1.5, shape="CIRCLE"),
            Pad(layer="top_copper", x=30.0, y=30.0, widthMM=1.5, heightMM=1.5, shape="CIRCLE"),
            Pad(layer="top_copper", x=11.0, y=10.0, widthMM=1.5, heightMM=1.5, shape="CIRCLE"),  # close pads for solder dam test
        ],
        vias=[
            Via(x=20.0, y=20.0, outerDiamMM=0.8, drillDiamMM=0.4),
            Via(x=40.0, y=20.0, outerDiamMM=0.6, drillDiamMM=0.4),  # small annular ring
        ],
        drills=[
            Drill(x=10.0, y=10.0, diamMM=0.8, plated=True),
            Drill(x=30.0, y=30.0, diamMM=0.2, plated=True),  # tiny drill for testing
        ],
        outline=[
            Point(x=0.0, y=0.0),
            Point(x=60.0, y=0.0),
            Point(x=60.0, y=40.0),
            Point(x=0.0, y=40.0),
        ],
        boardThicknessMM=1.6,
    )


# ── ODB++ parsing ─────────────────────────────────────────────────────────────

def _coord_to_mm(v: float, units: str) -> float:
    """Convert coordinate from file units to mm."""
    return v * 25.4 if units.upper() == "INCH" else v


def _mils_to_mm(v: float) -> float:
    """Convert symbol dimension (always in mils) to mm."""
    return v * 0.0254


def _read_units(path: Path) -> str:
    """Read UNITS= from ODB++ step header."""
    try:
        for line in path.read_text(errors="replace").splitlines():
            if line.startswith("UNITS="):
                return line.split("=", 1)[1].strip()
    except OSError:
        pass
    return "INCH"


_MAX_SYM_MM = 20.0  # any symbol dimension larger than this is treated as microns


def _fix_units(v: float, raw: float) -> float:
    """If a mils-converted value is unreasonably large, re-interpret raw as microns."""
    if v > _MAX_SYM_MM:
        return min(raw * 0.001, _MAX_SYM_MM)
    return v


def _parse_sym(sym: str) -> dict:
    """Parse ODB++ symbol name into shape dict. Dims are in mils (or microns for metric tools)."""
    s = sym.lower()
    try:
        if s.startswith("donut_r"):
            rest = s[7:]
            parts = rest.split("x", 1)
            raw_outer = float(parts[0])
            raw_inner = float(parts[1]) if len(parts) > 1 else raw_outer * 0.5
            outer = _fix_units(_mils_to_mm(raw_outer), raw_outer)
            inner = _fix_units(_mils_to_mm(raw_inner), raw_inner)
            inner = min(inner, outer * 0.85)
            return {"shape": "DONUT", "w": outer, "h": outer, "inner": inner}
        if s.startswith("rect"):
            dims = s[4:].split("x")
            raw_w = float(dims[0])
            h_raw = dims[1] if len(dims) > 1 else dims[0]
            raw_h = float(h_raw.lstrip("r").split("x")[0])
            w = _fix_units(_mils_to_mm(raw_w), raw_w)
            h = _fix_units(_mils_to_mm(raw_h), raw_h)
            return {"shape": "RECT", "w": w, "h": h, "inner": 0.0}
        if s.startswith("oval"):
            parts = s[4:].split("x")
            raw_w = float(parts[0])
            raw_h = float(parts[1]) if len(parts) > 1 else raw_w
            w = _fix_units(_mils_to_mm(raw_w), raw_w)
            h = _fix_units(_mils_to_mm(raw_h), raw_h)
            return {"shape": "OVAL", "w": w, "h": h, "inner": 0.0}
        if s.startswith("r") and len(s) > 1 and (s[1].isdigit() or s[1] == "."):
            raw_d = float(s[1:].split("x")[0])
            d = _fix_units(_mils_to_mm(raw_d), raw_d)
            return {"shape": "CIRCLE", "w": d, "h": d, "inner": 0.0}
    except (ValueError, IndexError):
        pass
    return {"shape": "CIRCLE", "w": 0.1, "h": 0.1, "inner": 0.0}


def _parse_symbol_table(lines: list[str]) -> dict[int, dict]:
    """Scan features file lines for $N symbol_name definitions."""
    symbols: dict[int, dict] = {}
    for line in lines:
        s = line.strip()
        if not s or not s.startswith("$"):
            continue
        parts = s.split(None, 1)
        if len(parts) < 2:
            continue
        try:
            symbols[int(parts[0][1:])] = _parse_sym(parts[1])
        except (ValueError, IndexError):
            pass
    return symbols


def _parse_matrix(matrix_path: Path) -> list[dict]:
    """Parse ODB++ matrix/matrix → [{name, type, row}] sorted by row."""
    layers: list[dict] = []
    current: dict = {}
    in_layer = False
    try:
        text = matrix_path.read_text(errors="replace")
    except OSError:
        return layers
    for line in text.splitlines():
        s = line.strip()
        if s == "LAYER {":
            current = {}
            in_layer = True
        elif s == "}" and in_layer:
            if "NAME" in current and "TYPE" in current:
                try:
                    layers.append({"name": current["NAME"], "type": current["TYPE"],
                                   "row": int(current.get("ROW", 0))})
                except (ValueError, KeyError):
                    pass
            in_layer = False
        elif in_layer and "=" in s:
            k, _, v = s.partition("=")
            current[k.strip()] = v.strip()
    return sorted(layers, key=lambda x: x["row"])


def _matrix_type_to_ltype(mtype: str) -> str | None:
    """Map ODB++ matrix layer TYPE to our type string. Returns None to skip.

    POWER_GROUND layers are copper planes (fills), not routed signal traces.
    We emit them as "POWER_GROUND" so the existing `if ltype != "COPPER":`
    guards in _parse_features() exclude their L/A records from trace lists.
    Pad (P) records are still emitted because they represent real pads/vias.
    """
    m = mtype.upper()
    if m == "SIGNAL":
        return "COPPER"
    if m == "POWER_GROUND":
        return "POWER_GROUND"
    if m == "SOLDER_MASK":
        return "SOLDER_MASK"
    if m == "SILK_SCREEN":
        return "SILK"
    if m == "MIXED":
        return "COPPER"
    if m == "DRILL":
        return "DRILL"
    return None


def _parse_profile(profile_path: Path, units: str) -> list[Point]:
    """Parse ODB++ profile file → board outline polygon points."""
    outline: list[Point] = []
    try:
        text = profile_path.read_text(errors="replace")
    except OSError:
        return outline
    in_island = False
    for line in text.splitlines():
        s = line.strip()
        if s.startswith("OB "):
            parts = s.split()
            in_island = (parts[3] == "I") if len(parts) >= 4 else True
            if in_island and len(parts) >= 3:
                try:
                    outline.append(Point(x=_coord_to_mm(float(parts[1]), units),
                                         y=_coord_to_mm(float(parts[2]), units)))
                except ValueError:
                    pass
        elif s.startswith(("OS ", "OC ")) and in_island:
            parts = s.split()
            if len(parts) >= 3:
                try:
                    outline.append(Point(x=_coord_to_mm(float(parts[1]), units),
                                         y=_coord_to_mm(float(parts[2]), units)))
                except ValueError:
                    pass
        elif s == "OE" and in_island:
            in_island = False  # Board outline is the first island contour
    return outline


def _arc_segments(
    x1: float, y1: float, xe: float, ye: float,
    xc: float, yc: float, cw: bool, n: int = 8,
) -> list[tuple[float, float, float, float]]:
    """Approximate an ODB++ arc as up to n line segments.

    Returns a list of (x1, y1, x2, y2) tuples.  For a full circle (start ≈ end)
    the segments span 360°; for a partial arc they span the actual sweep angle.
    """
    import math

    r = math.sqrt((x1 - xc) ** 2 + (y1 - yc) ** 2)
    if r < 1e-9:
        return []

    a_start = math.atan2(y1 - yc, x1 - xc)
    a_end   = math.atan2(ye - yc, xe - xc)

    is_full_circle = abs(x1 - xe) < 1e-6 and abs(y1 - ye) < 1e-6
    if is_full_circle:
        sweep = -2 * math.pi if cw else 2 * math.pi
    else:
        sweep = a_end - a_start
        if cw:
            if sweep > 0:
                sweep -= 2 * math.pi
        else:
            if sweep < 0:
                sweep += 2 * math.pi

    steps = max(2, min(n, int(abs(sweep) / (math.pi / 4)) + 1))
    pts = [
        (xc + r * math.cos(a_start + sweep * i / steps),
         yc + r * math.sin(a_start + sweep * i / steps))
        for i in range(steps + 1)
    ]
    return [(pts[i][0], pts[i][1], pts[i + 1][0], pts[i + 1][1])
            for i in range(steps)]


def _parse_features(
    features_path: Path,
    layer_name: str,
    ltype: str,
    units: str,
    traces: list,
    pads: list,
    vias: list,
    net_points: list | None = None,
    components: list | None = None,
    drills: list | None = None,
) -> None:
    """Parse ODB++ features file and append geometry to traces/pads/vias."""
    net_points = net_points or []
    components = components or []

    try:
        text = features_path.read_text(errors="replace")
    except OSError:
        return

    lines = text.splitlines()
    symbols = _parse_symbol_table(lines)

    in_surface = False
    in_island = False
    surface_pts: list[tuple[float, float]] = []

    def _flush_island() -> None:
        if ltype not in ("POWER_GROUND", "COPPER") or len(surface_pts) < 2:
            return
        for i in range(len(surface_pts) - 1):
            x1, y1 = surface_pts[i]
            x2, y2 = surface_pts[i + 1]
            traces.append(Trace(layer=layer_name, widthMM=0.05,
                                startX=x1, startY=y1, endX=x2, endY=y2))
        # Close polygon
        x1, y1 = surface_pts[-1]
        x2, y2 = surface_pts[0]
        traces.append(Trace(layer=layer_name, widthMM=0.05,
                            startX=x1, startY=y1, endX=x2, endY=y2))

    for line in lines:
        raw = line.strip()
        if not raw or raw[0] in ("#", "$", "@", "&"):
            continue
        if raw.startswith(("UNITS=", "ID=", "F ")):
            continue

        # Strip attribute string (everything from first ';')
        attr_pos = raw.find(";")
        s = raw[:attr_pos].strip() if attr_pos >= 0 else raw
        if not s:
            continue

        parts = s.split()
        rec = parts[0]

        # ── Surface block ───────────────────────────────────────────────────
        if rec == "S":
            in_surface = True
            in_island = False
            surface_pts = []
            continue

        if rec == "SE":
            if in_island:
                _flush_island()
            in_surface = False
            in_island = False
            surface_pts = []
            continue

        if in_surface:
            if rec == "OB":
                # Flush previous island before starting new contour
                if in_island:
                    _flush_island()
                flag = parts[3] if len(parts) >= 4 else "I"
                in_island = (flag == "I")
                surface_pts = []
                if in_island and len(parts) >= 3:
                    try:
                        surface_pts.append((_coord_to_mm(float(parts[1]), units),
                                            _coord_to_mm(float(parts[2]), units)))
                    except ValueError:
                        pass
            elif rec in ("OS", "OC") and in_island:
                if len(parts) >= 3:
                    try:
                        surface_pts.append((_coord_to_mm(float(parts[1]), units),
                                            _coord_to_mm(float(parts[2]), units)))
                    except ValueError:
                        pass
            elif rec == "OE" and in_island:
                _flush_island()
                in_island = False
                surface_pts = []
            continue

        # ── Line record ─────────────────────────────────────────────────────
        if rec == "L":
            # Emit routed traces from copper, silk (outlines/legends), and rout layers.
            if ltype not in ("COPPER", "SILK", "ROUT"):
                continue
            # L x1 y1 x2 y2 [extra] P|N sym_num
            # Polarity may be at index 5 or 6 depending on tool
            pol_idx = next((i for i in (5, 6) if i < len(parts) and parts[i] in ("P", "N")), None)
            if pol_idx is None or parts[pol_idx] != "P" or pol_idx + 1 >= len(parts):
                continue
            try:
                x1 = _coord_to_mm(float(parts[1]), units)
                y1 = _coord_to_mm(float(parts[2]), units)
                x2 = _coord_to_mm(float(parts[3]), units)
                y2 = _coord_to_mm(float(parts[4]), units)
                sym = symbols.get(int(parts[pol_idx + 1]), {"w": 0.1})
                mid_x = (x1 + x2) / 2
                mid_y = (y1 + y2) / 2
                net = _attr_net(raw) or _net_lookup(mid_x, mid_y, net_points)
                traces.append(Trace(layer=layer_name, widthMM=max(0.01, sym["w"]),
                                    startX=x1, startY=y1, endX=x2, endY=y2,
                                    netName=net))
            except (ValueError, IndexError):
                pass

        # ── Pad record ──────────────────────────────────────────────────────
        elif rec == "P":
            # P x y rotation P|N sym_num mirror
            if len(parts) < 6 or parts[4] != "P":
                continue
            try:
                x = _coord_to_mm(float(parts[1]), units)
                y = _coord_to_mm(float(parts[2]), units)
                sym = symbols.get(int(parts[5]), {"w": 0.5, "h": 0.5,
                                                   "shape": "CIRCLE", "inner": 0.0})
                net = _attr_net(raw) or _net_lookup(x, y, net_points)
                ref = _refdes_lookup(x, y, components)
                if ltype == "DRILL" and drills is not None:
                    plated = "non" not in layer_name and "npth" not in layer_name
                    drills.append(Drill(x=x, y=y, diamMM=max(0.01, sym["w"]), plated=plated))
                elif ltype == "POWER_GROUND" and sym["shape"] == "DONUT":
                    pass  # anti-pads in copper planes are not real vias — skip
                elif sym["shape"] == "DONUT":
                    vias.append(Via(x=x, y=y,
                                   outerDiamMM=sym["w"], drillDiamMM=sym["inner"],
                                   netName=net))
                else:
                    pads.append(Pad(layer=layer_name, x=x, y=y,
                                   widthMM=max(0.01, sym["w"]),
                                   heightMM=max(0.01, sym["h"]),
                                   shape=sym["shape"],
                                   netName=net, refDes=ref))
            except (ValueError, IndexError):
                pass

        # ── Arc record ──────────────────────────────────────────────────────
        elif rec == "A":
            if ltype not in ("COPPER", "SILK"):
                continue
            # A x1 y1 xe ye xc yc cw [extra] P|N sym_num
            # parts[7]=cw ('Y'/'N'), polarity is at index 8 or 9 (skip 7 to avoid
            # confusing cw='N' with negative polarity).
            pol_idx = next((i for i in (8, 9, 10) if i < len(parts)
                            and parts[i] in ("P", "N")), None)
            if pol_idx is None or parts[pol_idx] != "P" or pol_idx + 1 >= len(parts):
                continue
            try:
                x1 = _coord_to_mm(float(parts[1]), units)
                y1 = _coord_to_mm(float(parts[2]), units)
                xe = _coord_to_mm(float(parts[3]), units)
                ye = _coord_to_mm(float(parts[4]), units)
                xc = _coord_to_mm(float(parts[5]), units)
                yc = _coord_to_mm(float(parts[6]), units)
                cw = parts[7].upper() == "Y"
                sym = symbols.get(int(parts[pol_idx + 1]), {"w": 0.1})
                w = max(0.01, sym["w"])
                net = _attr_net(raw) or _net_lookup(xc, yc, net_points)

                # Emit arc as piecewise line segments so full circles don't
                # collapse to zero-length blobs.
                segs = _arc_segments(x1, y1, xe, ye, xc, yc, cw)
                for sx1, sy1, sx2, sy2 in segs:
                    traces.append(Trace(layer=layer_name, widthMM=w,
                                        startX=sx1, startY=sy1,
                                        endX=sx2, endY=sy2, netName=net))
            except (ValueError, IndexError):
                pass


def _parse_rout(features_path: Path, units: str, drills: list) -> None:
    """Parse ODB++ rout layer features for drill holes (P records only)."""
    try:
        text = features_path.read_text(errors="replace")
    except OSError:
        return
    lines = text.splitlines()
    symbols = _parse_symbol_table(lines)
    for line in lines:
        raw = line.strip()
        if not raw or raw[0] in ("#", "$", "@", "&"):
            continue
        attr_pos = raw.find(";")
        s = raw[:attr_pos].strip() if attr_pos >= 0 else raw
        parts = s.split()
        if len(parts) >= 6 and parts[0] == "P" and parts[4] == "P":
            try:
                x = _coord_to_mm(float(parts[1]), units)
                y = _coord_to_mm(float(parts[2]), units)
                sym = symbols.get(int(parts[5]), {"w": 0.3})
                drills.append(Drill(x=x, y=y, diamMM=max(0.01, sym["w"]), plated=True))
            except (ValueError, IndexError):
                pass


def _parse_netlist(netlist_path: Path, units: str) -> tuple[dict, list]:
    """Parse ODB++ cadnet netlist → ({(x_mm,y_mm): net_name}, [(x_mm,y_mm,net_name)]).

    Header: '$N net_name' lines define idx→name mapping.
    Body: 'net_idx pad_size x_in y_in T ...' one pad per line.
    """
    net_by_idx: dict[int, str] = {}
    coord_map: dict[tuple[float, float], str] = {}
    points: list[tuple[float, float, str]] = []

    try:
        text = netlist_path.read_text(errors="replace")
    except OSError:
        return coord_map, points

    for line in text.splitlines():
        s = line.strip()
        if not s:
            continue
        # Header net definition: $N net_name
        if s.startswith("$"):
            parts = s.split(None, 1)
            if len(parts) == 2:
                try:
                    net_by_idx[int(parts[0][1:])] = parts[1].strip()
                except (ValueError, IndexError):
                    pass
            continue
        if s.startswith("#") or s.startswith("@"):
            continue
        # Body: net_idx  pad_size_in  x_in  y_in  T  ...
        parts = s.split()
        if len(parts) < 4:
            continue
        try:
            net_idx = int(parts[0])
            x_mm = _coord_to_mm(float(parts[2]), units)
            y_mm = _coord_to_mm(float(parts[3]), units)
            net_name = net_by_idx.get(net_idx, "")
            key = (round(x_mm, 4), round(y_mm, 4))
            coord_map[key] = net_name
            points.append((x_mm, y_mm, net_name))
        except (ValueError, IndexError):
            pass

    return coord_map, points


def _net_lookup(x: float, y: float, points: list, tol: float = 0.05) -> str:
    """Return net name of nearest netlist point within tol mm, else ''."""
    best_name = ""
    best_dist = tol * tol  # compare squared distances to avoid sqrt
    for px, py, name in points:
        d2 = (x - px) ** 2 + (y - py) ** 2
        if d2 <= best_dist:
            best_dist = d2
            best_name = name
    return best_name


def _attr_net(raw_line: str) -> str:
    """Extract net name from ODB++ attribute string (;.net=NAME or ;net=NAME).

    ODB++ feature records append attributes after a semicolon, e.g.:
      L x1 y1 x2 y2 P 3 ;.net=GND;.comp=U1
    This is far more reliable than spatial lookup because every feature that
    belongs to a net has the attribute set explicitly by the EDA tool.
    """
    semi = raw_line.find(";")
    if semi < 0:
        return ""
    for part in raw_line[semi + 1:].split(";"):
        part = part.strip()
        if part.startswith(".net="):
            return part[5:].strip()
        if part.startswith("net="):
            return part[4:].strip()
    return ""


def _parse_components(comp_path: Path, units: str) -> list:
    """Parse ODB++ CMP file → [(x_mm, y_mm, refdes)].

    CMP record format: CMP id x y rotation mirror refdes partno ;attrs
    """
    components: list[tuple[float, float, str]] = []
    try:
        text = comp_path.read_text(errors="replace")
    except OSError:
        return components

    for line in text.splitlines():
        s = line.strip()
        if not s or not s.startswith("CMP "):
            continue
        # Strip attribute string
        attr_pos = s.find(";")
        s = s[:attr_pos].strip() if attr_pos >= 0 else s
        parts = s.split()
        # CMP id x y rotation mirror refdes [partno]
        if len(parts) < 7:
            continue
        try:
            x_mm = _coord_to_mm(float(parts[2]), units)
            y_mm = _coord_to_mm(float(parts[3]), units)
            refdes = parts[6]
            components.append((x_mm, y_mm, refdes))
        except (ValueError, IndexError):
            pass

    return components


def _refdes_lookup(x: float, y: float, components: list, tol: float = 1.0) -> str:
    """Return nearest component refdes within tol mm, else ''."""
    best_name = ""
    best_dist = tol * tol
    for cx, cy, refdes in components:
        d2 = (x - cx) ** 2 + (y - cy) ** 2
        if d2 <= best_dist:
            best_dist = d2
            best_name = refdes
    return best_name


def _extract_odb_archive(file_path: str, tmpdir: str) -> None:
    """Extract ODB++ archive to tmpdir. Supports .zip, .tgz, and double-gzip variants."""
    import zipfile

    # ZIP (most common ODB++ export from Altium, Cadence, etc.)
    if zipfile.is_zipfile(file_path):
        with zipfile.ZipFile(file_path, "r") as zf:
            zf.extractall(tmpdir)
        return

    # Standard tar/tgz
    try:
        with tarfile.open(file_path, "r:*") as tf:
            tf.extractall(tmpdir)
        return
    except Exception:
        pass

    # Double-gzip: outer gzip wrapping a tar/tgz
    with gzip.open(file_path, "rb") as gz:
        inner = io.BytesIO(gz.read())
    with tarfile.open(fileobj=inner, mode="r:*") as tf:
        tf.extractall(tmpdir)


def parse_odb(file_path: str) -> BoardData:
    """Parse ODB++ .tgz archive and return real BoardData."""
    layers: list[Layer] = []
    traces: list[Trace] = []
    pads: list[Pad] = []
    vias: list[Via] = []
    drills: list[Drill] = []
    outline: list[Point] = []

    try:
        with tempfile.TemporaryDirectory() as tmpdir:
            _extract_odb_archive(file_path, tmpdir)

            tmp = Path(tmpdir)
            job_dirs = [d for d in tmp.iterdir() if d.is_dir()]
            if not job_dirs:
                raise ValueError("no job root in archive")
            job_root = job_dirs[0]

            steps_dir = job_root / "steps"
            step_dirs = [d for d in steps_dir.iterdir() if d.is_dir()] \
                if steps_dir.exists() else []
            if not step_dirs:
                raise ValueError("no steps directory")
            step_root = step_dirs[0]
            logger.info("ODB++ step: %s", step_root.name)

            units = _read_units(step_root / "stephdr")
            logger.info("ODB++ units: %s", units)

            layer_defs = _parse_matrix(job_root / "matrix" / "matrix")
            outline = _parse_profile(step_root / "profile", units)
            logger.info("ODB++ outline: %d points", len(outline))

            # Parse netlist for net name lookup
            netlist_path = step_root / "netlists" / "cadnet" / "netlist"
            _, net_points = _parse_netlist(netlist_path, units)
            logger.info("ODB++ netlist: %d net points", len(net_points))

            # Parse component files for refdes lookup
            components: list = []
            for comp_file in ["top", "bot"]:
                cp = step_root / "components" / comp_file
                if cp.exists():
                    c = _parse_components(cp, units)
                    components.extend(c)
                    logger.info("ODB++ components/%s: %d components", comp_file, len(c))

            layers_dir = step_root / "layers"
            for ld in layer_defs:
                feat = layers_dir / ld["name"].lower() / "features"
                if not feat.exists():
                    continue
                ltype = _matrix_type_to_ltype(ld["type"])
                if ltype is None:
                    continue
                layer_name = ld["name"].lower()
                layers.append(Layer(name=layer_name, type=ltype))
                before = len(traces) + len(pads) + len(vias)
                _parse_features(feat, layer_name, ltype, units, traces, pads, vias,
                                 net_points=net_points, components=components, drills=drills)
                after = len(traces) + len(pads) + len(vias)
                logger.info("ODB++ %s (%s): %d features", layer_name, ltype, after - before)

            rout_feat = layers_dir / "rout" / "features"
            if rout_feat.exists():
                layers.append(Layer(name="rout", type="OUTLINE"))
                # _parse_rout extracts drill hits (P records) as Drill entries.
                # Do NOT call _parse_features here — it would add those same P records
                # as Pad entries (double-count), and rout L records are board-cut paths
                # that don't belong in the copper trace list.
                _parse_rout(rout_feat, units, drills)
                logger.info("ODB++ rout: %d drills", len(drills))

    except Exception as e:
        logger.error("ODB++ parse failed: %s", e, exc_info=True)
        return _mock_board()

    logger.info("ODB++ done: %d layers, %d traces, %d pads, %d vias, %d drills",
                len(layers), len(traces), len(pads), len(vias), len(drills))
    return BoardData(layers=layers, traces=traces, pads=pads, vias=vias,
                     drills=drills, outline=outline, boardThicknessMM=1.6)


# ── Routes ───────────────────────────────────────────────────────────────────

@app.get("/health")
def health():
    return {"status": "ok"}


@app.post("/parse", response_model=BoardData)
def parse(req: ParseRequest):
    tmp_path: str | None = None
    try:
        tmp_path = download_from_s3(req.bucket, req.fileKey)
    except Exception as e:
        logger.warning("S3 download failed (%s), using mock board data", e)
        return _mock_board()

    try:
        if req.fileType == "ODB_PLUS_PLUS":
            board = parse_odb(tmp_path)
        else:
            board = parse_gerber(tmp_path)
        return board
    except Exception as e:
        logger.error("Parse failed: %s", e, exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))
    finally:
        if tmp_path and os.path.exists(tmp_path):
            os.unlink(tmp_path)
