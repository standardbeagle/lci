package parser

import (
	tree_sitter_zig "github.com/tree-sitter-grammars/tree-sitter-zig/bindings/go"
	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_csharp "github.com/tree-sitter/tree-sitter-c-sharp/bindings/go"
	tree_sitter_cpp "github.com/tree-sitter/tree-sitter-cpp/bindings/go"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_java "github.com/tree-sitter/tree-sitter-java/bindings/go"
	tree_sitter_javascript "github.com/tree-sitter/tree-sitter-javascript/bindings/go"
	tree_sitter_php "github.com/tree-sitter/tree-sitter-php/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
	tree_sitter_typescript "github.com/tree-sitter/tree-sitter-typescript/bindings/go"
)

func (p *TreeSitterParser) setupJavaScript() {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_javascript.Language()
	language := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(language)
	if err != nil {
		return
	}

	p.parsers[".js"] = parser
	p.parsers[".jsx"] = parser

	queryStr := `
        (function_declaration name: (identifier) @function.name) @function
        (generator_function_declaration name: (identifier) @function.name) @function
        (variable_declarator
            name: (identifier) @function.name
            value: [(arrow_function) (function_expression) (generator_function)]) @function
        (variable_declarator
            name: (identifier) @variable.name
            value: (_) @variable.value) @variable
        (method_definition name: (property_identifier) @method.name) @method
        (class_declaration name: (identifier) @class.name) @class
        (export_statement declaration: (_) @export)
        (import_statement source: (string) @import.source) @import
    `
	query, _ := tree_sitter.NewQuery(language, queryStr)
	// Check if query was actually created (tree-sitter Go binding bug)
	if query != nil {
		p.queries[".js"] = query
		p.queries[".jsx"] = query
	}
}

func (p *TreeSitterParser) setupTypeScript() {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_typescript.LanguageTypescript()
	language := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(language)
	if err != nil {
		return
	}

	p.parsers[".ts"] = parser
	p.parsers[".tsx"] = parser

	queryStr := `
        (function_declaration name: (identifier) @function.name) @function
        (generator_function_declaration name: (identifier) @function.name) @function
        (method_definition name: (property_identifier) @method.name) @method
        (arrow_function) @function
        (function_expression name: (identifier) @function.name) @function
        (class_declaration name: (type_identifier) @class.name) @class
        (interface_declaration name: (type_identifier) @interface.name) @interface
        (type_alias_declaration name: (type_identifier) @type.name) @type
        (enum_declaration name: (identifier) @enum.name) @enum
        (export_statement declaration: (_) @export)
        (import_statement source: (string) @import.source) @import
    `
	query, _ := tree_sitter.NewQuery(language, queryStr)
	// Check if query was actually created (tree-sitter Go binding bug)
	if query != nil {
		p.queries[".ts"] = query
		p.queries[".tsx"] = query
	}
}

func (p *TreeSitterParser) setupGo() {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_go.Language()
	language := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(language)
	if err != nil {
		return
	}

	p.parsers[".go"] = parser

	queryStr := `
        (function_declaration name: (identifier) @function.name) @function
        (method_declaration
            receiver: (parameter_list) @method.receiver
            name: (field_identifier) @method.name) @method
        (type_declaration
            (type_spec name: (type_identifier) @type.name)) @type
        (func_literal) @function
        (import_spec path: (interpreted_string_literal) @import.path) @import
    `
	query, _ := tree_sitter.NewQuery(language, queryStr)

	// The Tree-sitter Go binding has a bug where it returns a typed nil error
	// We need to check if the query was actually created
	if query != nil {
		p.queries[".go"] = query
	}
}

func (p *TreeSitterParser) setupPython() {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_python.Language()
	language := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(language)
	if err != nil {
		return
	}

	p.parsers[".py"] = parser

	queryStr := `
        (class_definition
            body: (block
                (function_definition name: (identifier) @method.name))) @method
        (function_definition name: (identifier) @function.name) @function
        (class_definition name: (identifier) @class.name) @class
        (import_statement) @import
        (import_from_statement) @import
    `
	query, _ := tree_sitter.NewQuery(language, queryStr)
	// Check if query was actually created (tree-sitter Go binding bug)
	if query != nil {
		p.queries[".py"] = query
	}
}

// Phase 4: New language setup methods

func (p *TreeSitterParser) setupRust() {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_rust.Language()
	language := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(language)
	if err != nil {
		return
	}

	p.parsers[".rs"] = parser

	queryStr := `
        (impl_item
            body: (declaration_list
                (function_item name: (identifier) @method.name))) @method
        (trait_item
            body: (declaration_list
                (function_item name: (identifier) @method.name))) @method
        (function_item name: (identifier) @function.name) @function
        (struct_item name: (type_identifier) @struct.name) @struct
        (enum_item name: (type_identifier) @enum.name) @enum
        (trait_item name: (type_identifier) @interface.name) @interface
        (type_item name: (type_identifier) @type.name) @type
        (use_declaration) @import
        (mod_item name: (identifier) @module.name) @module
    `
	query, _ := tree_sitter.NewQuery(language, queryStr)
	if query != nil {
		p.queries[".rs"] = query
	}
}

