import sys
from pathlib import Path
import pytest

# Make `main` importable without installing the package
sys.path.insert(0, str(Path(__file__).parent.parent))

FIXTURES = Path(__file__).parent / "fixtures"
