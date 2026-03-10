import sys, tempfile
from pathlib import Path
import pytest
sys.path.insert(0, str(Path(__file__).parent.parent))
from main import _find_job_root, _find_step_root, _find_layer_features


def make_odb_tree(tmp: Path, job_name="myjob", step_name="pcb", layers=("TOP", "BOT")):
    """Create a minimal ODB++ directory tree."""
    job = tmp / job_name
    (job / "matrix").mkdir(parents=True)
    (job / "matrix" / "matrix").write_text("# matrix\n")
    step = job / "steps" / step_name
    (step / "layers").mkdir(parents=True)
    (step / "stephdr").write_text("UNITS=INCH\n")
    for layer in layers:
        (step / "layers" / layer).mkdir(parents=True)
        (step / "layers" / layer / "features").write_text("# features\n")
    return job


def test_find_job_root_basic():
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        job = make_odb_tree(tmp)
        found = _find_job_root(tmp)
        assert found.name == job.name


def test_find_job_root_skips_macosx():
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        # Add __MACOSX artifact directory
        (tmp / "__MACOSX").mkdir()
        (tmp / "__MACOSX" / "._something").write_text("junk")
        job = make_odb_tree(tmp)
        found = _find_job_root(tmp)
        assert found.name != "__MACOSX"
        assert found.name == job.name


def test_find_step_root_prefers_pcb():
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        job = make_odb_tree(tmp, step_name="pcb")
        # Add an alphabetically-first step that should NOT be chosen
        (job / "steps" / "aaa").mkdir(parents=True)
        found = _find_step_root(job)
        assert found.name == "pcb"


def test_find_step_root_prefers_board():
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        job = make_odb_tree(tmp, step_name="board")
        (job / "steps" / "zzz").mkdir(parents=True)
        found = _find_step_root(job)
        assert found.name == "board"


def test_find_step_root_fallback_alphabetical():
    with tempfile.TemporaryDirectory() as td:
        tmp = Path(td)
        job = make_odb_tree(tmp, step_name="step1")
        (job / "steps" / "step2").mkdir(parents=True)
        found = _find_step_root(job)
        assert found.name == "step1"  # alphabetically first


def test_find_layer_features_original_case():
    with tempfile.TemporaryDirectory() as td:
        layers_dir = Path(td)
        (layers_dir / "TOP").mkdir()
        (layers_dir / "TOP" / "features").write_text("# features\n")
        result = _find_layer_features(layers_dir, "TOP")
        assert result is not None
        assert result.exists()


def test_find_layer_features_lowercase_name():
    """Upper-case layer dir found when looked up by lowercase name."""
    with tempfile.TemporaryDirectory() as td:
        layers_dir = Path(td)
        (layers_dir / "TOP").mkdir()
        (layers_dir / "TOP" / "features").write_text("# features\n")
        result = _find_layer_features(layers_dir, "top")
        assert result is not None


def test_find_layer_features_missing_returns_none():
    with tempfile.TemporaryDirectory() as td:
        layers_dir = Path(td)
        result = _find_layer_features(layers_dir, "nonexistent")
        assert result is None
