#!/usr/bin/env node

const { execFileSync } = require("child_process");
const path = require("path");
const fs = require("fs");
const os = require("os");

function getBinaryPath() {
  const platform = os.platform();
  const arch = os.arch();

  let binaryName;

  if (platform === "darwin") {
    if (arch === "x64") {
      binaryName = "lci_darwin_amd64";
    } else if (arch === "arm64") {
      binaryName = "lci_darwin_arm64";
    }
  } else if (platform === "linux") {
    if (arch === "x64") {
      binaryName = "lci_linux_amd64";
    } else if (arch === "arm64") {
      binaryName = "lci_linux_arm64";
    }
  } else if (platform === "win32") {
    if (arch === "x64") {
      binaryName = "lci_windows_amd64.exe";
    }
  }

  if (!binaryName) {
    console.error(`Unsupported platform: ${platform} ${arch}`);
    console.error("Supported platforms: darwin (x64, arm64), linux (x64, arm64), win32 (x64)");
    process.exit(1);
  }

  const binaryPath = path.join(__dirname, "..", "binaries", binaryName);

  if (!fs.existsSync(binaryPath)) {
    console.error(`Binary not found: ${binaryPath}`);
    console.error("This may be a development installation.");
    console.error("Please install from npm: npm install -g @standardbeagle/lci");
    process.exit(1);
  }

  return binaryPath;
}

function main() {
  const binaryPath = getBinaryPath();
  const args = process.argv.slice(2);

  try {
    execFileSync(binaryPath, args, {
      stdio: "inherit",
      env: process.env,
    });
  } catch (error) {
    if (error.status !== undefined) {
      process.exit(error.status);
    }
    console.error(`Failed to execute binary: ${error.message}`);
    process.exit(1);
  }
}

main();
