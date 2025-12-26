package core

import (
	"regexp"
	"strings"
)

// Enhanced fragment extraction for HTML/JSX content
func extractHTMLFragments(pattern string, minLength int) []string {
	fragments := make(map[string]bool)
	
	// Regular expressions for HTML/JSX patterns
	// More comprehensive tag matching
	tagRegex := regexp.MustCompile(`</?(\w+)[^>]*>`)
	attrRegex := regexp.MustCompile(`(\w+(?:-\w+)*)=["'{]([^"'}]*)["'}]`)
	jsxExprRegex := regexp.MustCompile(`\{([^}]+)\}`)
	classNameRegex := regexp.MustCompile(`className=["']([^"']+)["']`)
	dataAttrRegex := regexp.MustCompile(`(data-\w+|aria-\w+)=["']([^"']+)["']`)
	
	// Extract tag names
	for _, match := range tagRegex.FindAllStringSubmatch(pattern, -1) {
		if len(match) > 1 && match[1] != "" {
			tag := match[1]
			// Even single-letter tags like 'p' or 'a' are important in HTML
			if len(tag) >= 1 {
				fragments[tag] = true
			}
		}
	}
	
	// Extract attributes and their values
	for _, match := range attrRegex.FindAllStringSubmatch(pattern, -1) {
		if len(match) > 1 && len(match[1]) >= minLength {
			fragments[match[1]] = true // attribute name
		}
		if len(match) > 2 && len(match[2]) >= minLength {
			// Split attribute values by common separators
			parts := strings.FieldsFunc(match[2], func(r rune) bool {
				return r == '-' || r == '_' || r == ' ' || r == '/'
			})
			for _, part := range parts {
				if len(part) >= minLength {
					fragments[part] = true
				}
			}
		}
	}
	
	// Extract JSX expressions
	for _, match := range jsxExprRegex.FindAllStringSubmatch(pattern, -1) {
		if len(match) > 1 {
			expr := match[1]

			// Handle function calls like console.log, api.call, etc.
			funcCallRegex := regexp.MustCompile(`(\w+(?:\.\w+)*)\s*\(`)
			funcMatches := funcCallRegex.FindAllStringSubmatch(expr, -1)
			for _, funcMatch := range funcMatches {
				if len(funcMatch) > 1 {
					funcName := funcMatch[1]
					if len(funcName) >= minLength {
						fragments[funcName] = true
					}
					// Also extract individual parts if dotted
					if strings.Contains(funcName, ".") {
						parts := strings.Split(funcName, ".")
						for _, part := range parts {
							part = strings.TrimSpace(part)
							if len(part) >= minLength {
								fragments[part] = true
							}
						}
					}
				}
			}

			// Handle object property access like user.name
			if strings.Contains(expr, ".") {
				parts := strings.Split(expr, ".")
				for _, part := range parts {
					part = strings.TrimSpace(part)
					// Clean up common JavaScript syntax
					part = strings.TrimSuffix(part, "(")
					part = strings.TrimSuffix(part, ")")
					if len(part) >= minLength {
						fragments[part] = true
					}
				}
			}

			// Extract identifiers from expressions
			identifierRegex := regexp.MustCompile(`\b(\w+)\b`)
			identifiers := identifierRegex.FindAllString(expr, -1)
			for _, id := range identifiers {
				if len(id) >= minLength {
					fragments[id] = true
				}
			}

			// Also include the full expression if it's not too long
			if len(expr) >= minLength && len(expr) <= 30 {
				fragments[expr] = true
			}
		}
	}
	
	// Extract className values specifically
	for _, match := range classNameRegex.FindAllStringSubmatch(pattern, -1) {
		if len(match) > 1 {
			// Split by spaces for multiple classes
			classes := strings.Fields(match[1])
			for _, class := range classes {
				if len(class) >= minLength {
					fragments[class] = true
				}
			}
		}
	}
	
	// Extract data and aria attributes
	for _, match := range dataAttrRegex.FindAllStringSubmatch(pattern, -1) {
		if len(match) > 1 && len(match[1]) >= minLength {
			fragments[match[1]] = true
		}
		if len(match) > 2 && len(match[2]) >= minLength {
			// Split compound values
			parts := strings.FieldsFunc(match[2], func(r rune) bool {
				return r == '-' || r == ' '
			})
			for _, part := range parts {
				if len(part) >= minLength {
					fragments[part] = true
				}
			}
		}
	}
	
	// Extract text content between tags (improved to handle JSX expressions)
	// First, extract pure text content
	textRegex := regexp.MustCompile(`>([^<]+)<`)
	for _, match := range textRegex.FindAllStringSubmatch(pattern, -1) {
		if len(match) > 1 {
			text := strings.TrimSpace(match[1])
			// Remove JSX expressions temporarily to get plain text
			textWithoutJSX := regexp.MustCompile(`\{[^}]*\}`).ReplaceAllString(text, " ")
			// Split text content by spaces and punctuation
			words := strings.FieldsFunc(textWithoutJSX, func(r rune) bool {
				return r == ' ' || r == ':' || r == ',' || r == '.' || r == '\t' || r == '\n'
			})
			for _, word := range words {
				word = strings.TrimSpace(word)
				if len(word) >= minLength {
					fragments[word] = true
				}
			}
		}
	}
	
	// Convert map to slice
	result := make([]string, 0, len(fragments))
	for frag := range fragments {
		result = append(result, frag)
	}
	
	return result
}

// enhancedFragmentString combines regular and HTML-specific fragmentation
func (ase *AssemblySearchEngine) enhancedFragmentString(pattern string, minLength int) []string {
	// Check if pattern looks like HTML/JSX
	if strings.Contains(pattern, "<") && strings.Contains(pattern, ">") {
		htmlFragments := extractHTMLFragments(pattern, minLength)
		
		// Also run regular fragmentation for any non-HTML parts
		regularFragments := ase.fragmentString(pattern, minLength)
		
		// Combine and deduplicate
		seen := make(map[string]bool)
		combined := []string{}
		
		for _, frag := range htmlFragments {
			if !seen[frag] {
				seen[frag] = true
				combined = append(combined, frag)
			}
		}
		
		for _, frag := range regularFragments {
			if !seen[frag] {
				seen[frag] = true
				combined = append(combined, frag)
			}
		}
		
		return combined
	}
	
	// Not HTML/JSX, use regular fragmentation
	return ase.fragmentString(pattern, minLength)
}