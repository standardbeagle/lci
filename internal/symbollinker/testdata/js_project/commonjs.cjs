// CommonJS module example
const fs = require('fs');
const path = require('path');
const { promisify } = require('util');

// Module imports
const express = require('express');
const lodash = require('lodash');

// Local requires
const utils = require('./lib/utils');
const { formatDate, parseJSON } = require('./lib/utils');

// Module variables
const readFile = promisify(fs.readFile);
const writeFile = promisify(fs.writeFile);

// Module exports
module.exports = {
  readConfig,
  saveConfig,
  ConfigManager
};

// Named exports
module.exports.VERSION = '1.0.0';
module.exports.DEFAULT_CONFIG = {
  debug: false,
  port: 3000
};

// Function definitions
function readConfig(filepath) {
  try {
    const content = fs.readFileSync(filepath, 'utf8');
    return JSON.parse(content);
  } catch (error) {
    console.error('Error reading config:', error);
    return null;
  }
}

async function saveConfig(filepath, config) {
  const content = JSON.stringify(config, null, 2);
  await writeFile(filepath, content, 'utf8');
}

// Class definition
class ConfigManager {
  constructor(configPath) {
    this.configPath = configPath;
    this.config = null;
  }
  
  load() {
    this.config = readConfig(this.configPath);
    return this.config;
  }
  
  async save() {
    if (this.config) {
      await saveConfig(this.configPath, this.config);
    }
  }
  
  get(key) {
    return lodash.get(this.config, key);
  }
  
  set(key, value) {
    lodash.set(this.config, key, value);
  }
}

// Alternative export patterns
exports.createManager = function(path) {
  return new ConfigManager(path);
};

exports.helpers = {
  validateConfig(config) {
    return config && typeof config === 'object';
  },
  
  mergeConfigs(...configs) {
    return Object.assign({}, ...configs);
  }
};