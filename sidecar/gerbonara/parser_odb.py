from __future__ import annotations

import gzip
import io
import logging
import math
import re
import tarfile
import tempfile
import zipfile
from pathlib import Path

from models import BoardData, Layer, Trace, Pad, Via, Drill, Point, Polygon, Component
from units import _coord_to_mm, _sym_to_mm

logger = logging.getLogger(__name__)


# ── Symbol parsing ─────────────────────────────────────────────────────────────

def _parse_sym(
    sym: str,
    units: str = "INCH",
    warnings: list[str] | None = None,
    layer_name: str = "",
    custom_syms: dict[str, dict] | None = None,
) -> dict:
    """Parse ODB++ symbol name into shape dict.

    custom_syms is a name → shape dict pre-loaded from the job's symbols/
    directory; used when the heuristic name parsing falls through (e.g.
    `special_<vendor>_*` symbols that have no encoded dimensions).
    """
    tokens = sym.strip().split()
    if not tokens:
        return {"shape": "CIRCLE", "w": 0.1, "h": 0.1, "inner": 0.0}
    sym = tokens[0]
    s = sym.lower()
    if custom_syms is not None:
        cs = custom_syms.get(sym) or custom_syms.get(s)
        if cs is not None:
            return cs
    try:
        if s.startswith("donut_r"):
            rest = s[7:]
            parts = rest.split("x", 1)
            raw_outer = float(parts[0])
            raw_inner = float(parts[1]) if len(parts) > 1 else raw_outer * 0.5
            outer = _sym_to_mm(raw_outer, units)
            inner = _sym_to_mm(raw_inner, units)
            inner = min(inner, outer * 0.85)
            return {"shape": "DONUT", "w": outer, "h": outer, "inner": inner}
        if s.startswith("chamf_rect"):
            rest = s[10:]
            dims = rest.split("x")
            try:
                raw_w = float(dims[0])
                h_raw = dims[1] if len(dims) > 1 else dims[0]
                h_raw = h_raw.lstrip("rc") or dims[0]
                raw_h = float(h_raw)
                w = _sym_to_mm(raw_w, units)
                h = _sym_to_mm(raw_h, units)
                return {"shape": "RECT", "w": w, "h": h, "inner": 0.0}
            except (ValueError, IndexError):
                pass
        if s.startswith("rect"):
            dims = s[4:].split("x")
            raw_w = float(dims[0])
            h_raw = dims[1] if len(dims) > 1 else dims[0]
            h_raw = h_raw.lstrip("r") or dims[0]
            raw_h = float(h_raw)
            w = _sym_to_mm(raw_w, units)
            h = _sym_to_mm(raw_h, units)
            return {"shape": "RECT", "w": w, "h": h, "inner": 0.0}
        if s.startswith("oval"):
            parts = s[4:].split("x")
            raw_w = float(parts[0])
            raw_h = float(parts[1]) if len(parts) > 1 else raw_w
            w = _sym_to_mm(raw_w, units)
            h = _sym_to_mm(raw_h, units)
            return {"shape": "OVAL", "w": w, "h": h, "inner": 0.0}
        if s.startswith("s") and len(s) > 1 and (s[1].isdigit() or s[1] == "."):
            raw_d = float(s[1:].split("x")[0])
            d = _sym_to_mm(raw_d, units)
            return {"shape": "RECT", "w": d, "h": d, "inner": 0.0}
        if s.startswith("r") and len(s) > 1 and (s[1].isdigit() or s[1] == "."):
            raw_d = float(s[1:].split("x")[0])
            d = _sym_to_mm(raw_d, units)
            return {"shape": "CIRCLE", "w": d, "h": d, "inner": 0.0}
        if s.startswith("moire"):
            return {"shape": "CIRCLE", "w": 1.0, "h": 1.0, "inner": 0.0}
        if s.startswith("thermal"):
            rest = s[7:]
            try:
                raw_d = float(rest.split("x")[0]) if rest else 0
                if raw_d > 0:
                    d = _sym_to_mm(raw_d, units)
                    return {"shape": "CIRCLE", "w": d, "h": d, "inner": 0.0}
            except ValueError:
                pass
            return {"shape": "CIRCLE", "w": 1.0, "h": 1.0, "inner": 0.0}
        parts = s.split("_")
        if len(parts) >= 2:
            last = parts[-1]
            if "x" in last:
                dim_parts = last.split("x", 1)
                try:
                    raw_w = float(dim_parts[0])
                    raw_h = float(dim_parts[1])
                    w = _sym_to_mm(raw_w, units)
                    h = _sym_to_mm(raw_h, units)
                    if w > 0 and h > 0:
                        return {"shape": "RECT", "w": w, "h": h, "inner": 0.0}
                except (ValueError, IndexError):
                    pass
            else:
                try:
                    raw_d = float(last)
                    d = _sym_to_mm(raw_d, units)
                    if d > 0:
                        return {"shape": "CIRCLE", "w": d, "h": d, "inner": 0.0}
                except ValueError:
                    pass
    except (ValueError, IndexError):
        pass
    logger.debug("Unknown symbol %r — using 0.1 mm circle fallback", sym)
    if warnings is not None:
        warnings.append(f"Layer {layer_name!r}: unknown symbol {sym!r} → 0.1mm fallback")
    return {"shape": "CIRCLE", "w": 0.1, "h": 0.1, "inner": 0.0}


def _parse_symbol_table(lines: list[str], units: str = "INCH",
                         warnings: list[str] | None = None,
                         layer_name: str = "",
                         custom_syms: dict[str, dict] | None = None) -> dict[int, dict]:
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
            symbols[int(parts[0][1:])] = _parse_sym(parts[1], units,
                                                     warnings=warnings,
                                                     layer_name=layer_name,
                                                     custom_syms=custom_syms)
        except (ValueError, IndexError):
            pass
    return symbols


# ── Matrix / profile parsing ───────────────────────────────────────────────────

def _parse_units_decl(stripped_line: str) -> str | None:
    """Return 'MM' / 'INCH' if the line is an ODB++ units declaration, else None.

    ODB++ accepts two equivalent forms in stephdr, profile, and feature files:
        UNITS=MM   (or =INCH)   — older, more common
        U MM       (or U INCH)  — Mentor / Cadence variant

    Misreading this defaults a file to INCH and silently inflates every
    coordinate / symbol diameter by 25.4× — a `U MM` board reads as 6mm /
    25mm / 127mm "drills" when the ODB++ source actually says 0.24 / 1.0 /
    5.0mm. Used by every reader that needs to self-resolve a file's units
    (stephdr, profile, feature files, custom-symbol scans).
    """
    if stripped_line.startswith("UNITS="):
        token = stripped_line.split("=", 1)[1].strip().upper()
    elif stripped_line.startswith("U "):
        token = stripped_line[2:].strip().upper()
    else:
        return None
    return token if token in ("MM", "INCH") else None


def _read_units(path: Path) -> str:
    """Read UNITS from ODB++ step header (stephdr). Defaults to INCH."""
    try:
        for line in path.read_text(errors="replace").splitlines():
            unit = _parse_units_decl(line.strip())
            if unit is not None:
                return unit
    except OSError:
        pass
    return "INCH"


def _parse_matrix(matrix_path: Path) -> list[dict]:
    """Parse ODB++ matrix/matrix → [{name, type, row, start, end}] sorted by row.

    For drill layers the matrix encodes the layer span via START_NAME/END_NAME
    (e.g. D_1_10 has START_NAME=SIGNAL_1, END_NAME=SIGNAL_10). We surface those
    so the viewer can show a drill whenever any of the copper layers it passes
    through is visible — physically a through-hole IS visible from every
    layer it intersects, not only when its own drill-layer toggle is on.
    """
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
                                   "row": int(current.get("ROW", 0)),
                                   "start": current.get("START_NAME", ""),
                                   "end": current.get("END_NAME", "")})
                except (ValueError, KeyError):
                    pass
            in_layer = False
        elif in_layer and "=" in s:
            k, _, v = s.partition("=")
            current[k.strip()] = v.strip()
    return sorted(layers, key=lambda x: x["row"])


def _layer_side(layer_name: str) -> str | None:
    """Classify a layer as belonging to the top or bottom stack.

    Used to disambiguate the spatial refdes lookup: a pad on a top-stack
    layer must only be matched against top-side components, otherwise the
    top-side pins of a chip can be wrongly attributed to a bottom-side
    passive sitting directly beneath it (or vice versa).

    Returns "top", "bot", or None when the layer side cannot be determined
    from the name (callers fall back to the legacy unfiltered lookup).
    """
    n = layer_name.lower()
    # Order matters: check "bot"/"bottom" before "top" so names like
    # "bottom" don't accidentally match a substring rule.
    if "bot" in n or "btm" in n or "back" in n or "b.cu" in n:
        return "bot"
    if "top" in n or "t.cu" in n or "f.cu" in n or "front" in n:
        return "top"
    return None


def _matrix_type_to_ltype(mtype: str) -> str | None:
    """Map ODB++ matrix layer TYPE to our type string. Returns None to skip."""
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
    if m == "SOLDER_PASTE":
        return "SOLDER_PASTE"
    if m == "ROUT":
        return "ROUT"
    return None


