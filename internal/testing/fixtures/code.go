package fixtures

import (
	"fmt"
	"strconv"
	"strings"
)

// TestFile represents a file for testing
type TestFile struct {
	Path    string
	Content string
}

// GenerateSimpleTestFiles creates test files with realistic content
func GenerateSimpleTestFiles(count int) []TestFile {
	files := make([]TestFile, count)

	templates := []struct {
		ext     string
		content string
	}{
		{
			ext: "go",
			content: `package pkg%d

import "fmt"

// Function%d does something
func Function%d() {
	fmt.Println("Function %d")
}

type Struct%d struct {
	Field1 string
	Field2 int
}

func (s *Struct%d) Method%d() {
	// Method implementation
}
`,
		},
		{
			ext: "js",
			content: `// Module %d
class Class%d {
	constructor() {
		this.property = %d;
	}
	
	method%d() {
		console.log("Method %d");
	}
}

function function%d() {
	return %d;
}

const variable%d = %d;
`,
		},
		{
			ext: "py",
			content: `# Module %d

class Class%d:
	def __init__(self):
		self.property = %d
	
	def method%d(self):
		print("Method %d")

def function%d():
	return %d

variable%d = %d
`,
		},
	}

	for i := 0; i < count; i++ {
		template := templates[i%len(templates)]
		n := i + 1

		content := template.content
		// Replace all %d with the number
		for strings.Contains(content, "%d") {
			content = strings.Replace(content, "%d", strconv.Itoa(n), 1)
		}

		files[i] = TestFile{
			Path:    fmt.Sprintf("file%d.%s", n, template.ext),
			Content: content,
		}
	}

	return files
}
