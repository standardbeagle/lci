// Utility functions
import os from 'os';
import fs from 'fs/promises';

// Named exports
export function formatDate(date) {
  return date.toISOString();
}

export function parseJSON(text) {
  try {
    return JSON.parse(text);
  } catch (e) {
    return null;
  }
}

// Async function
export async function readFile(filepath) {
  const content = await fs.readFile(filepath, 'utf8');
  return content;
}

// Generator function
export function* range(start, end) {
  for (let i = start; i <= end; i++) {
    yield i;
  }
}

// Arrow function export
export const delay = (ms) => new Promise(resolve => setTimeout(resolve, ms));

// Complex function with default parameters and rest
export function createLogger(prefix = '[LOG]', ...tags) {
  return function log(message) {
    console.log(`${prefix} ${tags.join(',')} ${message}`);
  };
}

// System utilities
export async function getSystemStatus() {
  return {
    platform: os.platform(),
    memory: os.freemem(),
    uptime: os.uptime()
  };
}

// Re-export
export { default as config } from '../config.js';

// Class with static methods
export class Utils {
  static formatNumber(num) {
    return num.toLocaleString();
  }
  
  static async fetchData(url) {
    const response = await fetch(url);
    return response.json();
  }
}