def _parse_profile(profile_path: Path, units: str) -> tuple[list[Point], list[list[Point]]]:
    """Parse ODB++ profile file → (boundary_points, holes).

    boundary_points: outer island points (flag "I")
    holes: list of rings, one per "H" block

    OS entries are straight-segment endpoints. OC entries are arcs of the form
    `OC xe ye xc yc [Y|N]` — tessellated so curved edges (e.g. half-circle
    scallops) don't collapse to a chord and self-intersect the polygon.

    Self-resolves units from the profile file's own header — boards exported
    by Mentor / Cadence-derived tools declare `U MM` directly in the profile
    file even when stephdr has no UNITS line at all. Without this the outline
    falls back to the INCH default, the bbox is read 25.4× too large, and the
    actual board features render as a tiny smudge in the corner of an
    enormous empty rectangle.
    """
    boundary: list[Point] = []
    holes: list[list[Point]] = []
    current_ring: list[Point] = []
    current_flag: str = "I"
    in_island = False
    last_xy: tuple[float, float] | None = None

    try:
        text = profile_path.read_text(errors="replace")
    except OSError:
        return boundary, holes

    lines = text.splitlines()
    for line in lines[:10]:
        unit = _parse_units_decl(line.strip())
        if unit is not None:
            units = unit
            break

    def to_mm(v: float) -> float:
        return _coord_to_mm(v, units)
    for line in lines:
        s = line.strip()
        if s.startswith("OB "):
            # flush previous ring if open
            if in_island and current_ring:
                if current_flag == "I":
                    boundary.extend(current_ring)
                elif current_flag == "H":
                    holes.append(current_ring)
            current_ring = []
            last_xy = None
            parts = s.split()
            current_flag = parts[3] if len(parts) >= 4 else "I"
            in_island = True
            if len(parts) >= 3:
                try:
                    x = to_mm(float(parts[1]))
                    y = to_mm(float(parts[2]))
                    current_ring.append(Point(x=x, y=y))
                    last_xy = (x, y)
                except ValueError:
                    pass
        elif s.startswith("OS ") and in_island:
            parts = s.split()
            if len(parts) >= 3:
                try:
                    x = to_mm(float(parts[1]))
                    y = to_mm(float(parts[2]))
                    current_ring.append(Point(x=x, y=y))
                    last_xy = (x, y)
                except ValueError:
                    pass
        elif s.startswith("OC ") and in_island and last_xy is not None:
            parts = s.split()
            if len(parts) >= 6:
                try:
                    xe = to_mm(float(parts[1]))
                    ye = to_mm(float(parts[2]))
                    xc = to_mm(float(parts[3]))
                    yc = to_mm(float(parts[4]))
                    cw = parts[5].upper() == "Y"
                    x1, y1 = last_xy
                    segs = _arc_segments(x1, y1, xe, ye, xc, yc, cw, n=24)
                    for _sx, _sy, ex, ey in segs:
                        current_ring.append(Point(x=ex, y=ey))
                    last_xy = (xe, ye)
                except ValueError:
                    pass
        elif s == "OE" and in_island:
            if current_ring:
                if current_flag == "I":
                    boundary.extend(current_ring)
                elif current_flag == "H":
                    holes.append(list(current_ring))
            current_ring = []
            last_xy = None
            in_island = False
    # flush any open ring at EOF
    if in_island and current_ring:
        if current_flag == "I":
            boundary.extend(current_ring)
        elif current_flag == "H":
            holes.append(current_ring)
    return boundary, holes


# ── Custom symbol geometry ────────────────────────────────────────────────────

def _scan_custom_symbol(features_path: Path, units: str) -> dict | None:
    """Compute a bounding-box shape from a `<job>/symbols/<name>/features` file.

    ODB++ "special" symbols (`special_*`, vendor-specific named shapes, etc.)
    encode their geometry as one or more positive surfaces (S P 0 ... SE)
    rather than encoding it in the name. The heuristic in `_parse_sym` can't
    size them, so without this they fall back to a 0.1mm circle and render
    as invisible specks. We pick the union bbox of all surface vertices and
    return it as a RECT — coarse but vastly better than the 0.1mm fallback.
    """
    try:
        text = features_path.read_text(errors="replace")
    except OSError:
        return None
    file_units = units
    for line in text.splitlines()[:20]:
        unit = _parse_units_decl(line.strip())
        if unit is not None:
            file_units = unit
            break
    xs: list[float] = []
    ys: list[float] = []
    last: tuple[float, float] | None = None
    for line in text.splitlines():
        s = line.strip()
        parts = s.split()
        if not parts:
            continue
        if parts[0] in ("OB", "OS") and len(parts) >= 3:
            try:
                x = _coord_to_mm(float(parts[1]), file_units)
                y = _coord_to_mm(float(parts[2]), file_units)
                xs.append(x); ys.append(y)
                last = (x, y)
            except ValueError:
                pass
        elif parts[0] == "OC" and len(parts) >= 6 and last is not None:
            try:
                xe = _coord_to_mm(float(parts[1]), file_units)
                ye = _coord_to_mm(float(parts[2]), file_units)
                xc = _coord_to_mm(float(parts[3]), file_units)
                yc = _coord_to_mm(float(parts[4]), file_units)
                cw = parts[5].upper() == "Y"
                x1, y1 = last
                # Tessellate so the bbox captures arc bulges, not just chords.
                for _sx, _sy, ex, ey in _arc_segments(x1, y1, xe, ye, xc, yc, cw, n=16):
                    xs.append(ex); ys.append(ey)
                last = (xe, ye)
            except ValueError:
                pass
    if not xs or not ys:
        return None
    w = max(xs) - min(xs)
    h = max(ys) - min(ys)
    if w <= 0 or h <= 0:
        return None
    return {"shape": "RECT", "w": w, "h": h, "inner": 0.0}


def _load_custom_symbols(symbols_root: Path, units: str) -> dict[str, dict]:
    """Pre-scan `<job>/symbols/<name>/features` for all custom symbols.

    Returns a name → shape dict keyed by both the original case and the
    lowercased form, since `_parse_sym` lowercases the input before matching.
    """
    out: dict[str, dict] = {}
    if not symbols_root.is_dir():
        return out
    for d in symbols_root.iterdir():
        if not d.is_dir():
            continue
        feat = d / "features"
        if not feat.exists():
            continue
        shape = _scan_custom_symbol(feat, units)
        if shape is not None:
            out[d.name] = shape
            out[d.name.lower()] = shape
    return out


# ── Arc approximation ─────────────────────────────────────────────────────────

def _arc_segments(
    x1: float, y1: float, xe: float, ye: float,
    xc: float, yc: float, cw: bool, n: int = 8,
) -> list[tuple[float, float, float, float]]:
    """Approximate an ODB++ arc as up to n line segments."""
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


# ── Via geometry extraction ───────────────────────────────────────────────────

_VIA_ROUND_RE = re.compile(r"D([0-9.]+)H([0-9.]+)", re.IGNORECASE)
_VIA_ALLEGRO_RE = re.compile(r"(?:microvia|via)([0-9.]+)_round([0-9.]+)", re.IGNORECASE)
# "VIA 0.5x0.25" (Altium-style, outer x hole).
_VIA_ALTIUM_RE = re.compile(r"VIA\s+([0-9.]+)\s*[xX]\s*([0-9.]+)", re.IGNORECASE)


def _attr_int_value(attrs_str: str, attr_idx: int) -> int | None:
    """Pull the integer value for attr index `attr_idx` from an ODB++ attrs
    suffix like `"0=10,1=1,2=0,4=335"`. Returns None if absent or malformed.
    """
    key = f"{attr_idx}="
    for segment in attrs_str.split(";"):
        for pair in segment.split(","):
            pair = pair.strip()
            if not pair.startswith(key):
                continue
            try:
                return int(pair[len(key):])
            except ValueError:
                return None
    return None


def _parse_attr_tables(lines: list[str]) -> tuple[dict[int, str], dict[int, str]]:
    """Parse @N attr_name and &N value_string tables from an ODB++ features file."""
    names: dict[int, str] = {}
    values: dict[int, str] = {}
    for line in lines:
        s = line.strip()
        parts = s.split(None, 1)
        if len(parts) < 2:
            continue
        try:
            if s.startswith("@"):
                names[int(parts[0][1:])] = parts[1].strip()
            elif s.startswith("&"):
                values[int(parts[0][1:])] = parts[1].strip()
        except (ValueError, IndexError):
            pass
    return names, values


def _via_geometry_mm(
    attr_str: str, attr_values: dict[int, str], units: str
) -> tuple[float, float] | None:
    """Extract (outer_mm, hole_mm) from a drill P record's attribute string."""
    for segment in attr_str.split(";"):
        for pair in segment.split(","):
            if "=" not in pair:
                continue
            try:
                _k_str, v_str = pair.strip().split("=", 1)
                value_text = attr_values.get(int(v_str), "")
                if not value_text:
                    continue
                m = _VIA_ROUND_RE.search(value_text)
                if m:
                    outer = _sym_to_mm(float(m.group(1)), units)
                    hole = _sym_to_mm(float(m.group(2)), units)
                    return (outer, hole)
                m2 = _VIA_ALLEGRO_RE.match(value_text)
                if m2:
                    return (float(m2.group(2)), float(m2.group(1)))
                m3 = re.match(r"hole([0-9.]+)_round([0-9.]+)_p", value_text, re.IGNORECASE)
                if m3:
                    return (float(m3.group(2)), float(m3.group(1)))
                m4 = _VIA_ALTIUM_RE.search(value_text)
                if m4:
                    outer = _sym_to_mm(float(m4.group(1)), units)
                    hole = _sym_to_mm(float(m4.group(2)), units)
                    return (outer, hole)
            except (ValueError, IndexError):
                pass
    return None


# ── Feature tokenizer & builder ───────────────────────────────────────────────

def _tokenize_features(lines: list[str]) -> list[dict]:
    """Convert raw feature file lines into a list of token dicts."""
    tokens = []
    for line in lines:
        raw = line.strip()
        if not raw or raw[0] in ("#", "@", "&"):
            tokens.append({"type": "skip", "parts": [], "raw": raw, "attrs": ""})
            continue
        if raw.startswith(("UNITS=", "ID=")):
            tokens.append({"type": "skip", "parts": [], "raw": raw, "attrs": ""})
            continue
        if raw.startswith("$"):
            tokens.append({"type": "$", "parts": raw.split(), "raw": raw, "attrs": ""})
            continue
        attr_pos = raw.find(";")
        attrs = raw[attr_pos + 1:].strip() if attr_pos >= 0 else ""
        s = raw[:attr_pos].strip() if attr_pos >= 0 else raw
        if not s:
            tokens.append({"type": "skip", "parts": [], "raw": raw, "attrs": attrs})
            continue
        parts = s.split()
        rec = parts[0]
        rec_type = rec if rec in ("L", "P", "A", "S", "SE", "OB", "OS", "OC", "OE", "F") else "skip"
        tokens.append({"type": rec_type, "parts": parts, "raw": raw, "attrs": attrs})
    return tokens


