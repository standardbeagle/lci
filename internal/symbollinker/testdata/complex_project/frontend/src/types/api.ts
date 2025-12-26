// API types that match the Go backend models

export interface User {
  id: number;
  name: string;
  email: string;
  created_at: string;
  updated_at: string;
}

export interface Project {
  id: number;
  name: string;
  description: string;
  created_at: string;
  updated_at: string;
  users?: User[];
}

export interface CreateUserRequest {
  name: string;
  email: string;
}

export interface UpdateUserRequest {
  name?: string;
  email?: string;
}

export interface CreateProjectRequest {
  name: string;
  description: string;
}

export interface UpdateProjectRequest {
  name?: string;
  description?: string;
}

export interface ApiResponse<T> {
  data: T;
  error?: string;
}

export interface ApiError {
  error: string;
  code?: number;
}

export interface PaginationMeta {
  page: number;
  per_page: number;
  total: number;
  total_pages: number;
}

export interface PaginatedResponse<T> {
  data: T[];
  meta: PaginationMeta;
}

// API client configuration
export interface ApiClientConfig {
  baseUrl: string;
  timeout: number;
  headers?: Record<string, string>;
}

// Authentication types
export interface LoginRequest {
  email: string;
  password: string;
}

export interface LoginResponse {
  token: string;
  user: User;
  expires_at: string;
}

export interface AuthContext {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
}

// WebSocket message types
export interface WebSocketMessage<T = any> {
  type: string;
  payload: T;
  timestamp: string;
}

export interface UserStatusUpdate {
  user_id: number;
  status: 'online' | 'offline' | 'away';
}

export interface ProjectUpdate {
  project_id: number;
  action: 'created' | 'updated' | 'deleted';
  data: Project;
}