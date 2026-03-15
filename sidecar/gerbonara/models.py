from __future__ import annotations

from pydantic import BaseModel


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


class Polygon(BaseModel):
    layer: str
    points: list[Point]
    holes: list[list[Point]] = []
    netName: str = ""


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
