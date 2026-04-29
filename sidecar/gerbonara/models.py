from __future__ import annotations

from pydantic import BaseModel


class ParseRequest(BaseModel):
    fileKey: str
    fileType: str  # "ODB_PLUS_PLUS"
    bucket: str


class Point(BaseModel):
    x: float
    y: float


class Layer(BaseModel):
    name: str
    type: str  # "COPPER" | "SOLDER_MASK" | "SOLDER_PASTE" | "SILK" | "DRILL" | "OUTLINE"


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
    shape: str  # "RECT" | "CIRCLE" | "OVAL" | "POLYGON" | "DONUT"
    netName: str = ""
    refDes: str = ""
    packageClass: str = ""  # e.g. "0201", "0402", "0603", "0805", "1206"
    contour: list[Point] = []  # polygon contour when shape == "POLYGON"
    holeMM: float = 0.0       # inner diameter when shape == "DONUT" (via catch-pad ring)
    isFiducial: bool = False
    isViaCatchPad: bool = False


class Via(BaseModel):
    x: float
    y: float
    outerDiamMM: float
    drillDiamMM: float
    netName: str = ""
    layer: str = ""    # ODB++ drill layer name (e.g. "D_1_10" for SIGNAL_1↔SIGNAL_10),
                       # so the viewer can hide microvias when only through-hole layers are on.


class Drill(BaseModel):
    x: float
    y: float
    diamMM: float
    plated: bool
    layer: str = ""    # ODB++ drill layer (D_1_10, D_5_6, etc.) — the layer span
                       # is encoded in the name and the matrix START_NAME/END_NAME.


class Polygon(BaseModel):
    layer: str
    points: list[Point]
    holes: list[list[Point]] = []
    netName: str = ""


class Component(BaseModel):
    refDes: str
    x: float
    y: float
    side: str = ""         # "top" | "bot" | ""
    partName: str = ""     # raw part/package name as parsed
    packageClass: str = "" # IPC class (e.g. "0402") when classifiable
    heightMM: float = 0.0  # from ODB++ `.comp_height`, 0 if not declared
    mountType: str = ""    # "smt" | "thmt" | "pressfit" | "manual" | "other"


class BoardData(BaseModel):
    layers: list[Layer]
    traces: list[Trace]
    pads: list[Pad]
    vias: list[Via]
    drills: list[Drill]
    outline: list[Point]
    boardThicknessMM: float
    warnings: list[str] = []
    polygons: list[Polygon] = []
    outlineHoles: list[list[Point]] = []  # inner cutout boundaries (slots, step-outs)
    components: list[Component] = []      # for component-level rules (height, etc.)
    sourceFormat: str = ""  # "ODB_PLUS_PLUS"
