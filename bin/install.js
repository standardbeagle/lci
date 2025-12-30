#!/usr/bin/env node

import { execSync } from 'child_process';
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const rootDir = path.join(__dirname, '..');
const distDir = path.join(rootDir, 'dist');

// Skip build if binary already exists (for development)
const platform = process.platform;
const arch = process.arch;
const ext = platform === 'win32' ? '.exe' : '';
const binaryName = `lci-${platform}-${arch}${ext}`;
const binaryPath = path.join(distDir, binaryName);

if (fs.existsSync(binaryPath)) {
  console.log(`✓ Binary already exists: ${binaryName}`);
  process.exit(0);
}

// Check if we're in a development environment with Go
const hasGo = (() => {
  try {
    execSync('go version', { stdio: 'ignore' });
    return true;
  } catch {
    return false;
  }
})();

if (!hasGo) {
  console.error('Error: Go is not installed or not in PATH');
  console.error('Please install Go from https://go.dev/dl/');
  process.exit(1);
}

// Build the binary
console.log(`Building lci for ${platform}-${arch}...`);
try {
  fs.mkdirSync(distDir, { recursive: true });

  execSync(`go build -o "${binaryPath}" ./cmd/lci`, {
    cwd: rootDir,
    stdio: 'inherit',
  });

  console.log(`✓ Build successful: ${binaryName}`);
} catch (error) {
  console.error('Build failed:', error.message);
  process.exit(1);
}
