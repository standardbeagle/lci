// Test file with various masterData pattern scenarios
package main

import "fmt"

// Example structs with masterData patterns
type ViewController struct {
	viewManager *ViewManager
	masterData  []DataModel
	state       string
	keys        []string
	width       int
	height      int
}

func (vc *ViewController) Setup() {
	// These should match the pattern: \.(masterData|state|viewManager|keys|width|height)\s*=
	vc.masterData = []DataModel{{}}
	vc.state = "initialized"
	vc.viewManager = &ViewManager{}
	vc.keys = []string{"key1", "key2"}
	vc.width = 100
	vc.height = 200
}

type ViewManager struct {
	viewState    string
	elementKeys  []string
	canvasWidth  float64
	canvasHeight float64
}

func (vm *ViewManager) Configure() {
	vm.viewState = "active"
	vm.elementKeys = []string{"button", "label"}
	vm.canvasWidth = 1024.0
	vm.canvasHeight = 768.0
}

type DataModel struct {
	ID   string
	Data map[string]interface{}
}

func main() {
	vc := &ViewController{}
	vc.Setup()
	fmt.Println("Setup complete")
}
