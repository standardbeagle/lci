package analysis

import "github.com/standardbeagle/lci/internal/types"

// Known Functions Database
//
// This file contains curated lists of functions with known purity characteristics.
// These are used to anchor the side-effect analysis with high-confidence information.
//
// IMPORTANT: Only add functions here that are DEFINITELY pure or DEFINITELY have side effects.
// When in doubt, leave it out - the analysis will conservatively mark unknowns as impure.

// KnownPureFunctions maps "language:package.function" to true for functions
// that are known to be pure (no side effects, deterministic output for same input).
//
// Criteria for inclusion:
// - No I/O operations
// - No mutation of parameters
// - No global state access
// - Deterministic (same input -> same output)
// - No panics on valid input
var KnownPureFunctions = map[string]bool{
	// Go standard library - strings
	"go:strings.ToLower":      true,
	"go:strings.ToUpper":      true,
	"go:strings.TrimSpace":    true,
	"go:strings.Trim":         true,
	"go:strings.TrimLeft":     true,
	"go:strings.TrimRight":    true,
	"go:strings.TrimPrefix":   true,
	"go:strings.TrimSuffix":   true,
	"go:strings.HasPrefix":    true,
	"go:strings.HasSuffix":    true,
	"go:strings.Contains":     true,
	"go:strings.ContainsAny":  true,
	"go:strings.ContainsRune": true,
	"go:strings.Count":        true,
	"go:strings.Index":        true,
	"go:strings.IndexAny":     true,
	"go:strings.IndexByte":    true,
	"go:strings.IndexRune":    true,
	"go:strings.LastIndex":    true,
	"go:strings.LastIndexAny": true,
	"go:strings.Split":        true,
	"go:strings.SplitN":       true,
	"go:strings.SplitAfter":   true,
	"go:strings.Fields":       true,
	"go:strings.Join":         true,
	"go:strings.Repeat":       true,
	"go:strings.Replace":      true,
	"go:strings.ReplaceAll":   true,
	"go:strings.EqualFold":    true,
	"go:strings.Compare":      true,
	"go:strings.Clone":        true,

	// Go standard library - strconv
	"go:strconv.Itoa":        true,
	"go:strconv.Atoi":        true, // Can return error but no side effects
	"go:strconv.ParseInt":    true,
	"go:strconv.ParseUint":   true,
	"go:strconv.ParseFloat":  true,
	"go:strconv.ParseBool":   true,
	"go:strconv.FormatInt":   true,
	"go:strconv.FormatUint":  true,
	"go:strconv.FormatFloat": true,
	"go:strconv.FormatBool":  true,
	"go:strconv.Quote":       true,
	"go:strconv.Unquote":     true,

	// Go standard library - math
	"go:math.Abs":       true,
	"go:math.Ceil":      true,
	"go:math.Floor":     true,
	"go:math.Round":     true,
	"go:math.Trunc":     true,
	"go:math.Sqrt":      true,
	"go:math.Cbrt":      true,
	"go:math.Pow":       true,
	"go:math.Exp":       true,
	"go:math.Log":       true,
	"go:math.Log10":     true,
	"go:math.Log2":      true,
	"go:math.Sin":       true,
	"go:math.Cos":       true,
	"go:math.Tan":       true,
	"go:math.Asin":      true,
	"go:math.Acos":      true,
	"go:math.Atan":      true,
	"go:math.Atan2":     true,
	"go:math.Sinh":      true,
	"go:math.Cosh":      true,
	"go:math.Tanh":      true,
	"go:math.Max":       true,
	"go:math.Min":       true,
	"go:math.Mod":       true,
	"go:math.Remainder": true,
	"go:math.IsNaN":     true,
	"go:math.IsInf":     true,

	// Go standard library - bytes
	"go:bytes.Equal":      true,
	"go:bytes.Compare":    true,
	"go:bytes.Contains":   true,
	"go:bytes.Count":      true,
	"go:bytes.HasPrefix":  true,
	"go:bytes.HasSuffix":  true,
	"go:bytes.Index":      true,
	"go:bytes.IndexByte":  true,
	"go:bytes.LastIndex":  true,
	"go:bytes.Split":      true,
	"go:bytes.SplitN":     true,
	"go:bytes.Join":       true,
	"go:bytes.Repeat":     true,
	"go:bytes.ToLower":    true,
	"go:bytes.ToUpper":    true,
	"go:bytes.TrimSpace":  true,
	"go:bytes.Trim":       true,
	"go:bytes.TrimLeft":   true,
	"go:bytes.TrimRight":  true,
	"go:bytes.TrimPrefix": true,
	"go:bytes.TrimSuffix": true,
	"go:bytes.Replace":    true,
	"go:bytes.ReplaceAll": true,
	"go:bytes.Clone":      true,

	// Go standard library - unicode
	"go:unicode.IsLetter":  true,
	"go:unicode.IsDigit":   true,
	"go:unicode.IsNumber":  true,
	"go:unicode.IsSpace":   true,
	"go:unicode.IsUpper":   true,
	"go:unicode.IsLower":   true,
	"go:unicode.IsPunct":   true,
	"go:unicode.IsSymbol":  true,
	"go:unicode.IsControl": true,
	"go:unicode.ToLower":   true,
	"go:unicode.ToUpper":   true,
	"go:unicode.ToTitle":   true,

	// Go standard library - path/filepath
	"go:filepath.Base":      true,
	"go:filepath.Dir":       true,
	"go:filepath.Ext":       true,
	"go:filepath.Clean":     true,
	"go:filepath.Join":      true,
	"go:filepath.Split":     true,
	"go:filepath.SplitList": true,
	"go:filepath.IsAbs":     true,
	"go:filepath.Match":     true,
	"go:filepath.Rel":       true,

	// Go standard library - path
	"go:path.Base":  true,
	"go:path.Dir":   true,
	"go:path.Ext":   true,
	"go:path.Clean": true,
	"go:path.Join":  true,
	"go:path.Split": true,
	"go:path.Match": true,

	// Go standard library - regexp (methods that don't modify state)
	"go:regexp.MustCompile": true, // Panics on error but pure otherwise
	"go:regexp.Compile":     true,
	"go:regexp.QuoteMeta":   true,

	// Go standard library - encoding/json (pure analysis functions)
	"go:json.Valid": true,

	// Go standard library - sort (pure comparison/search functions)
	"go:sort.Search":            true,
	"go:sort.SearchInts":        true,
	"go:sort.SearchFloat64s":    true,
	"go:sort.SearchStrings":     true,
	"go:sort.IntsAreSorted":     true,
	"go:sort.Float64sAreSorted": true,
	"go:sort.StringsAreSorted":  true,

	// Go builtins
	"go:len":     true,
	"go:cap":     true,
	"go:real":    true,
	"go:imag":    true,
	"go:complex": true,
	"go:new":     true, // Allocates but no observable side effects
	"go:make":    true, // Allocates but no observable side effects
	"go:append":  true, // Returns new slice, doesn't mutate original
	"go:copy":    true, // Copies elements but returns count, no external side effects
	"go:max":     true, // Go 1.21+ builtin for max value
	"go:min":     true, // Go 1.21+ builtin for min value
	"go:clear":   true, // Go 1.21+ clears map/slice but receiver-only mutation

	// JavaScript - String methods (return new strings, don't mutate)
	"javascript:String.prototype.toLowerCase":   true,
	"javascript:String.prototype.toUpperCase":   true,
	"javascript:String.prototype.trim":          true,
	"javascript:String.prototype.trimStart":     true,
	"javascript:String.prototype.trimEnd":       true,
	"javascript:String.prototype.slice":         true,
	"javascript:String.prototype.substring":     true,
	"javascript:String.prototype.substr":        true,
	"javascript:String.prototype.split":         true,
	"javascript:String.prototype.charAt":        true,
	"javascript:String.prototype.charCodeAt":    true,
	"javascript:String.prototype.indexOf":       true,
	"javascript:String.prototype.lastIndexOf":   true,
	"javascript:String.prototype.includes":      true,
	"javascript:String.prototype.startsWith":    true,
	"javascript:String.prototype.endsWith":      true,
	"javascript:String.prototype.replace":       true,
	"javascript:String.prototype.replaceAll":    true,
	"javascript:String.prototype.concat":        true,
	"javascript:String.prototype.repeat":        true,
	"javascript:String.prototype.padStart":      true,
	"javascript:String.prototype.padEnd":        true,
	"javascript:String.prototype.normalize":     true,
	"javascript:String.prototype.localeCompare": true,
	"javascript:String.prototype.match":         true,
	"javascript:String.prototype.search":        true,

	// JavaScript - Array methods that don't mutate (return new arrays)
	"javascript:Array.prototype.slice":       true,
	"javascript:Array.prototype.concat":      true,
	"javascript:Array.prototype.indexOf":     true,
	"javascript:Array.prototype.includes":    true,
	"javascript:Array.prototype.find":        true,
	"javascript:Array.prototype.findIndex":   true,
	"javascript:Array.prototype.filter":      true,
	"javascript:Array.prototype.map":         true,
	"javascript:Array.prototype.reduce":      true,
	"javascript:Array.prototype.reduceRight": true,
	"javascript:Array.prototype.every":       true,
	"javascript:Array.prototype.some":        true,
	"javascript:Array.prototype.join":        true,
	"javascript:Array.prototype.flat":        true,
	"javascript:Array.prototype.flatMap":     true,
	"javascript:Array.prototype.toReversed":  true, // ES2023 - non-mutating
	"javascript:Array.prototype.toSorted":    true, // ES2023 - non-mutating
	"javascript:Array.prototype.toSpliced":   true, // ES2023 - non-mutating
	"javascript:Array.prototype.with":        true, // ES2023 - non-mutating
	"javascript:Array.isArray":               true,
	"javascript:Array.from":                  true,
	"javascript:Array.of":                    true,

	// JavaScript - Math
	"javascript:Math.abs":   true,
	"javascript:Math.ceil":  true,
	"javascript:Math.floor": true,
	"javascript:Math.round": true,
	"javascript:Math.trunc": true,
	"javascript:Math.sqrt":  true,
	"javascript:Math.cbrt":  true,
	"javascript:Math.pow":   true,
	"javascript:Math.exp":   true,
	"javascript:Math.log":   true,
	"javascript:Math.log10": true,
	"javascript:Math.log2":  true,
	"javascript:Math.sin":   true,
	"javascript:Math.cos":   true,
	"javascript:Math.tan":   true,
	"javascript:Math.asin":  true,
	"javascript:Math.acos":  true,
	"javascript:Math.atan":  true,
	"javascript:Math.atan2": true,
	"javascript:Math.max":   true,
	"javascript:Math.min":   true,
	"javascript:Math.sign":  true,
	"javascript:Math.hypot": true,

	// JavaScript - Object (some methods)
	"javascript:Object.keys":        true,
	"javascript:Object.values":      true,
	"javascript:Object.entries":     true,
	"javascript:Object.fromEntries": true,
	"javascript:Object.hasOwn":      true,
	"javascript:Object.is":          true,

	// JavaScript - JSON
	"javascript:JSON.parse":     true, // Can throw but no side effects
	"javascript:JSON.stringify": true,

	// JavaScript - Number
	"javascript:Number.isNaN":      true,
	"javascript:Number.isFinite":   true,
	"javascript:Number.isInteger":  true,
	"javascript:Number.parseInt":   true,
	"javascript:Number.parseFloat": true,
	"javascript:parseInt":          true,
	"javascript:parseFloat":        true,
	"javascript:isNaN":             true,
	"javascript:isFinite":          true,

	// TypeScript - same as JavaScript
	"typescript:String.prototype.toLowerCase": true,
	"typescript:String.prototype.toUpperCase": true,
	"typescript:Math.abs":                     true,
	"typescript:Math.sqrt":                    true,
	// ... (inherit from JavaScript)

	// Python - built-in functions
	"python:len":        true,
	"python:abs":        true,
	"python:round":      true,
	"python:min":        true,
	"python:max":        true,
	"python:sum":        true,
	"python:sorted":     true, // Returns new list
	"python:reversed":   true, // Returns iterator
	"python:enumerate":  true,
	"python:zip":        true,
	"python:map":        true,
	"python:filter":     true,
	"python:range":      true,
	"python:list":       true, // Constructor
	"python:tuple":      true, // Constructor
	"python:dict":       true, // Constructor
	"python:set":        true, // Constructor
	"python:frozenset":  true,
	"python:str":        true,
	"python:int":        true,
	"python:float":      true,
	"python:bool":       true,
	"python:type":       true,
	"python:isinstance": true,
	"python:issubclass": true,
	"python:hasattr":    true,
	"python:getattr":    true,
	"python:id":         true,
	"python:hash":       true,
	"python:repr":       true,
	"python:ascii":      true,
	"python:chr":        true,
	"python:ord":        true,
	"python:hex":        true,
	"python:oct":        true,
	"python:bin":        true,
	"python:format":     true,
	"python:all":        true,
	"python:any":        true,
}

