// Main entry point for the application
import express from 'express';
import { createServer } from 'http';
import { fileURLToPath } from 'url';
import path from 'path';
import * as utils from './lib/utils.js';
import config, { API_KEY, SERVER_PORT } from './config.js';

// CommonJS compatibility
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Create express app
const app = express();

// Constants
const DEFAULT_PORT = 3000;
export const APP_NAME = 'TestApp';
export const VERSION = '1.0.0';

// Initialize server
export function initServer(port = DEFAULT_PORT) {
  const server = createServer(app);
  
  app.use(express.json());
  app.use(express.static(path.join(__dirname, 'public')));
  
  // Setup routes
  app.get('/', (req, res) => {
    res.json({ 
      message: 'Welcome',
      version: VERSION 
    });
  });
  
  app.get('/api/status', async (req, res) => {
    const status = await utils.getSystemStatus();
    res.json(status);
  });
  
  return server;
}

// Start server
export async function startServer() {
  const port = process.env.PORT || SERVER_PORT || DEFAULT_PORT;
  const server = initServer(port);
  
  return new Promise((resolve, reject) => {
    server.listen(port, (err) => {
      if (err) {
        reject(err);
      } else {
        console.log(`Server running on port ${port}`);
        resolve(server);
      }
    });
  });
}

// Class example
export class Application {
  constructor(name, config) {
    this.name = name;
    this.config = config;
    this.#privateField = 'secret';
  }
  
  #privateField;
  
  static VERSION = VERSION;
  
  async start() {
    await this.initialize();
    return this.run();
  }
  
  initialize() {
    console.log('Initializing...');
  }
  
  run() {
    console.log(`Running ${this.name}`);
  }
  
  get appName() {
    return this.name;
  }
  
  set appName(value) {
    this.name = value;
  }
}

// Arrow functions
export const multiply = (a, b) => a * b;
export const add = (a, b) => {
  return a + b;
};

// Destructuring examples
export function processData({ name, age, ...rest }) {
  const { street, city } = rest.address || {};
  const [first, last] = name.split(' ');
  
  return {
    firstName: first,
    lastName: last,
    age,
    location: `${street}, ${city}`
  };
}

// Variable declarations
let globalVar = 'mutable';
const CONSTANT = 42;
var oldStyleVar = 'legacy';

// Array destructuring
const [item1, item2, ...restItems] = [1, 2, 3, 4, 5];

// Object destructuring with renaming
const { prop1: renamed, prop2 = 'default' } = { prop1: 'value' };

// Default export
export default {
  app,
  startServer,
  Application
};