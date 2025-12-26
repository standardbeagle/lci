package git

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"
	"strings"
)

// PatternDetector identifies conflict-prone code patterns
type PatternDetector struct {
	// Configurable thresholds
	RegistrationCallsThreshold int
	EnumValuesThreshold        int
	GodObjectLinesThreshold    int
	SwitchCasesThreshold       int
	ConfigFieldsThreshold      int
	BarrelExportRatioThreshold float64
}

// NewPatternDetector creates a pattern detector with default thresholds
func NewPatternDetector() *PatternDetector {
	return &PatternDetector{
		RegistrationCallsThreshold: 10,
		EnumValuesThreshold:        10,
		GodObjectLinesThreshold:    1500,
		SwitchCasesThreshold:       10,
		ConfigFieldsThreshold:      10,
		BarrelExportRatioThreshold: 0.5,
	}
}

// DetectPatterns analyzes file content for conflict-prone patterns
func (d *PatternDetector) DetectPatterns(content []byte, filePath string) []AntiPattern {
	var patterns []AntiPattern

	// Registration function detection
	if p := d.detectRegistrationFunction(content, filePath); p != nil {
		patterns = append(patterns, *p)
	}

	// Enum/const aggregation detection
	if p := d.detectEnumAggregation(content, filePath); p != nil {
		patterns = append(patterns, *p)
	}

	// God object detection (large file)
	if p := d.detectGodObject(content, filePath); p != nil {
		patterns = append(patterns, *p)
	}

	// Switch factory detection
	if ps := d.detectSwitchFactories(content, filePath); len(ps) > 0 {
		patterns = append(patterns, ps...)
	}

	// Barrel file detection
	if p := d.detectBarrelFile(content, filePath); p != nil {
		patterns = append(patterns, *p)
	}

	// Config aggregation detection
	if p := d.detectConfigAggregation(content, filePath); p != nil {
		patterns = append(patterns, *p)
	}

	return patterns
}

// detectRegistrationFunction finds large functions with many sequential registrations
func (d *PatternDetector) detectRegistrationFunction(content []byte, filePath string) *AntiPattern {
	// Look for common registration patterns
	registrationPatterns := []string{
		`\.AddTool\(`,
		`\.Register\(`,
		`\.RegisterHandler\(`,
		`\.AddRoute\(`,
		`\.Handle\(`,
		`\.HandleFunc\(`,
		`\.Post\(`,
		`\.Get\(`,
		`\.Put\(`,
		`\.Delete\(`,
		`router\.`,
		`mux\.`,
		`app\.Use\(`,
		`container\.Bind\(`,
		`container\.Register`,
	}

	// Count occurrences
	totalMatches := 0
	matchedPattern := ""
	for _, pattern := range registrationPatterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindAllIndex(content, -1)
		if len(matches) > totalMatches {
			totalMatches = len(matches)
			matchedPattern = pattern
		}
	}

	if totalMatches >= d.RegistrationCallsThreshold {
		// Find the function containing these registrations
		funcName, lineRange := findContainingFunction(content, matchedPattern)

		return &AntiPattern{
			Type:        PatternRegistrationFunction,
			Description: fmt.Sprintf("Large registration function with %d sequential registrations", totalMatches),
			Location:    lineRange,
			Severity:    determineSeverity(totalMatches, d.RegistrationCallsThreshold),
			Suggestion:  "Consider self-registering pattern using init() functions or a plugin architecture. Function: " + funcName,
			Metrics: map[string]int{
				"registration_calls": totalMatches,
			},
		}
	}

	return nil
}

// detectEnumAggregation finds files with many const/enum definitions
func (d *PatternDetector) detectEnumAggregation(content []byte, filePath string) *AntiPattern {
	// Count const blocks and individual consts
	constBlockPattern := regexp.MustCompile(`const\s*\(`)
	constSinglePattern := regexp.MustCompile(`(?m)^const\s+\w+`)
	iotaPattern := regexp.MustCompile(`\biota\b`)
	enumMemberPattern := regexp.MustCompile(`(?m)^\s*\w+\s*=\s*(iota|\d+|"[^"]*")`)

	constBlocks := len(constBlockPattern.FindAllIndex(content, -1))
	constSingles := len(constSinglePattern.FindAllIndex(content, -1))
	iotaCount := len(iotaPattern.FindAllIndex(content, -1))
	enumMembers := len(enumMemberPattern.FindAllIndex(content, -1))

	// For TypeScript/JavaScript
	enumPattern := regexp.MustCompile(`enum\s+\w+\s*\{`)
	exportConstPattern := regexp.MustCompile(`export\s+const\s+\w+`)

	enumDecls := len(enumPattern.FindAllIndex(content, -1))
	exportConsts := len(exportConstPattern.FindAllIndex(content, -1))

	totalEnumLike := constBlocks + constSingles + iotaCount + enumDecls + exportConsts + enumMembers

	if totalEnumLike >= d.EnumValuesThreshold {
		return &AntiPattern{
			Type:        PatternEnumAggregation,
			Description: fmt.Sprintf("File contains %d enum/const definitions", totalEnumLike),
			Location:    "file-wide",
			Severity:    determineSeverity(totalEnumLike, d.EnumValuesThreshold),
			Suggestion:  "Consider splitting constants by domain/feature, or using code generation (go:generate stringer)",
			Metrics: map[string]int{
				"const_definitions": totalEnumLike,
				"enum_declarations": enumDecls,
				"iota_usages":       iotaCount,
			},
		}
	}

	return nil
}

