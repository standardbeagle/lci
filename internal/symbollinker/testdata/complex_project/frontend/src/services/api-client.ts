import axios, { AxiosInstance, AxiosResponse } from 'axios';
import {
  User,
  Project,
  CreateUserRequest,
  UpdateUserRequest,
  CreateProjectRequest,
  UpdateProjectRequest,
  ApiResponse,
  PaginatedResponse,
  LoginRequest,
  LoginResponse,
  ApiClientConfig
} from '../types/api';

export class ApiClient {
  private client: AxiosInstance;
  private config: ApiClientConfig;

  constructor(config: Partial<ApiClientConfig> = {}) {
    this.config = {
      baseUrl: config.baseUrl || 'http://localhost:8080/api',
      timeout: config.timeout || 10000,
      headers: config.headers || {}
    };

    this.client = axios.create({
      baseURL: this.config.baseUrl,
      timeout: this.config.timeout,
      headers: {
        'Content-Type': 'application/json',
        ...this.config.headers
      }
    });

    this.setupInterceptors();
  }

  private setupInterceptors(): void {
    // Request interceptor to add auth token
    this.client.interceptors.request.use(
      (config) => {
        const token = localStorage.getItem('auth_token');
        if (token) {
          config.headers.Authorization = `Bearer ${token}`;
        }
        return config;
      },
      (error) => Promise.reject(error)
    );

    // Response interceptor to handle common errors
    this.client.interceptors.response.use(
      (response) => response,
      (error) => {
        if (error.response?.status === 401) {
          // Handle unauthorized access
          localStorage.removeItem('auth_token');
          window.location.href = '/login';
        }
        return Promise.reject(error);
      }
    );
  }

  // Authentication methods
  async login(credentials: LoginRequest): Promise<LoginResponse> {
    const response = await this.client.post<LoginResponse>('/auth/login', credentials);
    return response.data;
  }

  async logout(): Promise<void> {
    await this.client.post('/auth/logout');
    localStorage.removeItem('auth_token');
  }

  async refreshToken(): Promise<{ token: string }> {
    const response = await this.client.post<{ token: string }>('/auth/refresh');
    return response.data;
  }

  // User methods
  async getUsers(): Promise<User[]> {
    const response = await this.client.get<User[]>('/users');
    return response.data;
  }

  async getUser(id: number): Promise<User> {
    const response = await this.client.get<User>(`/users/${id}`);
    return response.data;
  }

  async createUser(userData: CreateUserRequest): Promise<User> {
    const response = await this.client.post<User>('/users', userData);
    return response.data;
  }

  async updateUser(id: number, userData: UpdateUserRequest): Promise<User> {
    const response = await this.client.put<User>(`/users/${id}`, userData);
    return response.data;
  }

  async deleteUser(id: number): Promise<void> {
    await this.client.delete(`/users/${id}`);
  }

  async getUserProjects(userId: number): Promise<Project[]> {
    const response = await this.client.get<Project[]>(`/users/${userId}/projects`);
    return response.data;
  }

  // Project methods
  async getProjects(): Promise<Project[]> {
    const response = await this.client.get<Project[]>('/projects');
    return response.data;
  }

  async getProject(id: number): Promise<Project> {
    const response = await this.client.get<Project>(`/projects/${id}`);
    return response.data;
  }

  async createProject(projectData: CreateProjectRequest): Promise<Project> {
    const response = await this.client.post<Project>('/projects', projectData);
    return response.data;
  }

  async updateProject(id: number, projectData: UpdateProjectRequest): Promise<Project> {
    const response = await this.client.put<Project>(`/projects/${id}`, projectData);
    return response.data;
  }

  async deleteProject(id: number): Promise<void> {
    await this.client.delete(`/projects/${id}`);
  }

  async addUserToProject(projectId: number, userId: number): Promise<void> {
    await this.client.post(`/projects/${projectId}/users/${userId}`);
  }

  async removeUserFromProject(projectId: number, userId: number): Promise<void> {
    await this.client.delete(`/projects/${projectId}/users/${userId}`);
  }

  async getProjectUsers(projectId: number): Promise<User[]> {
    const response = await this.client.get<User[]>(`/projects/${projectId}/users`);
    return response.data;
  }

  // Generic methods for handling paginated responses
  async getPaginated<T>(
    endpoint: string,
    page: number = 1,
    perPage: number = 20
  ): Promise<PaginatedResponse<T>> {
    const response = await this.client.get<PaginatedResponse<T>>(endpoint, {
      params: { page, per_page: perPage }
    });
    return response.data;
  }

  // File upload methods
  async uploadFile(file: File, endpoint: string = '/files'): Promise<{ url: string }> {
    const formData = new FormData();
    formData.append('file', file);

    const response = await this.client.post<{ url: string }>(endpoint, formData, {
      headers: {
        'Content-Type': 'multipart/form-data'
      },
      onUploadProgress: (progressEvent) => {
        const progress = progressEvent.total 
          ? Math.round((progressEvent.loaded * 100) / progressEvent.total)
          : 0;
        console.log(`Upload progress: ${progress}%`);
      }
    });

    return response.data;
  }

  // Health check
  async healthCheck(): Promise<{ status: string; timestamp: string }> {
    const response = await this.client.get<{ status: string; timestamp: string }>('/health');
    return response.data;
  }

  // Update configuration
  updateConfig(newConfig: Partial<ApiClientConfig>): void {
    this.config = { ...this.config, ...newConfig };
    
    if (newConfig.baseUrl) {
      this.client.defaults.baseURL = newConfig.baseUrl;
    }
    
    if (newConfig.timeout) {
      this.client.defaults.timeout = newConfig.timeout;
    }
    
    if (newConfig.headers) {
      this.client.defaults.headers = {
        ...this.client.defaults.headers,
        ...newConfig.headers
      };
    }
  }
}

// Singleton instance
export const apiClient = new ApiClient();

// Export for testing
export default ApiClient;