func (p *TreeSitterParser) setupCpp() {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_cpp.Language()
	language := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(language)
	if err != nil {
		return
	}

	// C++ uses the same parser for all C/C++ extensions
	p.parsers[".cpp"] = parser
	p.parsers[".cc"] = parser
	p.parsers[".cxx"] = parser
	p.parsers[".c"] = parser
	p.parsers[".h"] = parser
	p.parsers[".hpp"] = parser

	queryStr := `
        (function_definition declarator: (function_declarator declarator: (identifier) @function.name)) @function
        (class_specifier name: (type_identifier) @class.name) @class
        (struct_specifier name: (type_identifier) @struct.name) @struct
        (enum_specifier name: (type_identifier) @enum.name) @enum
        (namespace_definition) @namespace
        (preproc_include) @import
        (using_declaration) @import
    `
	query, _ := tree_sitter.NewQuery(language, queryStr)
	if query != nil {
		p.queries[".cpp"] = query
		p.queries[".cc"] = query
		p.queries[".cxx"] = query
		p.queries[".c"] = query
		p.queries[".h"] = query
		p.queries[".hpp"] = query
	}
}

func (p *TreeSitterParser) setupJava() {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_java.Language()
	language := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(language)
	if err != nil {
		return
	}

	p.parsers[".java"] = parser

	queryStr := `
        (method_declaration name: (identifier) @method.name) @method
        (constructor_declaration name: (identifier) @constructor.name) @constructor
        (class_declaration name: (identifier) @class.name) @class
        (record_declaration name: (identifier) @class.name) @class
        (interface_declaration name: (identifier) @interface.name) @interface
        (enum_declaration name: (identifier) @enum.name) @enum
        (field_declaration declarator: (variable_declarator name: (identifier) @field.name)) @field
        (import_declaration) @import
        (package_declaration) @package
        (annotation_type_declaration name: (identifier) @annotation.name) @annotation
    `
	query, _ := tree_sitter.NewQuery(language, queryStr)
	if query != nil {
		p.queries[".java"] = query
	}
}

func (p *TreeSitterParser) setupCSharp() {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_csharp.Language()
	language := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(language)
	if err != nil {
		return
	}
	p.parsers[".cs"] = parser
	queryStr := `
        (method_declaration name: (identifier) @method.name) @method
        (constructor_declaration name: (identifier) @constructor.name) @constructor
        (class_declaration name: (identifier) @class.name) @class
        (interface_declaration name: (identifier) @interface.name) @interface
        (struct_declaration name: (identifier) @struct.name) @struct
        (record_declaration name: (identifier) @record.name) @record
        (enum_declaration name: (identifier) @enum.name) @enum
        (property_declaration name: (identifier) @property.name) @property
        (field_declaration
            (variable_declaration
                (variable_declarator (identifier) @field.name))) @field
        (using_directive (qualified_name) @using.name) @using
        (using_directive (identifier) @using.name) @using
        (namespace_declaration name: (qualified_name) @namespace.name) @namespace
        (namespace_declaration name: (identifier) @namespace.name) @namespace
        (delegate_declaration name: (identifier) @delegate.name) @delegate
        (event_field_declaration
            (variable_declaration
                (variable_declarator (identifier) @event.name))) @event
    `
	query, _ := tree_sitter.NewQuery(language, queryStr)
	// Tree-sitter Go binding bug: err can be a typed nil which is != nil
	// Check if query is not nil instead of checking error
	if query != nil {
		p.queries[".cs"] = query
	}
}

// setupKotlin removed - no official Go bindings available

func (p *TreeSitterParser) setupCommunityParsers() {
	// Community parsers are registered individually as they become available.
	// Currently supported community parsers:
	// - Zig (via tree_sitter_zig)
	//
	// To add a new community parser:
	// 1. Add the parser's Go bindings as a dependency
	// 2. Create a setup function (e.g., setupRuby, setupSwift)
	// 3. Register it with registerLazyInit in NewTreeSitterParser
	//
	// Note: Many languages lack official Go bindings and require community support
}

func (p *TreeSitterParser) setupZig() {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_zig.Language()
	language := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(language)
	if err != nil {
		return
	}

	p.parsers[".zig"] = parser

	queryStr := `
        (function_declaration (identifier) @function.name) @function
        (variable_declaration
          (identifier) @struct.name
          (struct_declaration) @struct)
        (variable_declaration
          (identifier) @struct.name
          (union_declaration) @struct)
    `

	query, _ := tree_sitter.NewQuery(language, queryStr)
	if query != nil {
		p.queries[".zig"] = query
	}
}

func (p *TreeSitterParser) setupPHP() {
	parser := tree_sitter.NewParser()
	languagePtr := tree_sitter_php.LanguagePHP()
	language := tree_sitter.NewLanguage(languagePtr)
	err := parser.SetLanguage(language)
	if err != nil {
		return
	}

	p.parsers[".php"] = parser
	p.parsers[".phtml"] = parser

	// PHP tree-sitter query for symbol extraction
	// Covers: classes, interfaces, traits, enums, functions, methods, namespaces
	queryStr := `
        (class_declaration name: (name) @class.name) @class
        (interface_declaration name: (name) @interface.name) @interface
        (trait_declaration name: (name) @trait.name) @trait
        (enum_declaration name: (name) @enum.name) @enum
        (function_definition name: (name) @function.name) @function
        (method_declaration name: (name) @method.name) @method
        (namespace_definition name: (namespace_name) @namespace.name) @namespace
        (namespace_use_declaration) @import
        (property_declaration) @property
        (const_declaration) @constant
    `

	query, _ := tree_sitter.NewQuery(language, queryStr)
	if query != nil {
		p.queries[".php"] = query
		p.queries[".phtml"] = query
	}
}
