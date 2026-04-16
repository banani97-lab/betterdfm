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
    shape: str  # "RECT" | "CIRCLE" | "OVAL" | "POLYGON"
    netName: str = ""
    refDes: str = ""
    packageClass: str = ""  # e.g. "0201", "0402", "0603", "0805", "1206"
    contour: list[Point] = []  # polygon contour when shape == "POLYGON"
    isFiducial: bool = False
    isViaCatchPad: bool = False


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
