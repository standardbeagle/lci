package testing

import (
	"github.com/standardbeagle/lci/internal/config"
)

// ProjectGenerator interface for creating test projects
type ProjectGenerator interface {
	CreateProject(basePath string)
	GetProjectConfig() *config.Config
}

// NodeProjectGenerator creates Node.js test projects
type NodeProjectGenerator struct {
	patterns []string
	files    map[string]string
}

// NewNodeProjectGenerator creates a new Node.js project generator
func NewNodeProjectGenerator(gitignorePatterns ...string) *NodeProjectGenerator {
	patterns := gitignorePatterns
	if len(patterns) == 0 {
		patterns = []string{
			"node_modules/",
			"dist/",
			"build/",
			"*.log",
		}
	}

	return &NodeProjectGenerator{
		patterns: patterns,
		files:    NodeProjectFiles(),
	}
}

// CreateProject creates a Node.js project structure
func (gen *NodeProjectGenerator) CreateProject(basePath string) {
	// Create project files
	for path, content := range gen.files {
		_ = CreateTestFile(basePath, path, content)
	}
}

// GetProjectConfig returns the project configuration
func (gen *NodeProjectGenerator) GetProjectConfig() *config.Config {
	return &config.Config{
		Project: config.Project{
			Root: ".",
			Name: "node-test-project",
		},
		Index: config.Index{
			MaxFileSize:      1024 * 1024,
			RespectGitignore: true,
		},
		Performance: config.Performance{
			MaxMemoryMB:   500,
			MaxGoroutines: 4,
		},
	}
}

// GoProjectGenerator creates Go test projects
type GoProjectGenerator struct {
	patterns []string
	files    map[string]string
}

// NewGoProjectGenerator creates a new Go project generator
func NewGoProjectGenerator(gitignorePatterns ...string) *GoProjectGenerator {
	patterns := gitignorePatterns
	if len(patterns) == 0 {
		patterns = []string{
			"vendor/",
			"*.exe",
			"*.out",
			"*.test",
		}
	}

	return &GoProjectGenerator{
		patterns: patterns,
		files:    GoProjectFiles(),
	}
}

// CreateProject creates a Go project structure
func (gen *GoProjectGenerator) CreateProject(basePath string) {
	// Create project files
	for path, content := range gen.files {
		_ = CreateTestFile(basePath, path, content)
	}
}

// GetProjectConfig returns the project configuration
func (gen *GoProjectGenerator) GetProjectConfig() *config.Config {
	return &config.Config{
		Project: config.Project{
			Root: ".",
			Name: "go-test-project",
		},
		Index: config.Index{
			MaxFileSize:      1024 * 1024,
			RespectGitignore: true,
		},
		Performance: config.Performance{
			MaxMemoryMB:   500,
			MaxGoroutines: 4,
		},
	}
}

// WebProjectGenerator creates modern web project structures
type WebProjectGenerator struct {
	patterns []string
	files    map[string]string
}

// NewWebProjectGenerator creates a new web project generator
func NewWebProjectGenerator(gitignorePatterns ...string) *WebProjectGenerator {
	patterns := gitignorePatterns
	if len(patterns) == 0 {
		patterns = []string{
			".next/",
			"node_modules/",
			"out/",
			"dist/",
			"build/",
			".env.local",
		}
	}

	return &WebProjectGenerator{
		patterns: patterns,
		files:    webProjectFiles(),
	}
}

// CreateProject creates a web project structure
func (gen *WebProjectGenerator) CreateProject(basePath string) {
	// Create project files
	for path, content := range gen.files {
		_ = CreateTestFile(basePath, path, content)
	}
}

// GetProjectConfig returns the project configuration
func (gen *WebProjectGenerator) GetProjectConfig() *config.Config {
	return &config.Config{
		Project: config.Project{
			Root: ".",
			Name: "web-test-project",
		},
		Index: config.Index{
			MaxFileSize:      1024 * 1024,
			RespectGitignore: true,
		},
		Performance: config.Performance{
			MaxMemoryMB:   500,
			MaxGoroutines: 4,
		},
	}
}

// webProjectFiles returns files for a modern web project
func webProjectFiles() map[string]string {
	return map[string]string{
		"src/app/page.tsx": `export default function Home() {
    return (
        <main>
            <h1>Welcome to My App</h1>
            <p>This is the homepage</p>
        </main>
    );
}`,

		"src/components/Button.tsx": `interface ButtonProps {
    children: React.ReactNode;
    onClick: () => void;
    variant?: 'primary' | 'secondary';
}

export function Button({ children, onClick, variant = 'primary' }: ButtonProps) {
    return (
        <button
            className={"btn btn-" + variant}
            onClick={onClick}
        >
            {children}
        </button>
    );
}`,

		"src/lib/api.ts": `interface User {
    id: number;
    name: string;
    email: string;
}

class ApiService {
    private baseUrl: string;

    constructor(baseUrl: string = '/api') {
        this.baseUrl = baseUrl;
    }

    async getUsers(): Promise<User[]> {
        const response = await fetch(this.baseUrl + "/users");
        return response.json();
    }

    async createUser(user: Omit<User, 'id'>): Promise<User> {
        const response = await fetch(this.baseUrl + "/users", {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(user),
        });
        return response.json();
    }
}

export const apiService = new ApiService();`,

		"package.json": `{
  "name": "web-project",
  "version": "0.1.0",
  "private": true,
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "start": "next start",
    "lint": "next lint"
  },
  "dependencies": {
    "next": "13.0.0",
    "react": "18.0.0",
    "react-dom": "18.0.0"
  }
}`,
	}
}