// KnownIOFunctions maps "language:package.function" to the type of I/O they perform
var KnownIOFunctions = map[string]types.SideEffectCategory{
	// Go - fmt (console I/O)
	"go:fmt.Print":    types.SideEffectIO,
	"go:fmt.Println":  types.SideEffectIO,
	"go:fmt.Printf":   types.SideEffectIO,
	"go:fmt.Fprint":   types.SideEffectIO,
	"go:fmt.Fprintln": types.SideEffectIO,
	"go:fmt.Fprintf":  types.SideEffectIO,
	"go:fmt.Sprint":   types.SideEffectNone, // Returns string, no I/O
	"go:fmt.Sprintln": types.SideEffectNone,
	"go:fmt.Sprintf":  types.SideEffectNone,
	"go:fmt.Scan":     types.SideEffectIO,
	"go:fmt.Scanln":   types.SideEffectIO,
	"go:fmt.Scanf":    types.SideEffectIO,
	"go:fmt.Fscan":    types.SideEffectIO,
	"go:fmt.Fscanln":  types.SideEffectIO,
	"go:fmt.Fscanf":   types.SideEffectIO,
	"go:fmt.Sscan":    types.SideEffectNone, // Reads from string, not I/O
	"go:fmt.Sscanln":  types.SideEffectNone,
	"go:fmt.Sscanf":   types.SideEffectNone,
	"go:fmt.Errorf":   types.SideEffectNone, // Returns error, no I/O

	// Go - os (file I/O)
	"go:os.Open":      types.SideEffectIO,
	"go:os.OpenFile":  types.SideEffectIO,
	"go:os.Create":    types.SideEffectIO,
	"go:os.ReadFile":  types.SideEffectIO,
	"go:os.WriteFile": types.SideEffectIO,
	"go:os.Remove":    types.SideEffectIO,
	"go:os.RemoveAll": types.SideEffectIO,
	"go:os.Rename":    types.SideEffectIO,
	"go:os.Mkdir":     types.SideEffectIO,
	"go:os.MkdirAll":  types.SideEffectIO,
	"go:os.Chdir":     types.SideEffectIO,
	"go:os.Chmod":     types.SideEffectIO,
	"go:os.Chown":     types.SideEffectIO,
	"go:os.Chtimes":   types.SideEffectIO,
	"go:os.Link":      types.SideEffectIO,
	"go:os.Symlink":   types.SideEffectIO,
	"go:os.Truncate":  types.SideEffectIO,
	"go:os.Stat":      types.SideEffectIO,
	"go:os.Lstat":     types.SideEffectIO,
	"go:os.ReadDir":   types.SideEffectIO,
	"go:os.Getwd":     types.SideEffectIO,
	"go:os.Getenv":    types.SideEffectIO, // Reads environment
	"go:os.Setenv":    types.SideEffectIO | types.SideEffectGlobalWrite,
	"go:os.Unsetenv":  types.SideEffectIO | types.SideEffectGlobalWrite,
	"go:os.Exit":      types.SideEffectIO,
	"go:os.Stdout":    types.SideEffectIO, // Variable access for I/O
	"go:os.Stderr":    types.SideEffectIO,
	"go:os.Stdin":     types.SideEffectIO,

	// Go - io
	"go:io.Copy":        types.SideEffectIO,
	"go:io.CopyN":       types.SideEffectIO,
	"go:io.CopyBuffer":  types.SideEffectIO,
	"go:io.ReadAll":     types.SideEffectIO,
	"go:io.ReadFull":    types.SideEffectIO,
	"go:io.WriteString": types.SideEffectIO,

	// Go - io/ioutil (deprecated but still used)
	"go:ioutil.ReadFile":  types.SideEffectIO,
	"go:ioutil.WriteFile": types.SideEffectIO,
	"go:ioutil.ReadAll":   types.SideEffectIO,
	"go:ioutil.ReadDir":   types.SideEffectIO,
	"go:ioutil.TempDir":   types.SideEffectIO,
	"go:ioutil.TempFile":  types.SideEffectIO,

	// Go - net/http (network I/O)
	"go:http.Get":            types.SideEffectNetwork,
	"go:http.Post":           types.SideEffectNetwork,
	"go:http.PostForm":       types.SideEffectNetwork,
	"go:http.Head":           types.SideEffectNetwork,
	"go:http.ListenAndServe": types.SideEffectNetwork,
	"go:http.Serve":          types.SideEffectNetwork,
	"go:http.Handle":         types.SideEffectGlobalWrite, // Modifies global mux
	"go:http.HandleFunc":     types.SideEffectGlobalWrite,

	// Go - database/sql
	"go:sql.Open":  types.SideEffectDatabase,
	"go:sql.Query": types.SideEffectDatabase,
	"go:sql.Exec":  types.SideEffectDatabase,

	// Go - log
	"go:log.Print":   types.SideEffectIO,
	"go:log.Println": types.SideEffectIO,
	"go:log.Printf":  types.SideEffectIO,
	"go:log.Fatal":   types.SideEffectIO,
	"go:log.Fatalf":  types.SideEffectIO,
	"go:log.Fatalln": types.SideEffectIO,
	"go:log.Panic":   types.SideEffectIO | types.SideEffectThrow,
	"go:log.Panicf":  types.SideEffectIO | types.SideEffectThrow,
	"go:log.Panicln": types.SideEffectIO | types.SideEffectThrow,

	// Go - panic/recover
	"go:panic":   types.SideEffectThrow,
	"go:recover": types.SideEffectNone, // Reading panic value

	// JavaScript - console
	"javascript:console.log":     types.SideEffectIO,
	"javascript:console.error":   types.SideEffectIO,
	"javascript:console.warn":    types.SideEffectIO,
	"javascript:console.info":    types.SideEffectIO,
	"javascript:console.debug":   types.SideEffectIO,
	"javascript:console.trace":   types.SideEffectIO,
	"javascript:console.dir":     types.SideEffectIO,
	"javascript:console.table":   types.SideEffectIO,
	"javascript:console.time":    types.SideEffectIO,
	"javascript:console.timeEnd": types.SideEffectIO,
	"javascript:console.clear":   types.SideEffectIO,

	// JavaScript - fetch/network
	"javascript:fetch":          types.SideEffectNetwork | types.SideEffectAsync,
	"javascript:XMLHttpRequest": types.SideEffectNetwork,

	// JavaScript - DOM (side effects)
	"javascript:document.write":                    types.SideEffectIO,
	"javascript:document.writeln":                  types.SideEffectIO,
	"javascript:document.createElement":            types.SideEffectNone, // Creates but doesn't attach
	"javascript:document.getElementById":           types.SideEffectNone, // Read-only query
	"javascript:document.querySelector":            types.SideEffectNone,
	"javascript:document.querySelectorAll":         types.SideEffectNone,
	"javascript:Element.prototype.appendChild":     types.SideEffectIO, // Modifies DOM
	"javascript:Element.prototype.removeChild":     types.SideEffectIO,
	"javascript:Element.prototype.insertBefore":    types.SideEffectIO,
	"javascript:Element.prototype.replaceChild":    types.SideEffectIO,
	"javascript:Element.prototype.remove":          types.SideEffectIO,
	"javascript:Element.prototype.setAttribute":    types.SideEffectIO,
	"javascript:Element.prototype.removeAttribute": types.SideEffectIO,

	// JavaScript - localStorage/sessionStorage
	"javascript:localStorage.getItem":      types.SideEffectIO,
	"javascript:localStorage.setItem":      types.SideEffectIO,
	"javascript:localStorage.removeItem":   types.SideEffectIO,
	"javascript:localStorage.clear":        types.SideEffectIO,
	"javascript:sessionStorage.getItem":    types.SideEffectIO,
	"javascript:sessionStorage.setItem":    types.SideEffectIO,
	"javascript:sessionStorage.removeItem": types.SideEffectIO,
	"javascript:sessionStorage.clear":      types.SideEffectIO,

	// JavaScript - timers
	"javascript:setTimeout":            types.SideEffectAsync,
	"javascript:setInterval":           types.SideEffectAsync,
	"javascript:clearTimeout":          types.SideEffectAsync,
	"javascript:clearInterval":         types.SideEffectAsync,
	"javascript:requestAnimationFrame": types.SideEffectAsync,

	// JavaScript - throw
	"javascript:throw": types.SideEffectThrow,

	// JavaScript - Array mutating methods
	"javascript:Array.prototype.push":       types.SideEffectParamWrite,
	"javascript:Array.prototype.pop":        types.SideEffectParamWrite,
	"javascript:Array.prototype.shift":      types.SideEffectParamWrite,
	"javascript:Array.prototype.unshift":    types.SideEffectParamWrite,
	"javascript:Array.prototype.splice":     types.SideEffectParamWrite,
	"javascript:Array.prototype.sort":       types.SideEffectParamWrite, // Mutates in place
	"javascript:Array.prototype.reverse":    types.SideEffectParamWrite,
	"javascript:Array.prototype.fill":       types.SideEffectParamWrite,
	"javascript:Array.prototype.copyWithin": types.SideEffectParamWrite,

	// Python - print/I/O
	"python:print":      types.SideEffectIO,
	"python:input":      types.SideEffectIO,
	"python:open":       types.SideEffectIO,
	"python:read":       types.SideEffectIO,
	"python:write":      types.SideEffectIO,
	"python:readline":   types.SideEffectIO,
	"python:readlines":  types.SideEffectIO,
	"python:writelines": types.SideEffectIO,

	// Python - os module
	"python:os.getcwd":      types.SideEffectIO,
	"python:os.chdir":       types.SideEffectIO,
	"python:os.listdir":     types.SideEffectIO,
	"python:os.mkdir":       types.SideEffectIO,
	"python:os.makedirs":    types.SideEffectIO,
	"python:os.remove":      types.SideEffectIO,
	"python:os.rmdir":       types.SideEffectIO,
	"python:os.rename":      types.SideEffectIO,
	"python:os.path.exists": types.SideEffectIO,
	"python:os.path.isfile": types.SideEffectIO,
	"python:os.path.isdir":  types.SideEffectIO,

	// Python - exceptions
	"python:raise": types.SideEffectThrow,

	// Python - mutating list methods
	"python:list.append":  types.SideEffectParamWrite,
	"python:list.extend":  types.SideEffectParamWrite,
	"python:list.insert":  types.SideEffectParamWrite,
	"python:list.remove":  types.SideEffectParamWrite,
	"python:list.pop":     types.SideEffectParamWrite,
	"python:list.clear":   types.SideEffectParamWrite,
	"python:list.sort":    types.SideEffectParamWrite,
	"python:list.reverse": types.SideEffectParamWrite,

	// Python - mutating dict methods
	"python:dict.update":     types.SideEffectParamWrite,
	"python:dict.pop":        types.SideEffectParamWrite,
	"python:dict.popitem":    types.SideEffectParamWrite,
	"python:dict.clear":      types.SideEffectParamWrite,
	"python:dict.setdefault": types.SideEffectParamWrite,

	// Python - mutating set methods
	"python:set.add":                         types.SideEffectParamWrite,
	"python:set.remove":                      types.SideEffectParamWrite,
	"python:set.discard":                     types.SideEffectParamWrite,
	"python:set.pop":                         types.SideEffectParamWrite,
	"python:set.clear":                       types.SideEffectParamWrite,
	"python:set.update":                      types.SideEffectParamWrite,
	"python:set.intersection_update":         types.SideEffectParamWrite,
	"python:set.difference_update":           types.SideEffectParamWrite,
	"python:set.symmetric_difference_update": types.SideEffectParamWrite,
}

// IsKnownPure checks if a function is known to be pure
func IsKnownPure(language, qualifiedName string) bool {
	key := language + ":" + qualifiedName
	return KnownPureFunctions[key]
}

// GetKnownSideEffects returns the known side effects for a function, or SideEffectUncertain if unknown
func GetKnownSideEffects(language, qualifiedName string) types.SideEffectCategory {
	key := language + ":" + qualifiedName
	if IsKnownPure(language, qualifiedName) {
		return types.SideEffectNone
	}
	if effects, ok := KnownIOFunctions[key]; ok {
		return effects
	}
	// Unknown function - return uncertain
	return types.SideEffectUncertain
}

// CheckFunctionPurity returns purity status and confidence for a function
func CheckFunctionPurity(language, qualifiedName string) (isPure bool, confidence types.PurityConfidence) {
	key := language + ":" + qualifiedName

	// Check known pure
	if KnownPureFunctions[key] {
		return true, types.ConfidenceProven
	}

	// Check known impure
	if effects, ok := KnownIOFunctions[key]; ok {
		if effects == types.SideEffectNone {
			return true, types.ConfidenceProven
		}
		return false, types.ConfidenceProven
	}

	// Unknown - return conservative result
	return false, types.ConfidenceNone
}
