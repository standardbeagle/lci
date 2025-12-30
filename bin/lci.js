#!/usr/bin/env node

import { spawn } from 'child_process';
import { fileURLToPath } from 'url';
import path from 'path';
import fs from 'fs';

const __dirname = path.dirname(fileURLToPath(import.meta.url));

// Determine the correct binary name based on platform and architecture
const platform = process.platform; // darwin, linux, win32
const arch = process.arch; // x64, arm64
const ext = platform === 'win32' ? '.exe' : '';
const binaryName = `lci-${platform}-${arch}${ext}`;

// Try multiple possible locations for the binary
const possiblePaths = [
  // Installed via npm
  path.join(__dirname, '..', 'dist', binaryName),
  // Development mode
  path.join(__dirname, '..', 'lci'),
  path.join(__dirname, '..', 'cmd', 'lci', 'lci'),
];

let binaryPath = null;
for (const p of possiblePaths) {
  if (fs.existsSync(p)) {
    binaryPath = p;
    break;
  }
}

if (!binaryPath) {
  console.error(`Error: lci binary not found for ${platform}-${arch}`);
  console.error('Tried:', possiblePaths);
  console.error('\nPlease run: npm run build');
  process.exit(1);
}

// Spawn the binary with all arguments
const child = spawn(binaryPath, process.argv.slice(2), {
  stdio: 'inherit',
  env: process.env,
});

child.on('exit', (code) => {
  process.exit(code || 0);
});
