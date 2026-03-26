from __future__ import annotations

"""ODB++ / Gerber sidecar parser for BetterDFM (gerbonara).

Parse pipeline (ODB++):
    parse_odb(file_path)
      └─ _extract_odb_archive → _find_job_root → _find_step_root
         ├─ _read_units, _parse_matrix, _parse_profile
         ├─ _parse_netlist, _parse_components
         └─ for each layer:
              _parse_features(feat_path, layer, ltype, units, ...)
                ├─ _parse_symbol_table(lines, units) → _parse_sym(sym, units)
                ├─ _tokenize_features(lines) → list[token dicts]
                └─ _build_features(tokens, ...) → modifies traces/pads/vias in place

Coordinate invariant:
    All coordinates and dimensions leaving any parse function are in millimeters (float).
    Symbol dimensions use _sym_to_mm(v, units):
      - INCH files: symbol dims in mils (1/1000 inch) → × 0.0254
      - MM files:   symbol dims in microns (1/1000 mm) → × 0.001
"""

import logging
import os

from fastapi import FastAPI, HTTPException

from models import ParseRequest, BoardData
from storage import download_from_s3, _mock_board
from parser_odb import parse_odb
from parser_gerber import parse_gerber

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

app = FastAPI(title="BetterDFM Gerbonara Sidecar", version="0.1.0")


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