def _build_features(
    tokens: list[dict],
    layer_name: str,
    ltype: str,
    units: str,
    symbols: dict,
    net_points: list,
    components: list,
    drills: list | None,
    traces: list,
    pads: list,
    vias: list,
    warnings: list[str] | None = None,
    drill_attr_values: dict | None = None,
    polygons: list | None = None,
    attr_names: dict[int, str] | None = None,
    padstack_outer_mm: dict[int, float] | None = None,
    *,
    net_index: _NetIndex | None = None,
    refdes_index: _RefdesIndex | None = None,
) -> None:
    """Build geometry from a token list produced by _tokenize_features.

    `padstack_outer_mm` is an optional mutable map of `.padstack_id` → minimum
    copper outer diameter (mm) across copper layers. Populated on copper-layer
    passes and consulted on drill-layer passes to synthesize Via records when
    the regex-based attr parsing in `_via_geometry_mm` can't see explicit
    (outer, hole) dimensions. See the two-pass invocation in `parse_odb`.
    """
    if net_index is None and net_points:
        net_index = _NetIndex(net_points)
    if refdes_index is None and components:
        refdes_index = _RefdesIndex(components)

    # Find the attribute index for .pad_usage to detect fiducials
    _pad_usage_idx: int | None = None
    _padstack_id_idx: int | None = None
    if attr_names:
        for idx, name in attr_names.items():
            if name == ".pad_usage":
                _pad_usage_idx = idx
            elif name == ".padstack_id":
                _padstack_id_idx = idx
    # Pre-compute the side of this layer once — used to filter the refdes
    # spatial lookup so top-side features aren't attributed to bottom-side
    # components that happen to sit directly underneath (and vice versa).
    layer_side = _layer_side(layer_name)

    in_surface = False
    in_island = False
    island_flag: str = "I"  # "I" = outer island, "H" = hole
    surface_pts: list[tuple[float, float]] = []
    surface_sym_num: int = -1
    surface_net: str = ""
    current_polygon: Polygon | None = None  # outer island polygon being built
    current_holes: list[list[tuple[float, float]]] = []  # accumulated holes

    def _flush_island() -> None:
        nonlocal current_polygon
        if len(surface_pts) < 3:
            return
        if island_flag == "I":
            # A surface can have multiple outer islands; commit the previous one first
            if current_polygon is not None and polygons is not None:
                polygons.append(current_polygon)
                current_polygon = None
            # Outer island — start a new polygon
            if ltype in ("COPPER", "POWER_GROUND"):
                current_polygon = Polygon(
                    layer=layer_name,
                    points=[Point(x=x, y=y) for x, y in surface_pts],
                    netName=surface_net,
                )
            elif ltype == "SOLDER_MASK":
                xs = [p[0] for p in surface_pts]
                ys = [p[1] for p in surface_pts]
                cx = (min(xs) + max(xs)) / 2
                cy = (min(ys) + max(ys)) / 2
                w = max(0.01, max(xs) - min(xs))
                h = max(0.01, max(ys) - min(ys))
                pads.append(Pad(layer=layer_name, x=cx, y=cy,
                                widthMM=w, heightMM=h, shape="RECT",
                                netName="", refDes=""))
            elif ltype == "SILK":
                # Emit polygon boundary edges as individual traces rather than
                # an approximate RECT pad. The engine's silkscreen-on-pad rule
                # performs an exact capsule check on traces, so courtyard outlines
                # (e.g. octagon shapes whose bounding box contains a copper pad
                # but whose edges don't touch it) no longer produce false positives.
                n = len(surface_pts)
                for i in range(n):
                    a = surface_pts[i]
                    b = surface_pts[(i + 1) % n]
                    traces.append(Trace(
                        layer=layer_name, widthMM=0.12,
                        startX=a[0], startY=a[1],
                        endX=b[0], endY=b[1],
                    ))
        elif island_flag == "H":
            # Hole contour — attach to the current polygon (if any)
            if current_polygon is not None and ltype in ("COPPER", "POWER_GROUND"):
                current_polygon.holes.append([Point(x=x, y=y) for x, y in surface_pts])

    def _commit_polygon() -> None:
        """Emit the fully-built polygon (outer + holes) to the output list."""
        nonlocal current_polygon
        if current_polygon is not None and polygons is not None:
            polygons.append(current_polygon)
            current_polygon = None

    for token in tokens:
        if token["type"] in ("skip", "$"):
            continue

        parts = token["parts"]
        raw = token["raw"]
        rec = token["type"]

        if rec == "S":
            _commit_polygon()
            in_surface = True
            in_island = False
            island_flag = "I"
            surface_pts = []
            surface_net = _attr_net(raw)
            try:
                surface_sym_num = int(parts[2]) if len(parts) >= 3 else -1
            except (ValueError, IndexError):
                surface_sym_num = -1
            continue

        if rec == "SE":
            if in_island:
                _flush_island()
            _commit_polygon()
            in_surface = False
            in_island = False
            surface_pts = []
            continue

        if in_surface:
            if rec == "OB":
                if in_island:
                    _flush_island()
                flag = parts[3] if len(parts) >= 4 else "I"
                island_flag = flag
                in_island = True
                surface_pts = []
                if len(parts) >= 3:
                    try:
                        surface_pts.append((_coord_to_mm(float(parts[1]), units),
                                            _coord_to_mm(float(parts[2]), units)))
                    except ValueError:
                        pass
            elif rec == "OS" and in_island:
                if len(parts) >= 3:
                    try:
                        surface_pts.append((_coord_to_mm(float(parts[1]), units),
                                            _coord_to_mm(float(parts[2]), units)))
                    except ValueError:
                        pass
            elif rec == "OC" and in_island:
                # ODB++ arc: OC xe ye xc yc cw
                # xe/ye = end point, xc/yc = center, cw = Y/N clockwise.
                # Expand into line segments so circular anti-pads render
                # correctly instead of collapsing to a 2-point diamond.
                if len(parts) >= 6 and surface_pts:
                    try:
                        xe = _coord_to_mm(float(parts[1]), units)
                        ye = _coord_to_mm(float(parts[2]), units)
                        xc = _coord_to_mm(float(parts[3]), units)
                        yc = _coord_to_mm(float(parts[4]), units)
                        cw = parts[5].upper() == "Y"
                        x1, y1 = surface_pts[-1]  # arc starts from previous point
                        segs = _arc_segments(x1, y1, xe, ye, xc, yc, cw, n=16)
                        for _, _, sx2, sy2 in segs:
                            surface_pts.append((sx2, sy2))
                    except ValueError:
                        # Fallback: treat like a line segment
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

        if rec == "L":
            if ltype not in ("COPPER", "POWER_GROUND", "SILK", "ROUT"):
                continue
            pol_idx = next((i for i in (5, 6, 7) if i < len(parts) and parts[i] in ("P", "N")), None)
            if pol_idx is None or parts[pol_idx] != "P" or pol_idx + 1 >= len(parts):
                continue
            try:
                x1 = _coord_to_mm(float(parts[1]), units)
                y1 = _coord_to_mm(float(parts[2]), units)
                x2 = _coord_to_mm(float(parts[3]), units)
                y2 = _coord_to_mm(float(parts[4]), units)
                sym = symbols.get(int(parts[pol_idx + 1]), {"w": 0.1})
                trace_w = sym["w"]
                # Skip sub-minimum trace widths on copper layers. Below 0.05mm
                # (50µm) is not a manufacturable trace — it's a coordinate
                # marker, alignment reference, or fab-drawing feature that
                # ODB++ uses the L record for. Real copper traces start at
                # ~0.075mm (3 mil) on standard fabs, 0.05mm on premium.
                if ltype in ("COPPER", "POWER_GROUND") and trace_w < 0.05:
                    continue
                mid_x = (x1 + x2) / 2
                mid_y = (y1 + y2) / 2
                net = _attr_net(raw) or (net_index.lookup(mid_x, mid_y) if net_index else "")
                traces.append(Trace(layer=layer_name, widthMM=max(0.01, trace_w),
                                    startX=x1, startY=y1, endX=x2, endY=y2,
                                    netName=net))
            except (ValueError, IndexError):
                pass

        elif rec == "P":
            if len(parts) < 6 or parts[4] != "P":
                continue
            try:
                x = _coord_to_mm(float(parts[1]), units)
                y = _coord_to_mm(float(parts[2]), units)
                # ODB++ P record: P x y sym_num polarity dcode mirror rotation
                # parts[7] is rotation in degrees (0/90/180/270). For RECT and
                # OVAL pads, 90° and 270° rotations swap width and height.
                rotation = 0.0
                if len(parts) >= 8:
                    try:
                        rotation = float(parts[7])
                    except ValueError:
                        rotation = 0.0
                sym = symbols.get(int(parts[3]), {"w": 0.5, "h": 0.5,
                                                   "shape": "CIRCLE", "inner": 0.0})
                net = _attr_net(raw) or (net_index.lookup(x, y) if net_index else "")
                ref, pkg_class = refdes_index.lookup(x, y, layer_side) if refdes_index else ("", "")
                if ltype == "DRILL" and drills is not None:
                    plated = "non" not in layer_name.lower() and "npth" not in layer_name.lower()
                    hole_diam = max(0.01, sym["w"])
                    geom = None
                    if plated and drill_attr_values:
                        geom = _via_geometry_mm(token["attrs"], drill_attr_values, units)
                    if geom:
                        outer, attr_hole = geom
                        hole_diam = max(0.01, attr_hole)
                        if outer > hole_diam:
                            vias.append(Via(x=x, y=y, outerDiamMM=outer,
                                            drillDiamMM=hole_diam, netName=net,
                                            layer=layer_name))
                    elif plated and padstack_outer_mm and _padstack_id_idx is not None:
                        # Fallback: cross-reference .padstack_id against the
                        # outer-diameter map populated during the copper-layer
                        # pass. Dalsa (and most ODB++ exports that use symbolic
                        # .geometry values like "STANDARDVIA") don't carry
                        # numeric dimensions the regexes above can extract, but
                        # every via catch-pad on copper shares the same
                        # padstack_id integer with its drill record — that join
                        # gives us the outer diameter.
                        ps_id = _attr_int_value(token["attrs"], _padstack_id_idx)
                        if ps_id is not None:
                            outer = padstack_outer_mm.get(ps_id)
                            if outer is not None and outer > hole_diam:
                                vias.append(Via(x=x, y=y, outerDiamMM=outer,
                                                drillDiamMM=hole_diam, netName=net,
                                                layer=layer_name))
                    # Skip sub-minimum drill markers: features below 0.05mm
                    # (50µm) are pad markers or coordinate references, not
                    # real drill holes. The smallest laser-drilled microvia
                    # is ~0.05mm; mechanical drills start at ~0.1mm.
                    if hole_diam >= 0.05:
                        drills.append(Drill(x=x, y=y, diamMM=hole_diam, plated=plated,
                                            layer=layer_name))
                elif sym["shape"] == "DONUT" and ltype in ("COPPER", "POWER_GROUND"):
                    # A donut on a copper layer is the catch-pad of a through-hole
                    # via on that specific layer. Older revisions of this parser
                    # converted every donut into a Via, which produced one Via
                    # record per copper layer at the same (x, y) — N stacked
                    # rings on multi-layer boards, and a per-layer annular-ring
                    # rule that depended on the duplication for coverage. We
                    # now keep each layer's catch-pad as a per-layer Pad with
                    # shape="DONUT" so the renderer can filter by layer and the
                    # rule can check each layer's annular ring against the
                    # actual drill diameter (rule_annular_ring.go).
                    outer = sym["w"]
                    inner = sym["inner"]
                    pads.append(Pad(layer=layer_name, x=x, y=y,
                                   widthMM=max(0.01, outer),
                                   heightMM=max(0.01, outer),
                                   shape="DONUT",
                                   holeMM=max(0.0, inner),
                                   netName=net, refDes=ref,
                                   packageClass=pkg_class,
                                   isViaCatchPad=True))
                    # Populate padstack_outer_mm so the matching drill record
                    # (path 2 in the DRILL branch above) can synthesize a
                    # single Via with the correct outer diameter. Uses min
                    # across layers — same reasoning as the rect/oval path.
                    if (padstack_outer_mm is not None
                            and _padstack_id_idx is not None):
                        ps_id = _attr_int_value(token["attrs"], _padstack_id_idx)
                        if ps_id is not None:
                            prev = padstack_outer_mm.get(ps_id)
                            if prev is None or outer < prev:
                                padstack_outer_mm[ps_id] = outer
                elif sym["shape"] == "DONUT":
                    # Non-copper donut (mask opening, paste relief, etc.) — keep
                    # as Via; renderer paths for those layers don't differentiate
                    # and rules don't iterate them. Tag with the source layer
                    # so the painter's drill-layer-visibility filter still
                    # tracks visibility coherently when toggling layers.
                    vias.append(Via(x=x, y=y,
                                   outerDiamMM=sym["w"], drillDiamMM=sym["inner"],
                                   netName=net, layer=layer_name))
                else:
                    is_fid = False
                    if _pad_usage_idx is not None:
                        # Check attrs like ";0=2" where 0 is pad_usage idx, 2=g_fiducial, 3=l_fiducial
                        attrs_str = raw[raw.find(";"):] if ";" in raw else ""
                        for seg in attrs_str.split(";"):
                            seg = seg.strip()
                            if seg.startswith(f"{_pad_usage_idx}="):
                                val = seg.split("=", 1)[1].split(",")[0]
                                if val in ("2", "3"):  # g_fiducial or l_fiducial
                                    is_fid = True
                                break
                    # Apply rotation: 90° and 270° swap width/height for
                    # non-symmetric pads (RECT, OVAL). CIRCLE is invariant.
                    pw, ph = sym["w"], sym["h"]
                    if sym["shape"] in ("RECT", "OVAL") and rotation:
                        # Normalize to 0-360
                        r = rotation % 360
                        if abs(r - 90) < 1 or abs(r - 270) < 1:
                            pw, ph = ph, pw
                    pads.append(Pad(layer=layer_name, x=x, y=y,
                                   widthMM=max(0.01, pw),
                                   heightMM=max(0.01, ph),
                                   shape=sym["shape"],
                                   netName=net, refDes=ref,
                                   packageClass=pkg_class,
                                   isFiducial=is_fid))
                    # Capture padstack_id → min outer diameter seen across
                    # copper layers. Used during the drill pass to synthesize
                    # Via records when attr values don't carry numeric
                    # dimensions. We take min(w, h) for rect/oval catch-pads
                    # because annular-ring math is driven by the shorter
                    # dimension, and we keep the minimum across layers because
                    # inner-layer catch-pads are often smaller than the
                    # top/bottom cover pads on the same via.
                    if (padstack_outer_mm is not None
                            and _padstack_id_idx is not None
                            and ltype in ("COPPER", "POWER_GROUND")):
                        ps_id = _attr_int_value(token["attrs"], _padstack_id_idx)
                        if ps_id is not None:
                            od = min(pw, ph)
                            prev = padstack_outer_mm.get(ps_id)
                            if prev is None or od < prev:
                                padstack_outer_mm[ps_id] = od
            except (ValueError, IndexError):
                pass

        elif rec == "A":
            if ltype not in ("COPPER", "POWER_GROUND", "SILK"):
                continue
            # ODB++ A record: A xs ys xe ye xc yc sym pol dcode dir
            # sym is the symbol index immediately before the polarity flag, dir
            # is two slots after polarity (skipping the dcode). Reading either
            # field from the wrong slot produces silently broken output: the
            # direction read here mis-classifies clockwise (Y) arcs as ccw,
            # making near-tangent arcs sweep ~330° the wrong way and leaving
            # the trace looping far below the board outline.
            pol_idx = next((i for i in (8, 9, 10) if i < len(parts)
                            and parts[i] in ("P", "N")), None)
            if pol_idx is None or parts[pol_idx] != "P" or pol_idx + 2 >= len(parts):
                continue
            try:
                x1 = _coord_to_mm(float(parts[1]), units)
                y1 = _coord_to_mm(float(parts[2]), units)
                xe = _coord_to_mm(float(parts[3]), units)
                ye = _coord_to_mm(float(parts[4]), units)
                xc = _coord_to_mm(float(parts[5]), units)
                yc = _coord_to_mm(float(parts[6]), units)
                cw = parts[pol_idx + 2].upper() == "Y"
                sym = symbols.get(int(parts[pol_idx - 1]), {"w": 0.1})
                trace_w = sym["w"]
                # Skip sub-minimum arcs on copper — same rationale as L records.
                if ltype in ("COPPER", "POWER_GROUND") and trace_w < 0.05:
                    continue
                w = max(0.01, trace_w)
                net = _attr_net(raw) or (net_index.lookup(xc, yc) if net_index else "")
                segs = _arc_segments(x1, y1, xe, ye, xc, yc, cw)
                for sx1, sy1, sx2, sy2 in segs:
                    traces.append(Trace(layer=layer_name, widthMM=w,
                                        startX=sx1, startY=sy1,
                                        endX=sx2, endY=sy2, netName=net))
            except (ValueError, IndexError):
                pass

    if in_island:
        _flush_island()
    _commit_polygon()


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
    warnings: list[str] | None = None,
    polygons: list | None = None,
    padstack_outer_mm: dict[int, float] | None = None,
    custom_syms: dict[str, dict] | None = None,
) -> None:
    """Parse ODB++ features file and append geometry to traces/pads/vias/polygons."""
    net_points = net_points or []
    components = components or []

    try:
        text = features_path.read_text(errors="replace")
    except OSError:
        return

    lines = text.splitlines()
    units = _features_file_units(lines, units, layer_name, warnings)
    symbols = _parse_symbol_table(lines, units, warnings=warnings, layer_name=layer_name,
                                   custom_syms=custom_syms)
    tokens = _tokenize_features(lines)

    attr_names, attr_values = _parse_attr_tables(lines)

    _build_features(tokens, layer_name, ltype, units, symbols,
                    net_points, components, drills, traces, pads, vias,
                    warnings=warnings,
                    drill_attr_values=attr_values if ltype == "DRILL" else None,
                    attr_names=attr_names,
                    padstack_outer_mm=padstack_outer_mm,
                    polygons=polygons)


