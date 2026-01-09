"""Lightning Code Index - Sub-millisecond semantic code search and analysis."""

__version__ = "0.0.0"  # Replaced during release

import os
import platform
import subprocess
import sys
from pathlib import Path


def _get_binary_path() -> Path:
    """Get the path to the lci binary."""
    # Binary is bundled in the package
    package_dir = Path(__file__).parent

    system = platform.system().lower()
    machine = platform.machine().lower()

    # Normalize architecture names
    if machine in ("x86_64", "amd64"):
        arch = "amd64"
    elif machine in ("arm64", "aarch64"):
        arch = "arm64"
    else:
        raise RuntimeError(f"Unsupported architecture: {machine}")

    # Normalize OS names and get binary name
    if system == "darwin":
        binary_name = f"lci_darwin_{arch}"
    elif system == "linux":
        binary_name = f"lci_linux_{arch}"
    elif system == "windows":
        if arch != "amd64":
            raise RuntimeError(f"Windows only supports amd64, got: {arch}")
        binary_name = "lci_windows_amd64.exe"
    else:
        raise RuntimeError(f"Unsupported operating system: {system}")

    binary_path = package_dir / "bin" / binary_name

    if not binary_path.exists():
        raise RuntimeError(
            f"Binary not found at {binary_path}. "
            "This may be a source installation. "
            "Please install from PyPI: pip install lci"
        )

    return binary_path


def main() -> int:
    """Run the lci binary with the given arguments."""
    try:
        binary_path = _get_binary_path()
    except RuntimeError as e:
        print(f"Error: {e}", file=sys.stderr)
        return 1

    # Make sure binary is executable on Unix
    if platform.system() != "Windows":
        os.chmod(binary_path, 0o755)

    # Run the binary with all arguments
    result = subprocess.run(
        [str(binary_path)] + sys.argv[1:],
        stdin=sys.stdin,
        stdout=sys.stdout,
        stderr=sys.stderr,
    )

    return result.returncode


if __name__ == "__main__":
    sys.exit(main())
