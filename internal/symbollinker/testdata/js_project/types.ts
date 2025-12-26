// TypeScript type definitions
export interface User {
  id: number;
  name: string;
  email: string;
  roles: Role[];
  metadata?: UserMetadata;
}

export interface UserMetadata {
  createdAt: Date;
  updatedAt: Date;
  lastLogin?: Date;
}

export type Role = 'admin' | 'user' | 'guest';

export enum Status {
  Active = 'ACTIVE',
  Inactive = 'INACTIVE',
  Pending = 'PENDING',
  Deleted = 'DELETED'
}

export type ID = string | number;

export type Optional<T> = T | null | undefined;

export interface Repository<T> {
  findById(id: ID): Promise<Optional<T>>;
  findAll(): Promise<T[]>;
  save(entity: T): Promise<T>;
  delete(id: ID): Promise<boolean>;
}

export abstract class BaseEntity {
  abstract id: ID;
  abstract createdAt: Date;
  abstract updatedAt: Date;
  
  abstract validate(): boolean;
}

export class UserEntity extends BaseEntity {
  id: ID;
  createdAt: Date;
  updatedAt: Date;
  
  constructor(
    public name: string,
    public email: string,
    private passwordHash: string
  ) {
    super();
    this.id = Date.now().toString();
    this.createdAt = new Date();
    this.updatedAt = new Date();
  }
  
  validate(): boolean {
    return this.email.includes('@');
  }
  
  checkPassword(password: string): boolean {
    // Simplified password check
    return this.passwordHash === password;
  }
  
  static fromJSON(json: any): UserEntity {
    return new UserEntity(json.name, json.email, json.password);
  }
}

export namespace API {
  export interface Request {
    method: string;
    url: string;
    headers?: Record<string, string>;
    body?: any;
  }
  
  export interface Response<T = any> {
    status: number;
    data?: T;
    error?: Error;
  }
  
  export class Client {
    constructor(private baseURL: string) {}
    
    async request<T>(req: Request): Promise<Response<T>> {
      // Implementation
      return { status: 200 };
    }
  }
}

export type Handler<T> = (data: T) => void;
export type AsyncHandler<T> = (data: T) => Promise<void>;

export interface EventEmitter<T> {
  on(event: string, handler: Handler<T>): void;
  off(event: string, handler: Handler<T>): void;
  emit(event: string, data: T): void;
}

// Generic constraints
export interface Comparable<T> {
  compareTo(other: T): number;
}

export function sort<T extends Comparable<T>>(items: T[]): T[] {
  return items.sort((a, b) => a.compareTo(b));
}

// Conditional types
export type IsArray<T> = T extends any[] ? true : false;
export type ElementType<T> = T extends (infer E)[] ? E : T;

// Mapped types
export type Readonly<T> = {
  readonly [P in keyof T]: T[P];
};

export type Partial<T> = {
  [P in keyof T]?: T[P];
};

// Declaration merging
export interface Config {
  apiKey: string;
}

export interface Config {
  timeout: number;
}

// Type guards
export function isUser(obj: any): obj is User {
  return obj && typeof obj.name === 'string' && typeof obj.email === 'string';
}

// Const assertions
export const CONSTANTS = {
  MAX_RETRIES: 3,
  TIMEOUT: 5000,
  API_VERSION: 'v1'
} as const;

export type ConstantKeys = keyof typeof CONSTANTS;