def _features_file_units(lines: list[str], step_units: str,
                         layer_name: str,
                         warnings: list[str] | None) -> str:
    """Resolve UNITS for a single feature file.

    ODB++ lets each feature file declare its own units line that overrides
    the step-level UNITS from stephdr. Real-world HDI designs sometimes mix
    units — e.g. every copper/mask layer is MM but the drill layer is INCH.
    Missing the override squashes all coordinates ~25× and mis-scales symbol
    diameters, which on the drill layer silently discards ~99% of drills
    via the sub-50µm marker filter in _build_features.

    Accepts both `UNITS=MM` and `U MM` syntaxes via `_parse_units_decl`.
    """
    file_units: str | None = None
    for line in lines[:10]:
        unit = _parse_units_decl(line.strip())
        if unit is not None:
            file_units = unit
            break
    if file_units is not None and file_units != step_units.upper():
        if warnings is not None:
            warnings.append(
                f"Layer {layer_name!r}: feature file UNITS={file_units} "
                f"overrides step-level UNITS={step_units}"
            )
        return file_units
    return step_units


def _parse_rout(features_path: Path, units: str, drills: list,
                layer_name: str = "rout") -> None:
    """Parse ODB++ rout layer features for drill holes (P records only)."""
    try:
        text = features_path.read_text(errors="replace")
    except OSError:
        return
    lines = text.splitlines()
    units = _features_file_units(lines, units, layer_name, None)
    symbols = _parse_symbol_table(lines, units)
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
                sym = symbols.get(int(parts[3]), {"w": 0.3})
                drills.append(Drill(x=x, y=y, diamMM=max(0.01, sym["w"]),
                                    plated=True, layer=layer_name))
            except (ValueError, IndexError):
                pass


# ── Netlist / component lookup ────────────────────────────────────────────────

def _parse_netlist(netlist_path: Path, units: str) -> tuple[dict, list]:
    """Parse ODB++ cadnet netlist."""
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


class _NetIndex:
    """Grid-based spatial index for fast net point lookups.

    Instead of scanning all net points for every feature (O(n*m)),
    points are hashed into grid cells so lookups only check a 3x3
    neighbourhood (effectively O(1) per query).
    """

    __slots__ = ("_grid", "_cell_size")

    def __init__(self, points: list, cell_size: float = 0.5) -> None:
        self._cell_size = cell_size
        self._grid: dict[tuple[int, int], list[tuple[float, float, str]]] = {}
        for px, py, name in points:
            key = (int(math.floor(px / cell_size)), int(math.floor(py / cell_size)))
            self._grid.setdefault(key, []).append((px, py, name))

    def lookup(self, x: float, y: float, tol: float = 0.05) -> str:
        """Return net name of nearest indexed point within *tol* mm, else ''."""
        cs = self._cell_size
        cx = int(math.floor(x / cs))
        cy = int(math.floor(y / cs))
        best_name = ""
        best_dist = tol * tol
        for dx in (-1, 0, 1):
            for dy in (-1, 0, 1):
                bucket = self._grid.get((cx + dx, cy + dy))
                if bucket is None:
                    continue
                for px, py, name in bucket:
                    d2 = (x - px) ** 2 + (y - py) ** 2
                    if d2 <= best_dist:
                        best_dist = d2
                        best_name = name
        return best_name


def _net_lookup(x: float, y: float, points: list, tol: float = 0.05) -> str:
    """Return net name of nearest netlist point within tol mm, else ''."""
    best_name = ""
    best_dist = tol * tol
    for px, py, name in points:
        d2 = (x - px) ** 2 + (y - py) ** 2
        if d2 <= best_dist:
            best_dist = d2
            best_name = name
    return best_name


def _point_in_ring(x: float, y: float, ring: list) -> bool:
    """Ray-casting point-in-polygon test. `ring` is a list of Point objects."""
    n = len(ring)
    if n < 3:
        return False
    inside = False
    j = n - 1
    for i in range(n):
        yi = ring[i].y
        yj = ring[j].y
        if (yi > y) != (yj > y):
            xi = ring[i].x
            xj = ring[j].x
            x_intersect = (xj - xi) * (y - yi) / ((yj - yi) or 1e-20) + xi
            if x < x_intersect:
                inside = not inside
        j = i
    return inside


def _point_in_polygon(x: float, y: float, poly) -> bool:
    """True if (x, y) lies inside the polygon's outer ring and outside all holes."""
    if not _point_in_ring(x, y, poly.points):
        return False
    for hole in poly.holes:
        if _point_in_ring(x, y, hole):
            return False
    return True