// detectGodObject finds large files that could be split
func (d *PatternDetector) detectGodObject(content []byte, filePath string) *AntiPattern {
	lineCount := bytes.Count(content, []byte("\n")) + 1

	if lineCount >= d.GodObjectLinesThreshold {
		// Count functions/methods to understand complexity
		funcPattern := regexp.MustCompile(`(?m)^func\s+`)
		methodPattern := regexp.MustCompile(`(?m)^func\s+\([^)]+\)\s+`)
		classPattern := regexp.MustCompile(`(?m)^(class|struct|interface|type)\s+\w+`)

		funcCount := len(funcPattern.FindAllIndex(content, -1))
		methodCount := len(methodPattern.FindAllIndex(content, -1))
		typeCount := len(classPattern.FindAllIndex(content, -1))

		return &AntiPattern{
			Type:        PatternGodObject,
			Description: fmt.Sprintf("Large file with %d lines, %d functions/methods", lineCount, funcCount),
			Location:    "entire file",
			Severity:    AntiPatternSeverityHigh,
			Suggestion:  "Consider splitting into smaller, focused modules by responsibility",
			Metrics: map[string]int{
				"line_count":     lineCount,
				"function_count": funcCount,
				"method_count":   methodCount,
				"type_count":     typeCount,
			},
		}
	}

	return nil
}

// detectSwitchFactories finds large switch/case statements
func (d *PatternDetector) detectSwitchFactories(content []byte, filePath string) []AntiPattern {
	var patterns []AntiPattern

	// Find switch statements and their sizes
	switchPattern := regexp.MustCompile(`(?m)switch\s+[^{]*\{`)
	switchMatches := switchPattern.FindAllIndex(content, -1)

	for _, match := range switchMatches {
		// Find the matching closing brace
		startIdx := match[1]
		caseCount := countCasesInSwitch(content, startIdx)

		if caseCount >= d.SwitchCasesThreshold {
			lineNum := bytes.Count(content[:match[0]], []byte("\n")) + 1

			patterns = append(patterns, AntiPattern{
				Type:        PatternSwitchFactory,
				Description: fmt.Sprintf("Large switch statement with %d cases", caseCount),
				Location:    fmt.Sprintf("line %d", lineNum),
				Severity:    determineSeverity(caseCount, d.SwitchCasesThreshold),
				Suggestion:  "Consider using a map-based dispatch or strategy pattern",
				Metrics: map[string]int{
					"case_count": caseCount,
				},
			})
		}
	}

	// Also check for select statements (Go)
	selectPattern := regexp.MustCompile(`(?m)select\s*\{`)
	selectMatches := selectPattern.FindAllIndex(content, -1)

	for _, match := range selectMatches {
		startIdx := match[1]
		caseCount := countCasesInSwitch(content, startIdx)

		if caseCount >= d.SwitchCasesThreshold {
			lineNum := bytes.Count(content[:match[0]], []byte("\n")) + 1

			patterns = append(patterns, AntiPattern{
				Type:        PatternSwitchFactory,
				Description: fmt.Sprintf("Large select statement with %d cases", caseCount),
				Location:    fmt.Sprintf("line %d", lineNum),
				Severity:    determineSeverity(caseCount, d.SwitchCasesThreshold),
				Suggestion:  "Consider restructuring to reduce case complexity",
				Metrics: map[string]int{
					"case_count": caseCount,
				},
			})
		}
	}

	return patterns
}

