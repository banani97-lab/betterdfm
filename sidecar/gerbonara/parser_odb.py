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

from models import BoardData, Layer, Trace, Pad, Via, Drill, Point, Polygon
from units import _coord_to_mm, _sym_to_mm

logger = logging.getLogger(__name__)


# ── Symbol parsing ─────────────────────────────────────────────────────────────

def _parse_sym(sym: str, units: str = "INCH", warnings: list[str] | None = None, layer_name: str = "") -> dict:
    """Parse ODB++ symbol name into shape dict."""
    tokens = sym.strip().split()
    if not tokens:
        return {"shape": "CIRCLE", "w": 0.1, "h": 0.1, "inner": 0.0}
    sym = tokens[0]
    s = sym.lower()
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
                         layer_name: str = "") -> dict[int, dict]:
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
                                                     layer_name=layer_name)
        except (ValueError, IndexError):
            pass
    return symbols


# ── Matrix / profile parsing ───────────────────────────────────────────────────

def _read_units(path: Path) -> str:
    """Read UNITS= from ODB++ step header."""
    try:
        for line in path.read_text(errors="replace").splitlines():
            if line.startswith("UNITS="):
                return line.split("=", 1)[1].strip()
    except OSError:
        pass
    return "INCH"


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
        return "SOLDER_MASK"
    if m == "ROUT":
        return "ROUT"
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
            in_island = False
    return outline


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
) -> None:
    """Build geometry from a token list produced by _tokenize_features."""
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
            # Outer island — start a new polygon
            if ltype in ("COPPER", "POWER_GROUND"):
                current_polygon = Polygon(
                    layer=layer_name,
                    points=[Point(x=x, y=y) for x, y in surface_pts],
                    netName=surface_net,
                )
            elif ltype in ("SOLDER_MASK", "SILK"):
                xs = [p[0] for p in surface_pts]
                ys = [p[1] for p in surface_pts]
                cx = (min(xs) + max(xs)) / 2
                cy = (min(ys) + max(ys)) / 2
                w = max(0.01, max(xs) - min(xs))
                h = max(0.01, max(ys) - min(ys))
                is_paste = "paste" in layer_name.lower()
                if not is_paste and (w > 2.0 or h > 2.0):
                    return
                pads.append(Pad(layer=layer_name, x=cx, y=cy,
                                widthMM=w, heightMM=h, shape="RECT",
                                netName="", refDes=""))
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
                mid_x = (x1 + x2) / 2
                mid_y = (y1 + y2) / 2
                net = _attr_net(raw) or _net_lookup(mid_x, mid_y, net_points)
                traces.append(Trace(layer=layer_name, widthMM=max(0.01, sym["w"]),
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
                # ODB++ P record: P x y sym_num polarity rotation mirror ;attrs
                # sym_num is at parts[3]; parts[5] is rotation (not sym_num)
                sym = symbols.get(int(parts[3]), {"w": 0.5, "h": 0.5,
                                                   "shape": "CIRCLE", "inner": 0.0})
                net = _attr_net(raw) or _net_lookup(x, y, net_points)
                ref = _refdes_lookup(x, y, components)
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
                                            drillDiamMM=hole_diam, netName=net))
                    drills.append(Drill(x=x, y=y, diamMM=hole_diam, plated=plated))
                elif ltype == "POWER_GROUND" and sym["shape"] == "DONUT":
                    pass
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

        elif rec == "A":
            if ltype not in ("COPPER", "POWER_GROUND", "SILK"):
                continue
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
) -> None:
    """Parse ODB++ features file and append geometry to traces/pads/vias/polygons."""
    net_points = net_points or []
    components = components or []

    try:
        text = features_path.read_text(errors="replace")
    except OSError:
        return

    lines = text.splitlines()
    symbols = _parse_symbol_table(lines, units, warnings=warnings, layer_name=layer_name)
    tokens = _tokenize_features(lines)

    attr_values: dict[int, str] = {}
    if ltype == "DRILL":
        _attr_names, attr_values = _parse_attr_tables(lines)

    _build_features(tokens, layer_name, ltype, units, symbols,
                    net_points, components, drills, traces, pads, vias,
                    warnings=warnings,
                    drill_attr_values=attr_values,
                    polygons=polygons)