def _infer_polygon_nets(
    polygons: list,
    pads: list,
    traces: list,
    vias: list,
    net_points: list,
    warnings: list | None = None,
) -> None:
    """Fill in empty `netName` on copper pour polygons by majority vote of
    features that physically lie inside (or adjacent to) the polygon.

    Some ODB++ exports declare plane surfaces with an attribute block that
    has no `.net=` token (e.g. `S P 0;;ID=8400`), so `_attr_net` on the S
    record returns empty. Without a net on the polygon, the clearance rule's
    same-net skip can't match, and every thermal-relief via catch-pad gets
    flagged as "too close to the pour edge" — even though it's the designed
    anti-pad geometry.

    Inference is layered — each pass runs only on polygons still unlabeled
    after the previous pass. Earlier passes are stricter (inside-polygon
    containment); later passes relax to proximity as a last resort:

    1. **Same-layer pads inside the polygon.** Most effective on plane
       layers: thermal-relief catch-pads carry the plane's net via the
       netlist-to-pad association that `_build_features` already wires up.
    2. **Same-layer traces inside the polygon.** Covers small copper islands
       on signal layers where a trace passes through but no pad sits inside.
       Tallies each trace whose midpoint is inside the polygon.
    3. **Netlist points inside the polygon.** Uses the raw `(x, y, net_name)`
       records parsed from `netlists/cadnet/netlist`. Covers isolated islands
       that have no pads *or* traces inside — a netlist point that falls
       inside the polygon is strong evidence the island is on that net.
    4. **Nearest labeled via within 1 mm of the polygon centroid.** Fallback
       for slivers that contain no features at all (thermal-relief spokes,
       break-out copper adjacent to signal vias). These polygons are too
       small to contain anything inside their outer ring, but they sit
       immediately adjacent to a via whose net they almost certainly belong
       to. Scoped tightly (1 mm) so it can't pollute larger islands.

    Holes are respected in passes 1-3: a feature inside an anti-pad is on
    the *via's* net, not the pour's, and must not be counted.

    Split planes are handled correctly — each sub-polygon is labeled from
    the features inside *its own* boundary, so a board with a GND plane
    and a VCC_3V3 island on the same layer ends up with each sub-pour on
    the right net.
    """
    if not polygons:
        return
    # Group features by layer once.
    pads_by_layer: dict[str, list] = {}
    for p in pads:
        pads_by_layer.setdefault(p.layer, []).append(p)
    traces_by_layer: dict[str, list] = {}
    for t in traces:
        traces_by_layer.setdefault(t.layer, []).append(t)

    # For polygons with many holes (e.g. ground planes with thousands of
    # anti-pads), checking _point_in_polygon for every candidate feature is
    # O(features × hole_vertices) and can take minutes on large boards.
    # Threshold: if a polygon has more than 200 holes, skip hole checks and
    # rely on majority voting from the outer-ring test alone. A few anti-pad
    # pads will be counted (wrong net), but they're outnumbered by the real
    # thermal-relief pads on the plane's net, so majority still wins.
    _MAX_HOLES_FOR_EXACT = 200

    def _inside(poly, x: float, y: float) -> bool:
        if not _point_in_ring(x, y, poly.points):
            return False
        if len(poly.holes) > _MAX_HOLES_FOR_EXACT:
            return True  # skip hole check, rely on majority vote
        for hole in poly.holes:
            if _point_in_ring(x, y, hole):
                return False
        return True

    def _majority(tally: dict[str, int]) -> str:
        if not tally:
            return ""
        return max(tally.items(), key=lambda kv: kv[1])[0]

    inferred = {"pads": 0, "traces": 0, "netlist": 0, "nearest_via": 0}
    # Pass-4 tolerance: the sliver polygons we're catching here are on the
    # order of 0.3–0.5 mm, sitting immediately next to a via. 1 mm is large
    # enough to reach the adjacent via's center but small enough that we
    # can't accidentally label a larger island from an unrelated via across
    # the board.
    nearest_via_tol_mm = 1.0
    nearest_via_tol2 = nearest_via_tol_mm * nearest_via_tol_mm

    for poly in polygons:
        if poly.netName or not poly.points:
            continue

        # Bounding box filter — polygons can be huge, features usually aren't.
        xs = [pt.x for pt in poly.points]
        ys = [pt.y for pt in poly.points]
        minx, maxx = min(xs), max(xs)
        miny, maxy = min(ys), max(ys)

        # --- Pass 1: pads inside polygon ---
        tally: dict[str, int] = {}
        for p in pads_by_layer.get(poly.layer, []):
            if p.x < minx or p.x > maxx or p.y < miny or p.y > maxy:
                continue
            if not p.netName or p.netName == "$NONE$":
                continue
            if not _inside(poly, p.x, p.y):
                continue
            tally[p.netName] = tally.get(p.netName, 0) + 1
        if tally:
            poly.netName = _majority(tally)
            inferred["pads"] += 1
            continue

        # --- Pass 2: traces inside polygon (midpoint test) ---
        for t in traces_by_layer.get(poly.layer, []):
            if not t.netName or t.netName == "$NONE$":
                continue
            mx = (t.startX + t.endX) / 2
            my = (t.startY + t.endY) / 2
            if mx < minx or mx > maxx or my < miny or my > maxy:
                continue
            if not _inside(poly, mx, my):
                continue
            tally[t.netName] = tally.get(t.netName, 0) + 1
        if tally:
            poly.netName = _majority(tally)
            inferred["traces"] += 1
            continue

        # --- Pass 3: netlist points inside polygon ---
        # net_points is layer-independent; a netlist entry at (x, y) inside
        # this polygon's outer-minus-holes region is evidence of what net
        # passes through (and likely connects to) the island.
        for (nx, ny, nname) in net_points:
            if not nname or nname == "$NONE$":
                continue
            if nx < minx or nx > maxx or ny < miny or ny > maxy:
                continue
            if not _inside(poly, nx, ny):
                continue
            tally[nname] = tally.get(nname, 0) + 1
        if tally:
            poly.netName = _majority(tally)
            inferred["netlist"] += 1
            continue

        # --- Pass 4: nearest labeled via within 1 mm of centroid ---
        # Last-resort proximity fallback for thermal-relief slivers and
        # break-out copper. These tiny polygons contain no features inside
        # their own ring, but sit immediately next to a single via whose
        # net they almost certainly share.
        cx = sum(xs) / len(xs)
        cy = sum(ys) / len(ys)
        best_v_net = ""
        best_v_d2 = nearest_via_tol2
        for v in vias:
            if not v.netName or v.netName == "$NONE$":
                continue
            d2 = (v.x - cx) ** 2 + (v.y - cy) ** 2
            if d2 < best_v_d2:
                best_v_d2 = d2
                best_v_net = v.netName
        if best_v_net:
            poly.netName = best_v_net
            inferred["nearest_via"] += 1

    total = sum(inferred.values())
    unresolved = sum(1 for p in polygons if not p.netName)
    if total and warnings is not None:
        warnings.append(
            f"Inferred net name for {total} unlabeled copper pour polygon(s) "
            f"(pads:{inferred['pads']} traces:{inferred['traces']} "
            f"netlist:{inferred['netlist']} nearest_via:{inferred['nearest_via']})"
        )
    logger.info(
        "ODB++ polygon net inference: labeled %d "
        "(pads:%d traces:%d netlist:%d nearest_via:%d), %d still unlabeled",
        total, inferred["pads"], inferred["traces"],
        inferred["netlist"], inferred["nearest_via"], unresolved,
    )


def _propagate_trace_nets(
    traces: list, pads: list, layers: list,
    outline: list, warnings: list | None = None,
) -> None:
    """Fill in empty trace netName by walking connectivity from pads.

    Many ODB++ exports don't carry `.net=` attributes on L (trace) records
    and the netlist file is too sparse for midpoint lookups. This leaves
    most traces without a net, which breaks the clearance rule's same-net
    skip and produces thousands of false positives.

    Strategy:
    1. **Seed** — for each trace with an empty net, check if either endpoint
       is within tolerance of a pad that has a known net. If so, adopt it.
    2. **BFS propagation** — walk through connected trace chains via shared
       endpoints (spatial grid, 20 µm tolerance). Every trace reachable from
       a seeded trace without crossing a different-net pad gets the same net.

    Only empty nets are filled — traces that already have a net from the
    file are never overwritten. Only copper/power-ground layers are
    processed (silk, mask, paste, drill don't need net propagation).
    Out-of-board traces (fab drawing, panelization marks) are skipped to
    save memory on large boards.
    """
    if not traces:
        return

    TOL = 0.02  # 20 µm endpoint matching tolerance
    CELL = 0.1  # grid cell for endpoint spatial index

    # Only propagate on copper layers — silk/mask/paste/drill don't
    # participate in clearance checks, so spending memory on them is waste.
    copper_layer_names = {
        l.name for l in layers
        if l.type in ("COPPER", "POWER_GROUND")
    }

    # Compute board bbox for out-of-board filtering.
    if outline:
        o_xs = [p.x for p in outline]
        o_ys = [p.y for p in outline]
        o_min_x, o_max_x = min(o_xs), max(o_xs)
        o_min_y, o_max_y = min(o_ys), max(o_ys)
        BUF = 2.0  # same as clearance rule's panel buffer

        def _in_board(t) -> bool:
            mx = (t.startX + t.endX) / 2
            my = (t.startY + t.endY) / 2
            return (o_min_x - BUF <= mx <= o_max_x + BUF and
                    o_min_y - BUF <= my <= o_max_y + BUF)
    else:
        def _in_board(_t) -> bool:
            return True

    # Group traces by layer so propagation doesn't cross layers.
    from collections import defaultdict
    by_layer: dict[str, list[int]] = defaultdict(list)
    for i, t in enumerate(traces):
        if t.layer not in copper_layer_names:
            continue
        if not _in_board(t):
            continue
        by_layer[t.layer].append(i)

    # Group pads by layer.
    pads_by_layer: dict[str, list] = defaultdict(list)
    for p in pads:
        if p.netName and p.netName != "$NONE$":
            pads_by_layer[p.layer].append(p)

    total_seeded = 0
    total_propagated = 0

    for layer, trace_idxs in by_layer.items():
        layer_pads = pads_by_layer.get(layer, [])

        # Build spatial grid of trace endpoints for fast neighbour lookup.
        ep_grid: dict[tuple[int, int], list[int]] = defaultdict(list)
        for i in trace_idxs:
            t = traces[i]
            for x, y in ((t.startX, t.startY), (t.endX, t.endY)):
                ep_grid[(int(x / CELL), int(y / CELL))].append(i)

        def _touching(x: float, y: float, exclude: int) -> list[int]:
            """Trace indices with an endpoint within TOL of (x, y)."""
            gx, gy = int(x / CELL), int(y / CELL)
            result: list[int] = []
            tol2 = TOL * TOL
            for dx in (-1, 0, 1):
                for dy in (-1, 0, 1):
                    for j in ep_grid.get((gx + dx, gy + dy), ()):
                        if j == exclude:
                            continue
                        tj = traces[j]
                        if (tj.startX - x) ** 2 + (tj.startY - y) ** 2 <= tol2 or \
                           (tj.endX - x) ** 2 + (tj.endY - y) ** 2 <= tol2:
                            result.append(j)
            return result

        # Build spatial grid for pad lookup — avoids O(traces × pads) scan
        # that was taking minutes on 14-layer boards.
        PAD_CELL = 1.0  # 1mm cells; pads are typically < 5mm
        pad_grid: dict[tuple[int, int], list] = defaultdict(list)
        for p in layer_pads:
            pad_grid[(int(p.x / PAD_CELL), int(p.y / PAD_CELL))].append(p)

        def _pad_net_at(x: float, y: float) -> str:
            """Return the net of a pad whose edge is within TOL of (x, y)."""
            gx, gy = int(x / PAD_CELL), int(y / PAD_CELL)
            for ddx in (-1, 0, 1):
                for ddy in (-1, 0, 1):
                    for p in pad_grid.get((gx + ddx, gy + ddy), ()):
                        dx, dy = abs(x - p.x), abs(y - p.y)
                        hw, hh = p.widthMM / 2, p.heightMM / 2
                        if p.shape == "CIRCLE":
                            if (dx * dx + dy * dy) ** 0.5 - hw <= TOL:
                                return p.netName
                        else:
                            if max(dx - hw, dy - hh, 0) <= TOL:
                                return p.netName
            return ""

        # Phase 1: seed from pads.
        assigned: dict[int, str] = {}
        for i in trace_idxs:
            t = traces[i]
            if t.netName:
                assigned[i] = t.netName
                continue
            net = _pad_net_at(t.startX, t.startY) or _pad_net_at(t.endX, t.endY)
            if net:
                assigned[i] = net
                total_seeded += 1

        # Phase 2: BFS propagation through shared endpoints.
        queue = list(assigned.keys())
        visited = set(assigned.keys())
        while queue:
            i = queue.pop()
            net = assigned[i]
            t = traces[i]
            for x, y in ((t.startX, t.startY), (t.endX, t.endY)):
                for j in _touching(x, y, i):
                    if j in visited:
                        continue
                    visited.add(j)
                    assigned[j] = net
                    total_propagated += 1
                    queue.append(j)

        # Apply assignments.
        for i, net in assigned.items():
            if not traces[i].netName:
                traces[i].netName = net

    total = total_seeded + total_propagated
    if total and warnings is not None:
        warnings.append(
            f"Propagated net names to {total} traces "
            f"(seeded:{total_seeded} propagated:{total_propagated})"
        )
    logger.info(
        "ODB++ trace net propagation: labeled %d (seeded:%d propagated:%d)",
        total, total_seeded, total_propagated,
    )


