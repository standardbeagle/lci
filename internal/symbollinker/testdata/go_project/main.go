package main

import (
	"fmt"
	"os"

	"github.com/example/testproject/internal/config"
	"github.com/example/testproject/pkg/utils"

	// External packages
	"github.com/stretchr/testify/assert"
)

func main() {
	fmt.Println("Hello, World!")
	utils.Helper()
	config.Load()
}
