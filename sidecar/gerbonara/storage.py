from __future__ import annotations

import logging
import os
import tempfile
from pathlib import Path

import boto3
from botocore.exceptions import ClientError

from models import BoardData, Layer, Trace, Pad, Via, Drill, Point

logger = logging.getLogger(__name__)


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
            Pad(layer="top_copper", x=11.0, y=10.0, widthMM=1.5, heightMM=1.5, shape="CIRCLE"),
        ],
        vias=[
            Via(x=20.0, y=20.0, outerDiamMM=0.8, drillDiamMM=0.4),
            Via(x=40.0, y=20.0, outerDiamMM=0.6, drillDiamMM=0.4),
        ],
        drills=[
            Drill(x=10.0, y=10.0, diamMM=0.8, plated=True),
            Drill(x=30.0, y=30.0, diamMM=0.2, plated=True),
        ],
        outline=[
            Point(x=0.0, y=0.0),
            Point(x=60.0, y=0.0),
            Point(x=60.0, y=40.0),
            Point(x=0.0, y=40.0),
        ],
        boardThicknessMM=1.6,
    )