def _attr_net(raw_line: str) -> str:
    """Extract net name from ODB++ attribute string."""
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


_PASSIVE_PACKAGE_SIZES = frozenset({
    "01005", "0201", "0402", "0603", "0805",
    "1206", "1210", "1812", "2010", "2512",
})


def _classify_package(part_name: str) -> str:
    """Extract passive package class (e.g. '0402') from an ODB++ part name.

    Returns empty string if the part cannot be classified.
    """
    if not part_name:
        return ""
    # Match 5-digit code first (01005), then 4-digit codes like 0402, 0805
    m5 = re.search(r"(?:^|[_\-])(01005)(?:[_\-]|$)", part_name)
    if m5:
        return "01005"
    m = re.search(r"(?:^|[_\-])(\d{4})(?:[_\-]|$)", part_name)
    if m and m.group(1) in _PASSIVE_PACKAGE_SIZES:
        return m.group(1)
    # Try metric equivalents embedded in name (e.g. "1005Metric" → 0402)
    _metric_to_imperial = {
        "0402": "01005", "0603": "0201", "1005": "0402", "1608": "0603",
        "2012": "0805", "3216": "1206", "3225": "1210",
        "4532": "1812", "5025": "2010", "6332": "2512",
    }
    m2 = re.search(r"(\d{4})(?:Metric|metric|_metric)", part_name)
    if m2 and m2.group(1) in _metric_to_imperial:
        return _metric_to_imperial[m2.group(1)]
    return ""


def _parse_eda_packages(eda_path: Path, units: str) -> dict[int, dict]:
    """Parse eda/data PKG records → {pkg_index: {"name": str, "bbox_w_mm": float, "bbox_h_mm": float}}."""
    pkgs: dict[int, dict] = {}
    try:
        text = eda_path.read_text(errors="replace")
    except OSError:
        return pkgs

    pkg_idx = 0
    for line in text.splitlines():
        s = line.strip()
        if not s.startswith("PKG "):
            continue
        parts = s.rstrip(";").split()
        if len(parts) < 7:
            pkg_idx += 1
            continue
        try:
            name = parts[1]
            xmin = _coord_to_mm(float(parts[3]), units)
            ymin = _coord_to_mm(float(parts[4]), units)
            xmax = _coord_to_mm(float(parts[5]), units)
            ymax = _coord_to_mm(float(parts[6]), units)
            pkgs[pkg_idx] = {
                "name": name,
                "bbox_w_mm": abs(xmax - xmin),
                "bbox_h_mm": abs(ymax - ymin),
            }
        except (ValueError, IndexError):
            pass
        pkg_idx += 1

    return pkgs


def _classify_by_bbox(w_mm: float, h_mm: float) -> str:
    """Classify package by physical body dimensions (mm). Fallback when name parsing fails."""
    # Use the smaller dimension as width, larger as height (body size)
    lo, hi = min(w_mm, h_mm), max(w_mm, h_mm)
    # Approximate body dimensions for standard packages
    _body_ranges = [
        ("01005", 0.1, 0.25, 0.2, 0.45), # ~0.4 x 0.2 mm body
        ("0201", 0.2, 0.4, 0.4, 0.8),    # ~0.6 x 0.3 mm body
        ("0402", 0.6, 1.2, 0.3, 0.8),    # ~1.0 x 0.5 mm body
        ("0603", 1.2, 2.0, 0.6, 1.2),    # ~1.6 x 0.8 mm body
        ("0805", 1.6, 2.6, 1.0, 1.6),    # ~2.0 x 1.25 mm body
        ("1206", 2.8, 3.8, 1.2, 2.0),    # ~3.2 x 1.6 mm body
    ]
    for pkg, lo_min, lo_max, hi_min, hi_max in _body_ranges:
        if lo_min <= lo <= lo_max and hi_min <= hi <= hi_max:
            return pkg
    return ""


_MOUNT_TYPE_BY_INT: dict[str, str] = {
    "0": "other",
    "1": "smt",
    "2": "thmt",
    "3": "pressfit",
    "4": "manual",
}


def _parse_components(
    comp_path: Path, units: str, eda_pkgs: dict[int, dict] | None = None,
    side_hint: str | None = None,
) -> list[dict]:
    """Parse ODB++ CMP file into a list of component dicts.

    Each dict carries:
        x, y              — mm
        refDes, partName  — strings
        side              — "top" | "bot" | ""
        heightMM          — from `.comp_height` attr, 0.0 if not declared
        mountType         — "smt" | "thmt" | "pressfit" | "manual" | "other" | ""

    If eda_pkgs is provided, uses PKG name/bbox as fallback for classification.

    `side` is "top", "bot", or "" if unknown. Priority:
    1. `side_hint` from the caller (driven by the directory path, e.g.
       `components/top` or `layers/comp_+_bot`) — most reliable.
    2. The CMP record's mirror flag (parts[5]: "N" = not mirrored = top,
       "M" = mirrored = bottom) — fallback when the path is ambiguous.
    """
    eda_pkgs = eda_pkgs or {}
    components: list[dict] = []
    try:
        text = comp_path.read_text(errors="replace")
    except OSError:
        return components

    lines = text.splitlines()

    # Parse @N attribute-name table to find .comp_height and .comp_mount_type
    # indices. The CMP record's attr suffix uses integer keys that reference
    # these names; without the table we can't decode them.
    height_attr_idx: int | None = None
    mount_attr_idx: int | None = None
    for ln in lines:
        s = ln.strip()
        if s.startswith("@"):
            split = s.split(None, 1)
            if len(split) != 2:
                continue
            try:
                idx = int(split[0][1:])
            except ValueError:
                continue
            name = split[1].strip()
            if name == ".comp_height":
                height_attr_idx = idx
            elif name == ".comp_mount_type":
                mount_attr_idx = idx

    # Height values in the CMP attr are in file UNITS. Coord-style scaling
    # (inches → mm) applies because .comp_height denotes a physical length
    # in the same unit system as the board coords.
    height_scale = 25.4 if units.upper() == "INCH" else 1.0

    i = 0
    while i < len(lines):
        s = lines[i].strip()
        i += 1
        if not s or not s.startswith("CMP "):
            continue
        attr_pos = s.find(";")
        attr_str = s[attr_pos + 1:].strip() if attr_pos >= 0 else ""
        s_head = s[:attr_pos].strip() if attr_pos >= 0 else s
        parts = s_head.split()
        if len(parts) < 7:
            continue
        try:
            pkg_ref = int(parts[1])
            x_mm = _coord_to_mm(float(parts[2]), units)
            y_mm = _coord_to_mm(float(parts[3]), units)
            # parts[4] = rotation, parts[5] = mirror flag ("N" or "M")
            mirror = parts[5].upper() if len(parts) > 5 else ""
            refdes = parts[6]
            part_name = parts[7] if len(parts) > 7 else ""
            if side_hint in ("top", "bot"):
                side = side_hint
            elif mirror == "M":
                side = "bot"
            elif mirror == "N":
                side = "top"
            else:
                side = ""

            # Pull .comp_height and .comp_mount_type from the CMP's attr
            # suffix. Values look like `0=0.550000,1=1;ID=674464`; we only
            # need the k=v part before any trailing `;ID=...` tag.
            height_mm = 0.0
            mount_type = ""
            if height_attr_idx is not None or mount_attr_idx is not None:
                attr_payload = attr_str.split(";", 1)[0]
                for kv in attr_payload.split(","):
                    kv = kv.strip()
                    if "=" not in kv:
                        continue
                    k, v = kv.split("=", 1)
                    try:
                        ki = int(k.strip())
                    except ValueError:
                        continue
                    if ki == height_attr_idx:
                        try:
                            height_mm = float(v.strip()) * height_scale
                        except ValueError:
                            pass
                    elif ki == mount_attr_idx:
                        v = v.strip()
                        mount_type = _MOUNT_TYPE_BY_INT.get(v, v.lower())

            # Read PRP (property) lines that follow this CMP record
            prp_pkg = ""
            while i < len(lines):
                prp_line = lines[i].strip()
                if not prp_line.startswith("PRP "):
                    break
                i += 1
                # Fallback: Geometry.Height string property in mm or mil
                # (format `PRP Geometry.Height '1.2MM'`). Only read when the
                # numeric .comp_height attr was missing or zero.
                if height_mm <= 0 and "Geometry.Height" in prp_line:
                    q1 = prp_line.find("'"); q2 = prp_line.rfind("'")
                    if 0 <= q1 < q2:
                        raw = prp_line[q1+1:q2].strip().upper()
                        num_end = 0
                        while num_end < len(raw) and (raw[num_end].isdigit()
                                                      or raw[num_end] in ".-"):
                            num_end += 1
                        if num_end > 0:
                            try:
                                val = float(raw[:num_end])
                                unit_suffix = raw[num_end:].strip()
                                if unit_suffix in ("MIL", "MILS"):
                                    height_mm = val * 0.0254
                                else:  # default MM
                                    height_mm = val
                            except ValueError:
                                pass
                # Extract package class from common property names
                if not prp_pkg:
                    for prop_key in ("Imperial_Package_/_Case", "Case/Package"):
                        if prop_key in prp_line:
                            # Value is between single quotes: PRP Key 'Value'
                            q1 = prp_line.find("'")
                            q2 = prp_line.rfind("'")
                            if 0 <= q1 < q2:
                                val = prp_line[q1+1:q2].strip()
                                pkg = _classify_package(val)
                                if pkg:
                                    prp_pkg = pkg

            # Priority: PRP property > part_name > EDA PKG record > bbox
            if prp_pkg:
                part_name = f"{part_name}_{prp_pkg}" if part_name else prp_pkg
            elif not _classify_package(part_name) and pkg_ref in eda_pkgs:
                pkg_info = eda_pkgs[pkg_ref]
                pkg_from_name = _classify_package(pkg_info["name"])
                if pkg_from_name:
                    part_name = f"{part_name}_{pkg_from_name}" if part_name else pkg_from_name
                else:
                    pkg_from_bbox = _classify_by_bbox(
                        pkg_info["bbox_w_mm"], pkg_info["bbox_h_mm"])
                    if pkg_from_bbox:
                        part_name = f"{part_name}_{pkg_from_bbox}" if part_name else pkg_from_bbox

            components.append({
                "x": x_mm, "y": y_mm, "refDes": refdes, "partName": part_name,
                "side": side, "heightMM": height_mm, "mountType": mount_type,
            })
        except (ValueError, IndexError):
            pass

    return components


