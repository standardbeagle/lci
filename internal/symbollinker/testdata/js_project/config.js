// Configuration module
const env = process.env.NODE_ENV || 'development';

// Named exports
export const API_KEY = process.env.API_KEY || 'dev-key';
export const SERVER_PORT = parseInt(process.env.PORT || '3000');
export const DEBUG = env === 'development';

// Configuration object
const config = {
  env,
  api: {
    key: API_KEY,
    url: process.env.API_URL || 'http://localhost:3000',
    timeout: 5000
  },
  server: {
    port: SERVER_PORT,
    host: process.env.HOST || '0.0.0.0'
  },
  database: {
    url: process.env.DATABASE_URL || 'sqlite://./db.sqlite',
    pool: {
      min: 2,
      max: 10
    }
  },
  features: {
    auth: true,
    logging: DEBUG,
    caching: env === 'production'
  }
};

// Default export
export default config;