// countCasesInSwitch counts case statements within a switch block
func countCasesInSwitch(content []byte, startIdx int) int {
	if startIdx >= len(content) {
		return 0
	}

	// Find the matching closing brace
	depth := 1
	caseCount := 0
	i := startIdx

	for i < len(content) && depth > 0 {
		if content[i] == '{' {
			depth++
		} else if content[i] == '}' {
			depth--
		} else if i+4 < len(content) && string(content[i:i+4]) == "case" {
			// Check if it's a case keyword (not part of another word)
			if (i == 0 || !isAlphaNum(content[i-1])) && (i+4 >= len(content) || !isAlphaNum(content[i+4])) {
				caseCount++
			}
		} else if i+7 < len(content) && string(content[i:i+7]) == "default" {
			if (i == 0 || !isAlphaNum(content[i-1])) && (i+7 >= len(content) || !isAlphaNum(content[i+7])) {
				caseCount++
			}
		}
		i++
	}

	return caseCount
}

// isAlphaNum checks if a byte is alphanumeric
func isAlphaNum(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9') || b == '_'
}

// detectBarrelFile identifies index/re-export files
func (d *PatternDetector) detectBarrelFile(content []byte, filePath string) *AntiPattern {
	// Check filename patterns
	isBarrelName := strings.HasSuffix(filePath, "index.ts") ||
		strings.HasSuffix(filePath, "index.js") ||
		strings.HasSuffix(filePath, "__init__.py") ||
		strings.HasSuffix(filePath, "mod.rs") ||
		strings.HasSuffix(filePath, "exports.ts") ||
		strings.HasSuffix(filePath, "exports.js")

	if !isBarrelName {
		return nil
	}

	// Count export/import statements
	exportPattern := regexp.MustCompile(`(?m)^export\s+`)
	reexportPattern := regexp.MustCompile(`(?m)^export\s+\*?\s*\{?[^}]*\}?\s*from\s+`)
	importPattern := regexp.MustCompile(`(?m)^import\s+`)
	pyExportPattern := regexp.MustCompile(`(?m)^from\s+\.\w+\s+import\s+`)

	exports := len(exportPattern.FindAllIndex(content, -1))
	reexports := len(reexportPattern.FindAllIndex(content, -1))
	imports := len(importPattern.FindAllIndex(content, -1))
	pyExports := len(pyExportPattern.FindAllIndex(content, -1))

	totalExportLike := exports + reexports + pyExports
	totalLines := bytes.Count(content, []byte("\n")) + 1

	if totalLines > 0 {
		exportRatio := float64(totalExportLike) / float64(totalLines)

		if exportRatio >= d.BarrelExportRatioThreshold || totalExportLike >= 10 {
			return &AntiPattern{
				Type:        PatternBarrelFile,
				Description: fmt.Sprintf("Barrel/index file with %d exports (%.0f%% of lines)", totalExportLike, exportRatio*100),
				Location:    "entire file",
				Severity:    determineSeverity(totalExportLike, 10),
				Suggestion:  "Consider direct imports instead of barrel files, or split by feature domain",
				Metrics: map[string]int{
					"export_statements": totalExportLike,
					"import_statements": imports,
					"total_lines":       totalLines,
				},
			}
		}
	}

	return nil
}

// detectConfigAggregation finds large config structs
func (d *PatternDetector) detectConfigAggregation(content []byte, filePath string) *AntiPattern {
	// Check if file is config-related
	isConfigFile := strings.Contains(strings.ToLower(filePath), "config") ||
		strings.Contains(strings.ToLower(filePath), "settings") ||
		strings.Contains(strings.ToLower(filePath), "options")

	if !isConfigFile {
		return nil
	}

	// Count struct fields
	fieldPattern := regexp.MustCompile(`(?m)^\s+\w+\s+\w+.*\x60json:`)
	fields := len(fieldPattern.FindAllIndex(content, -1))

	// Alternative: count any struct fields
	if fields == 0 {
		genericFieldPattern := regexp.MustCompile(`(?m)^\s+\w+\s+\*?\w+[\[\]]*\s*$`)
		fields = len(genericFieldPattern.FindAllIndex(content, -1))
	}

	if fields >= d.ConfigFieldsThreshold {
		return &AntiPattern{
			Type:        PatternConfigAggregation,
			Description: fmt.Sprintf("Large config file with %d fields", fields),
			Location:    "file-wide",
			Severity:    determineSeverity(fields, d.ConfigFieldsThreshold),
			Suggestion:  "Consider splitting config by module/feature with nested structs",
			Metrics: map[string]int{
				"field_count": fields,
			},
		}
	}

	return nil
}