# Max pad-to-center distance by package class (mm).
# Derived from standard body sizes + pad overhang.
_PACKAGE_TOLERANCE: dict[str, float] = {
    "01005": 0.5, "0201": 0.6, "0402": 0.8, "0603": 1.2,
    "0805": 1.5, "1206": 2.2, "1210": 2.2, "1812": 3.0,
    "2010": 3.5, "2512": 4.0,
}
_DEFAULT_TOLERANCE = 3.0  # fallback for unclassified components


class _RefdesIndex:
    """Grid-based spatial index for fast component/refdes lookups."""

    __slots__ = ("_grid", "_cell_size")

    def __init__(self, components: list, cell_size: float = 10.0) -> None:
        self._cell_size = cell_size
        self._grid: dict[tuple[int, int], list[tuple[float, float, str, str, str]]] = {}
        for entry in components:
            # Accept dicts (current shape) plus legacy 4-/5-tuples for safety.
            if isinstance(entry, dict):
                cx = entry["x"]; cy = entry["y"]
                refdes = entry.get("refDes", "")
                part_name = entry.get("partName", "")
                side = entry.get("side", "")
            elif len(entry) == 5:
                cx, cy, refdes, part_name, side = entry
            else:
                cx, cy, refdes, part_name = entry
                side = ""
            key = (int(math.floor(cx / cell_size)), int(math.floor(cy / cell_size)))
            self._grid.setdefault(key, []).append((cx, cy, refdes, part_name, side))

    def lookup(self, x: float, y: float, side: str | None = None) -> tuple[str, str]:
        """Return (refdes, packageClass) for the nearest component whose
        tolerance covers this pad. Tolerance is derived from the component's
        package class so small packages use a tight radius and large packages
        use a wider one.

        When `side` is "top" or "bot", only components on the same side are
        considered — this prevents a top-side chip pin from being wrongly
        attributed to a bottom-side passive sitting directly underneath
        (or vice versa). A component with an unknown side ("") is treated
        as a wildcard and always eligible, preserving behavior for boards
        where side information couldn't be recovered.
        """
        cs = self._cell_size
        gx = int(math.floor(x / cs))
        gy = int(math.floor(y / cs))
        best_name = ""
        best_pkg = ""
        best_dist = float("inf")
        for dx in (-1, 0, 1):
            for dy in (-1, 0, 1):
                bucket = self._grid.get((gx + dx, gy + dy))
                if bucket is None:
                    continue
                for cx, cy, refdes, part_name, comp_side in bucket:
                    if side in ("top", "bot") and comp_side and comp_side != side:
                        continue
                    d2 = (x - cx) ** 2 + (y - cy) ** 2
                    pkg = _classify_package(part_name)
                    tol = _PACKAGE_TOLERANCE.get(pkg, _DEFAULT_TOLERANCE)
                    if d2 <= tol * tol and d2 < best_dist:
                        best_dist = d2
                        best_name = refdes
                        best_pkg = pkg
        return best_name, best_pkg


def _refdes_lookup(x: float, y: float, components: list, tol: float = 1.0) -> tuple[str, str]:
    """Return (refdes, packageClass) for nearest component within tol mm."""
    best_name = ""
    best_pkg = ""
    best_dist = tol * tol
    for cx, cy, refdes, part_name in components:
        d2 = (x - cx) ** 2 + (y - cy) ** 2
        if d2 <= best_dist:
            best_dist = d2
            best_name = refdes
            best_pkg = _classify_package(part_name)
    return best_name, best_pkg


# ── Archive extraction ────────────────────────────────────────────────────────

def _extract_odb_archive(file_path: str, tmpdir: str) -> None:
    """Extract ODB++ archive to tmpdir. Supports .zip, .tgz, and double-gzip variants."""
    if zipfile.is_zipfile(file_path):
        with zipfile.ZipFile(file_path, "r") as zf:
            zf.extractall(tmpdir)
        return

    try:
        with tarfile.open(file_path, "r:*") as tf:
            tf.extractall(tmpdir)
        return
    except Exception:
        pass

    with gzip.open(file_path, "rb") as gz:
        inner = io.BytesIO(gz.read())
    with tarfile.open(fileobj=inner, mode="r:*") as tf:
        tf.extractall(tmpdir)


def _find_job_root(tmp: Path) -> Path:
    """Find the ODB++ job root directory."""
    for matrix_file in tmp.rglob("matrix"):
        if matrix_file.is_file() and matrix_file.parent.name == "matrix":
            return matrix_file.parent.parent
    job_dirs = [d for d in tmp.iterdir() if d.is_dir()
                and d.name not in ("__MACOSX",)]
    if not job_dirs:
        raise ValueError("no job root in archive")
    return sorted(job_dirs)[0]


def _find_step_root(job_root: Path) -> Path:
    """Find the primary step directory inside job_root/steps/."""
    steps_dir = job_root / "steps"
    if not steps_dir.exists():
        raise ValueError("no steps directory in ODB++ job root")
    step_dirs = [d for d in steps_dir.iterdir() if d.is_dir()]
    if not step_dirs:
        raise ValueError("steps directory is empty")
    step_root: Path | None = None
    for preferred in ("pcb", "board"):
        for d in step_dirs:
            if d.name.lower() == preferred:
                step_root = d
                break
        if step_root is not None:
            break
    if step_root is None:
        step_root = sorted(step_dirs)[0]
    if len(step_dirs) > 1:
        skipped = [d.name for d in step_dirs if d != step_root]
        logger.info("ODB++ multi-step archive: using step %r, skipping %r", step_root.name, skipped)
    return step_root


def _find_layer_features(layers_dir: Path, layer_name: str) -> Path | None:
    """Find the features file for a layer using case-insensitive path lookup."""
    for candidate in (layer_name, layer_name.lower(), layer_name.upper()):
        feat = layers_dir / candidate / "features"
        if feat.exists():
            return feat
    return None


# ── Main parse entry point ────────────────────────────────────────────────────

