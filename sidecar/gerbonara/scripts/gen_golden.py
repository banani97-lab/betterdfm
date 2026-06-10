"""Generate golden-board fixtures for the DFM engine regression harness.

Parses real ODB++ boards with the production parser and writes the resulting
BoardData JSON (gzipped) into engine/dfm-engine/testdata/golden/<name>/.
The committed JSON is the contract: engine golden tests run against it in CI
without needing the raw board archives (which stay in S3 / local disk).

Usage (from sidecar/gerbonara):

    uv run python scripts/gen_golden.py <name>=<source> [<name>=<source> ...]

where <source> is one of:
    - a local .tgz/.tar/.zip archive
    - a local uncompressed ODB++ job directory (tarred on the fly)
    - an s3://bucket/key URI (downloaded to a temp file)

Run on demand whenever a parser change is meant to alter golden board data,
then regenerate expectations with:

    cd engine/dfm-engine && go test -run TestGoldenBoards -update ./...
"""

from __future__ import annotations

import gzip
import sys
import tarfile
import tempfile
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))

from parser_odb import parse_odb  # noqa: E402

GOLDEN_ROOT = (
    Path(__file__).resolve().parents[3] / "engine" / "dfm-engine" / "testdata" / "golden"
)


def _materialize(source: str, tmpdir: str) -> str:
    """Return a local archive path for `source`, downloading/tarring as needed."""
    if source.startswith("s3://"):
        import boto3

        bucket, _, key = source[5:].partition("/")
        local = str(Path(tmpdir) / Path(key).name)
        boto3.client("s3").download_file(bucket, key, local)
        return local
    p = Path(source).expanduser()
    if p.is_dir():
        # parse_odb only accepts archives; tar the job directory on the fly.
        local = str(Path(tmpdir) / (p.name + ".tar"))
        with tarfile.open(local, "w") as tf:
            tf.add(p, arcname=p.name)
        return local
    if not p.exists():
        raise FileNotFoundError(source)
    return str(p)


def generate(name: str, source: str) -> None:
    out_dir = GOLDEN_ROOT / name
    out_dir.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory() as tmpdir:
        archive = _materialize(source, tmpdir)
        board = parse_odb(archive)
    payload = board.model_dump_json().encode()
    out_path = out_dir / "board.json.gz"
    # mtime=0 + no filename keeps the .gz byte-stable across regenerations.
    with open(out_path, "wb") as raw:
        with gzip.GzipFile(fileobj=raw, mode="wb", compresslevel=9, mtime=0) as gz:
            gz.write(payload)
    print(
        f"{name}: layers={len(board.layers)} traces={len(board.traces)} "
        f"pads={len(board.pads)} vias={len(board.vias)} drills={len(board.drills)} "
        f"polygons={len(board.polygons)} components={len(board.components)} "
        f"warnings={len(board.warnings)}"
    )
    print(f"  -> {out_path} ({out_path.stat().st_size / 1024:.0f} KiB gzipped, "
          f"{len(payload) / 1024:.0f} KiB raw)")


def main(argv: list[str]) -> int:
    if not argv:
        print(__doc__)
        return 1
    for spec in argv:
        name, _, source = spec.partition("=")
        if not source:
            print(f"bad spec {spec!r}: expected <name>=<source>", file=sys.stderr)
            return 1
        generate(name, source)
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv[1:]))