def _parse_rout(features_path: Path, units: str, drills: list) -> None:
    """Parse ODB++ rout layer features for drill holes (P records only)."""
    try:
        text = features_path.read_text(errors="replace")
    except OSError:
        return
    lines = text.splitlines()
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
                drills.append(Drill(x=x, y=y, diamMM=max(0.01, sym["w"]), plated=True))
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


def _parse_components(comp_path: Path, units: str) -> list:
    """Parse ODB++ CMP file → [(x_mm, y_mm, refdes)]."""
    components: list[tuple[float, float, str]] = []
    try:
        text = comp_path.read_text(errors="replace")
    except OSError:
        return components

    for line in text.splitlines():
        s = line.strip()
        if not s or not s.startswith("CMP "):
            continue
        attr_pos = s.find(";")
        s = s[:attr_pos].strip() if attr_pos >= 0 else s
        parts = s.split()
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
            outline = _parse_profile(step_root / "profile", units)
            logger.info("ODB++ outline: %d points", len(outline))

            netlist_path = step_root / "netlists" / "cadnet" / "netlist"
            _, net_points = _parse_netlist(netlist_path, units)
            logger.info("ODB++ netlist: %d net points", len(net_points))

            components: list = []
            for comp_file in ["top", "bot"]:
                cp = step_root / "components" / comp_file
                if cp.exists():
                    c = _parse_components(cp, units)
                    components.extend(c)
                    logger.info("ODB++ components/%s: %d components", comp_file, len(c))

            layers_dir = step_root / "layers"
            outline_layer_name: str | None = None

            for ld in layer_defs:
                ltype = _matrix_type_to_ltype(ld["type"])
                feat = _find_layer_features(layers_dir, ld["name"])
                if feat is None:
                    logger.debug("ODB++ layer %r: features file not found (tried multiple cases)", ld["name"])
                    if ltype is not None:
                        warnings.append(f"Layer {ld['name']!r}: features file not found")
                    elif ld["type"].upper() in ("ODB_BOARD_OUTLINE", "ROUT"):
                        outline_layer_name = ld["name"]
                    continue
                if ltype is None:
                    if ld["type"].upper() in ("ODB_BOARD_OUTLINE", "ROUT"):
                        outline_layer_name = ld["name"]
                    continue
                layer_name = ld["name"]
                layers.append(Layer(name=layer_name, type=ltype))
                before = len(traces) + len(pads) + len(vias)
                _parse_features(feat, layer_name, ltype, units, traces, pads, vias,
                                 net_points=net_points, components=components, drills=drills,
                                 warnings=warnings, polygons=polygons)
                after = len(traces) + len(pads) + len(vias)
                logger.info("ODB++ %s (%s): %d features", layer_name, ltype, after - before)

            if not outline and outline_layer_name:
                feat = _find_layer_features(layers_dir, outline_layer_name)
                if feat:
                    outline = _parse_profile(feat, units)
                    logger.info("ODB++ outline from layer %r: %d points", outline_layer_name, len(outline))

            logger.info("ODB++ vias: %d", len(vias))

    except Exception as e:
        logger.error("ODB++ parse failed: %s", e, exc_info=True)
        warnings.append(f"Parse aborted: {e}")

    logger.info("ODB++ done: %d layers, %d traces, %d pads, %d vias, %d drills, %d polygons",
                len(layers), len(traces), len(pads), len(vias), len(drills), len(polygons))
    return BoardData(layers=layers, traces=traces, pads=pads, vias=vias,
                     drills=drills, outline=outline, boardThicknessMM=1.6,
                     warnings=warnings, polygons=polygons)
