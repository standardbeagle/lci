#!/usr/bin/env python3
"""Build platform-specific wheels with embedded binaries."""

import argparse
import os
import shutil
import subprocess
import sys
from pathlib import Path

# Mapping of platform tags to binary names
PLATFORM_BINARIES = {
    # macOS
    "macosx_10_12_x86_64": "lci_darwin_amd64",
    "macosx_11_0_arm64": "lci_darwin_arm64",
    # Linux
    "manylinux_2_17_x86_64": "lci_linux_amd64",
    "manylinux_2_17_aarch64": "lci_linux_arm64",
    "musllinux_1_1_x86_64": "lci_linux_amd64",
    "musllinux_1_1_aarch64": "lci_linux_arm64",
    # Windows
    "win_amd64": "lci_windows_amd64.exe",
}


def build_wheel(
    dist_dir: Path,
    binary_dir: Path,
    platform_tag: str,
    binary_name: str,
    version: str,
) -> Path:
    """Build a wheel for a specific platform."""
    # Create temporary package directory
    temp_dir = Path("_build_temp")
    if temp_dir.exists():
        shutil.rmtree(temp_dir)
    temp_dir.mkdir()

    # Copy Python package
    pkg_dir = temp_dir / "lci"
    shutil.copytree("python/lci", pkg_dir)

    # Update version in __init__.py
    init_file = pkg_dir / "__init__.py"
    content = init_file.read_text()
    content = content.replace('__version__ = "0.0.0"', f'__version__ = "{version}"')
    init_file.write_text(content)

    # Create bin directory and copy binary
    bin_dir = pkg_dir / "bin"
    bin_dir.mkdir()

    binary_src = binary_dir / binary_name
    if not binary_src.exists():
        # Try with archive extraction path
        archive_name = binary_name.replace(".exe", "")
        for archive in binary_dir.glob(f"lci_*_{archive_name.split('_', 1)[1]}*"):
            if archive.is_dir():
                binary_src = archive / ("lci.exe" if binary_name.endswith(".exe") else "lci")
                break

    if not binary_src.exists():
        print(f"Warning: Binary not found: {binary_name}, skipping platform {platform_tag}")
        shutil.rmtree(temp_dir)
        return None

    binary_dest = bin_dir / binary_name
    shutil.copy2(binary_src, binary_dest)

    # Make executable on Unix
    if not binary_name.endswith(".exe"):
        os.chmod(binary_dest, 0o755)

    # Create minimal pyproject.toml for wheel building
    pyproject = temp_dir / "pyproject.toml"
    pyproject.write_text(f'''[build-system]
requires = ["hatchling"]
build-backend = "hatchling.build"

[project]
name = "lightning-code-index"
version = "{version}"
description = "Lightning Code Index - Sub-millisecond semantic code search and analysis"
readme = "README.md"
license = "MIT"
requires-python = ">=3.8"

[project.scripts]
lci = "lci:main"

[tool.hatch.build.targets.wheel]
packages = ["lci"]
''')

    # Copy README
    shutil.copy2("README.md", temp_dir / "README.md")

    # Build wheel
    subprocess.run(
        [
            sys.executable, "-m", "pip", "wheel",
            "--no-deps",
            "--wheel-dir", str(dist_dir),
            str(temp_dir),
        ],
        check=True,
    )

    # Find the built wheel and rename with platform tag
    wheels = list(dist_dir.glob("lightning_code_index-*.whl"))
    if not wheels:
        raise RuntimeError("No wheel built")

    wheel = wheels[-1]
    # Rename to platform-specific wheel
    parts = wheel.name.split("-")
    # lci-version-py3-none-any.whl -> lci-version-py3-none-platform.whl
    new_name = f"{parts[0]}-{parts[1]}-py3-none-{platform_tag}.whl"
    new_path = dist_dir / new_name

    if new_path.exists():
        new_path.unlink()
    wheel.rename(new_path)

    # Cleanup
    shutil.rmtree(temp_dir)

    return new_path


def main():
    parser = argparse.ArgumentParser(description="Build platform-specific wheels")
    parser.add_argument("--version", required=True, help="Version string (e.g., 0.1.0)")
    parser.add_argument("--binary-dir", required=True, help="Directory containing binaries")
    parser.add_argument("--dist-dir", default="dist", help="Output directory for wheels")
    parser.add_argument("--platforms", nargs="+", help="Specific platforms to build (default: all)")
    args = parser.parse_args()

    dist_dir = Path(args.dist_dir)
    dist_dir.mkdir(exist_ok=True)

    binary_dir = Path(args.binary_dir)

    platforms = args.platforms or PLATFORM_BINARIES.keys()

    built = []
    for platform_tag in platforms:
        if platform_tag not in PLATFORM_BINARIES:
            print(f"Unknown platform: {platform_tag}")
            continue

        binary_name = PLATFORM_BINARIES[platform_tag]
        print(f"Building wheel for {platform_tag} using {binary_name}...")

        wheel_path = build_wheel(
            dist_dir=dist_dir,
            binary_dir=binary_dir,
            platform_tag=platform_tag,
            binary_name=binary_name,
            version=args.version,
        )

        if wheel_path:
            built.append(wheel_path)
            print(f"  Built: {wheel_path.name}")

    # Also build source distribution
    print("Building source distribution...")
    subprocess.run(
        [sys.executable, "-m", "build", "--sdist", "--outdir", str(dist_dir)],
        check=True,
    )

    print(f"\nBuilt {len(built)} wheels + sdist in {dist_dir}/")


if __name__ == "__main__":
    main()