// findContainingFunction attempts to identify the function containing a pattern
func findContainingFunction(content []byte, pattern string) (funcName, lineRange string) {
	re := regexp.MustCompile(pattern)
	matches := re.FindAllIndex(content, 1)
	if len(matches) == 0 {
		return "", ""
	}

	firstMatch := matches[0][0]

	// Search backward for function declaration
	funcPattern := regexp.MustCompile(`(?m)^func\s+(\w+)\s*\(`)
	funcMatches := funcPattern.FindAllSubmatchIndex(content[:firstMatch], -1)

	if len(funcMatches) > 0 {
		lastFunc := funcMatches[len(funcMatches)-1]
		funcName = string(content[lastFunc[2]:lastFunc[3]])
		startLine := bytes.Count(content[:lastFunc[0]], []byte("\n")) + 1

		// Estimate function end
		endLine := startLine + 100 // Rough estimate
		lineRange = fmt.Sprintf("lines %d-%d", startLine, endLine)
	}

	return funcName, lineRange
}

// determineSeverity calculates severity based on how much threshold is exceeded
func determineSeverity(count, threshold int) AntiPatternSeverity {
	ratio := float64(count) / float64(threshold)
	switch {
	case ratio >= 2.0:
		return AntiPatternSeverityHigh
	case ratio >= 1.5:
		return AntiPatternSeverityMedium
	default:
		return AntiPatternSeverityLow
	}
}

// AnalyzeFileForPatterns is a convenience method to analyze a file
func (d *PatternDetector) AnalyzeFileForPatterns(content []byte, filePath string, contributors []ContributorActivity) []AntiPattern {
	patterns := d.DetectPatterns(content, filePath)

	// Boost severity if file has many contributors
	if len(contributors) >= 3 {
		for i := range patterns {
			if patterns[i].Severity == AntiPatternSeverityMedium {
				patterns[i].Severity = AntiPatternSeverityHigh
				patterns[i].Description += fmt.Sprintf(" (high collision risk: %d contributors)", len(contributors))
			}
		}
	}

	return patterns
}

// DetectPatternsFromReader analyzes content from a reader
func (d *PatternDetector) DetectPatternsFromReader(content []byte, filePath string) []AntiPattern {
	return d.DetectPatterns(content, filePath)
}

// GetPatternRecommendations returns detailed recommendations for a pattern type
func GetPatternRecommendations(patternType AntiPatternType) []string {
	switch patternType {
	case PatternRegistrationFunction:
		return []string{
			"Use init() functions for self-registration in each module",
			"Implement a plugin architecture with auto-discovery",
			"Use code generation to build registration code",
			"Split registrations by feature domain into separate functions",
		}
	case PatternEnumAggregation:
		return []string{
			"Group related constants into separate files by domain",
			"Use go:generate stringer for type-safe enum handling",
			"Consider using typed constants with string methods",
			"Move constants closer to where they are used",
		}
	case PatternGodObject:
		return []string{
			"Extract cohesive functionality into separate packages",
			"Apply Single Responsibility Principle",
			"Use interfaces to define boundaries",
			"Consider domain-driven design for module organization",
		}
	case PatternBarrelFile:
		return []string{
			"Use direct imports instead of barrel re-exports",
			"If barrels are needed, split by feature domain",
			"Consider tree-shaking implications for bundles",
			"Document explicit public API in a README instead",
		}
	case PatternSwitchFactory:
		return []string{
			"Replace switch with map-based dispatch",
			"Use strategy pattern with registered handlers",
			"Consider polymorphism with interfaces",
			"Extract each case into a separate handler function",
		}
	case PatternConfigAggregation:
		return []string{
			"Split config by subsystem with nested structs",
			"Use separate config files per module",
			"Consider environment-based config loading",
			"Use config composition instead of single struct",
		}
	default:
		return []string{"Review the code structure for potential improvements"}
	}
}

// QuickScan performs a fast scan looking only for the most impactful patterns
func (d *PatternDetector) QuickScan(content []byte, filePath string) *AntiPattern {
	// Check file size first (fastest check)
	lineCount := bytes.Count(content, []byte("\n")) + 1
	if lineCount >= d.GodObjectLinesThreshold {
		return &AntiPattern{
			Type:        PatternGodObject,
			Description: fmt.Sprintf("Large file with %d lines", lineCount),
			Location:    "entire file",
			Severity:    AntiPatternSeverityHigh,
			Suggestion:  "Consider splitting into smaller, focused modules",
		}
	}

	return nil
}

// CountFileLines returns the line count of content
func CountFileLines(content []byte) int {
	scanner := bufio.NewScanner(bytes.NewReader(content))
	count := 0
	for scanner.Scan() {
		count++
	}
	return count
}