def parse_odb(file_path: str) -> BoardData:
    """Parse ODB++ .tgz archive and return real BoardData."""
    layers: list[Layer] = []
    traces: list[Trace] = []
    pads: list[Pad] = []
    vias: list[Via] = []
    drills: list[Drill] = []
    outline: list[Point] = []
    outline_holes: list[list[Point]] = []
    warnings: list[str] = []
    polygons: list[Polygon] = []

    try:
        with tempfile.TemporaryDirectory() as tmpdir:
            _extract_odb_archive(file_path, tmpdir)

            tmp = Path(tmpdir)
            job_root = _find_job_root(tmp)
            step_root = _find_step_root(job_root)
            logger.info("ODB++ job: %s  step: %s", job_root.name, step_root.name)

            units = _read_units(step_root / "stephdr")
            logger.info("ODB++ units: %s", units)

            layer_defs = _parse_matrix(job_root / "matrix" / "matrix")
            outline, outline_holes = _parse_profile(step_root / "profile", units)
            logger.info("ODB++ outline: %d points, %d holes", len(outline), len(outline_holes))

            custom_syms = _load_custom_symbols(job_root / "symbols", units)
            if custom_syms:
                # Each named symbol is registered twice (case + lowercased), so
                # the symbol count is half the dict size.
                logger.info("ODB++ custom symbols loaded: %d", len(custom_syms) // 2)

            netlist_path = step_root / "netlists" / "cadnet" / "netlist"
            _, net_points = _parse_netlist(netlist_path, units)
            logger.info("ODB++ netlist: %d net points", len(net_points))

            # Parse eda/data for PKG records (secondary classification source)
            eda_pkgs: dict[int, dict] = {}
            eda_data_path = step_root / "eda" / "data"
            if eda_data_path.exists():
                eda_pkgs = _parse_eda_packages(eda_data_path, units)
                logger.info("ODB++ eda/data: %d packages", len(eda_pkgs))

            components: list = []
            # ODB++ components can be at steps/<step>/components/{top,bot}
            # or steps/<step>/layers/comp_+_{top,bot}/components
            comp_search_paths = [
                (step_root / "components" / "top", "components/top"),
                (step_root / "components" / "bot", "components/bot"),
            ]
            layers_dir_tmp = step_root / "layers"
            if layers_dir_tmp.exists():
                for d in layers_dir_tmp.iterdir():
                    if d.is_dir() and d.name.lower().startswith("comp"):
                        cfile = d / "components"
                        if cfile.exists():
                            comp_search_paths.append((cfile, f"layers/{d.name}/components"))
            for cp, label in comp_search_paths:
                if cp.exists():
                    lower = label.lower()
                    if "top" in lower:
                        side_hint: str | None = "top"
                    elif "bot" in lower or "btm" in lower:
                        side_hint = "bot"
                    else:
                        side_hint = None
                    c = _parse_components(cp, units, eda_pkgs=eda_pkgs, side_hint=side_hint)
                    components.extend(c)
                    logger.info("ODB++ %s: %d components", label, len(c))

            layers_dir = step_root / "layers"
            outline_layer_name: str | None = None

            # Padstack-ID → minimum copper outer-diameter map, populated
            # during the first (copper-only) pass and read during the
            # second (drill + everything else) pass. Lets us synthesize
            # Via records when a board encodes via geometry as symbolic
            # padstack names (`STANDARDVIA`, `LARGEVIA`, etc.) instead of
            # numeric dimensions in attr values.
            padstack_outer_mm: dict[int, float] = {}

            # Pre-resolve every layer's features path + ltype once so we
            # don't walk the directory twice, and preserve layer order by
            # appending to `layers` in the original matrix/matrix order.
            resolved: list[tuple[dict, Path | None, str | None]] = []
            for ld in layer_defs:
                ltype = _matrix_type_to_ltype(ld["type"])
                feat = _find_layer_features(layers_dir, ld["name"])
                resolved.append((ld, feat, ltype))
                if feat is None and ltype is not None:
                    logger.debug("ODB++ layer %r: features file not found (tried multiple cases)", ld["name"])
                    warnings.append(f"Layer {ld['name']!r}: features file not found")
                if ltype is None and ld["type"].upper() in ("ODB_BOARD_OUTLINE", "ROUT"):
                    outline_layer_name = ld["name"]
                if ltype is not None:
                    layers.append(Layer(name=ld["name"], type=ltype,
                                        startLayer=ld.get("start", ""),
                                        endLayer=ld.get("end", "")))

            def _run_layer(ld: dict, feat: Path, ltype: str) -> None:
                layer_name = ld["name"]
                before = len(traces) + len(pads) + len(vias)
                _parse_features(feat, layer_name, ltype, units, traces, pads, vias,
                                 net_points=net_points, components=components, drills=drills,
                                 warnings=warnings, polygons=polygons,
                                 padstack_outer_mm=padstack_outer_mm,
                                 custom_syms=custom_syms)
                after = len(traces) + len(pads) + len(vias)
                logger.info("ODB++ %s (%s): %d features", layer_name, ltype, after - before)

            # Pass 1: copper layers only — populates padstack_outer_mm so
            # the drill pass can cross-reference .padstack_id for via OD.
            for ld, feat, ltype in resolved:
                if feat is None or ltype not in ("COPPER", "POWER_GROUND"):
                    continue
                _run_layer(ld, feat, ltype)

            logger.info("ODB++ padstack map: %d distinct padstack_ids captured from copper",
                        len(padstack_outer_mm))

            # Pass 2: everything else (drill, rout, silk, mask, paste, ...).
            for ld, feat, ltype in resolved:
                if feat is None or ltype is None:
                    continue
                if ltype in ("COPPER", "POWER_GROUND"):
                    continue
                _run_layer(ld, feat, ltype)

            # Backfill net names on pour polygons whose S record carried no
            # `.net=` attribute — inferred from features (pads, traces,
            # netlist points, nearest via) physically inside or adjacent to
            # each polygon. See _infer_polygon_nets for why this matters
            # for the clearance rule.
            _infer_polygon_nets(polygons, pads, traces, vias, net_points, warnings=warnings)

            # Propagate net names to traces that lack them. Many ODB++
            # exports don't put `.net=` on L records and the netlist file
            # is too sparse for midpoint lookups. We seed from pads (a
            # trace endpoint touching a pad inherits its net) then BFS
            # through connected trace chains via shared endpoints.
            _propagate_trace_nets(traces, pads, layers, outline, warnings=warnings)

            # Mark via catch-pads: any pad whose center coincides with a
            # drill hit is a through-hole via annular ring, not a component
            # mounting pad. Rules use pad.isViaCatchPad to skip them.
            # Via catch-pad tagging: two strategies combined.
            # 1. Drill coincidence (50 µm tolerance) — works when the ODB++
            #    drill layer has real drill records at each via.
            # 2. Multi-layer pad coincidence — a pad that appears at the same
            #    (x,y) on 3+ copper layers is almost certainly a via catch-pad,
            #    even if the drill layer doesn't have a matching record (e.g.
            #    when drill markers were sub-minimum-diameter and got filtered).
            _tol = 0.05
            _tol2 = _tol * _tol
            _cell = 2.0
            # Pass 1: drill coincidence
            _drill_grid: dict[tuple[int, int], list[tuple[float, float]]] = {}
            for d in drills:
                _k = (int(d.x / _cell), int(d.y / _cell))
                _drill_grid.setdefault(_k, []).append((d.x, d.y))
            for p in pads:
                _gx, _gy = int(p.x / _cell), int(p.y / _cell)
                for _dx in (-1, 0, 1):
                    for _dy in (-1, 0, 1):
                        for (_drx, _dry) in _drill_grid.get((_gx + _dx, _gy + _dy), ()):
                            if (_drx - p.x) ** 2 + (_dry - p.y) ** 2 <= _tol2:
                                p.isViaCatchPad = True
                                break
                        if p.isViaCatchPad:
                            break
                    if p.isViaCatchPad:
                        break
            # Pass 2: multi-layer pad coincidence
            # Build a grid of (x,y,layer) → pad index for copper-type layers.
            _copper_layer_names = {l.name for l in layers
                                   if l.type in ("COPPER", "POWER_GROUND")}
            _xy_layers: dict[tuple[int, int], dict[tuple[float, float], set[str]]] = {}
            for p in pads:
                if p.layer not in _copper_layer_names:
                    continue
                _gx, _gy = int(p.x / _cell), int(p.y / _cell)
                cell = _xy_layers.setdefault((_gx, _gy), {})
                # Round coordinates to 0.01mm to cluster near-coincident pads.
                _xy_key = (round(p.x, 2), round(p.y, 2))
                cell.setdefault(_xy_key, set()).add(p.layer)
            # Any (x,y) with 3+ copper layers is a via location.
            _via_xys: set[tuple[float, float]] = set()
            for cell in _xy_layers.values():
                for xy, layer_set in cell.items():
                    if len(layer_set) >= 3:
                        _via_xys.add(xy)
            for p in pads:
                if p.isViaCatchPad:
                    continue
                if p.layer not in _copper_layer_names:
                    continue
                if (round(p.x, 2), round(p.y, 2)) in _via_xys:
                    p.isViaCatchPad = True
            _via_catch_count = sum(1 for p in pads if p.isViaCatchPad)
            logger.info("ODB++ via catch-pad tagging: %d / %d pads marked "
                        "(%d via locations found via multi-layer coincidence)",
                        _via_catch_count, len(pads), len(_via_xys))

            # Last-resort via synthesis. If neither _via_geometry_mm nor the
            # padstack-ID cross-reference emitted any vias, but the multi-
            # layer pad coincidence pass above identified via locations,
            # fabricate Via records by joining each _via_xys point to the
            # nearest drill hit. Outer diameter is the smallest pad OD seen
            # at that xy across copper layers. This only fires when every
            # other path fails — healthy boards skip it entirely.
            if not vias and _via_xys and drills:
                _xy_min_od: dict[tuple[float, float], float] = {}
                for p in pads:
                    if p.layer not in _copper_layer_names:
                        continue
                    xy_key = (round(p.x, 2), round(p.y, 2))
                    if xy_key not in _via_xys:
                        continue
                    od = min(p.widthMM, p.heightMM)
                    prev = _xy_min_od.get(xy_key)
                    if prev is None or od < prev:
                        _xy_min_od[xy_key] = od
                _drill_by_cell: dict[tuple[int, int], list[tuple[float, float, float, str]]] = {}
                for d in drills:
                    _k = (int(d.x / _cell), int(d.y / _cell))
                    _drill_by_cell.setdefault(_k, []).append((d.x, d.y, d.diamMM, d.layer))
                _synth_added = 0
                for xy, od in _xy_min_od.items():
                    gx, gy = int(xy[0] / _cell), int(xy[1] / _cell)
                    best: tuple[float, float, float, str] | None = None
                    best_d2 = _tol2
                    for _dx in (-1, 0, 1):
                        for _dy in (-1, 0, 1):
                            for (drx, dry, diam, dlayer) in _drill_by_cell.get((gx + _dx, gy + _dy), ()):
                                d2 = (drx - xy[0]) ** 2 + (dry - xy[1]) ** 2
                                if d2 <= best_d2:
                                    best = (drx, dry, diam, dlayer)
                                    best_d2 = d2
                    if best is not None and od > best[2]:
                        vias.append(Via(x=best[0], y=best[1],
                                        outerDiamMM=od, drillDiamMM=best[2],
                                        netName="", layer=best[3]))
                        _synth_added += 1
                if _synth_added:
                    logger.info("ODB++ via synthesis (coincidence fallback): +%d vias", _synth_added)
                    warnings.append(
                        f"Synthesized {_synth_added} Via records from multi-layer pad "
                        "coincidence — no .padstack_id or numeric via attrs found"
                    )

            if not outline and outline_layer_name:
                feat = _find_layer_features(layers_dir, outline_layer_name)
                if feat:
                    outline, outline_holes = _parse_profile(feat, units)
                    logger.info("ODB++ outline from layer %r: %d points, %d holes", outline_layer_name, len(outline), len(outline_holes))

            logger.info("ODB++ vias: %d", len(vias))

    except Exception as e:
        logger.error("ODB++ parse failed: %s", e, exc_info=True)
        warnings.append(f"Parse aborted: {e}")

    # Materialize component records for the BoardData payload. Downstream
    # rules (e.g. component-height) operate on these; the existing
    # refdes-lookup path above uses the same list internally.
    comp_models: list[Component] = []
    for c in components:
        if isinstance(c, dict):
            comp_models.append(Component(
                refDes=c.get("refDes", ""),
                x=c.get("x", 0.0), y=c.get("y", 0.0),
                side=c.get("side", ""),
                partName=c.get("partName", ""),
                packageClass=_classify_package(c.get("partName", "")) or "",
                heightMM=float(c.get("heightMM", 0.0) or 0.0),
                mountType=c.get("mountType", ""),
            ))

    logger.info("ODB++ done: %d layers, %d traces, %d pads, %d vias, %d drills, %d polygons, %d components",
                len(layers), len(traces), len(pads), len(vias), len(drills), len(polygons), len(comp_models))
    return BoardData(layers=layers, traces=traces, pads=pads, vias=vias,
                     drills=drills, outline=outline, boardThicknessMM=1.6,
                     warnings=warnings, polygons=polygons, outlineHoles=outline_holes,
                     components=comp_models)
