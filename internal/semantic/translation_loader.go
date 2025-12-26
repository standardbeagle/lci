package semantic

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// TranslationDictionary holds semantic translation mappings for code search
// Maps terms to their semantic synonyms and abbreviation expansions
type TranslationDictionary struct {
	Abbreviations     map[string][]string            // auth → [authenticate, authorization]
	Domains           map[string][]string            // authentication → [signin, login, authenticate]
	Languages         map[string]map[string][]string // go.interface → [contract, ...]
	FuzzyConfig       FuzzyConfig
	StemmingConfig    StemmingConfig
	PropagationConfig PropagationConfig
	TagMappings       map[string][]string // critical → [high-priority, ...]

	// Performance optimization: pre-built reverse indexes (built at init time)
	reverseAbbrev map[string][]string // full form → abbreviations (e.g., "authenticate" → ["auth"])
	termToDomain  map[string][]string // term → all domain terms (e.g., "login" → ["signin", "authenticate", ...])
}

// FuzzyConfig holds fuzzy matching configuration
type FuzzyConfig struct {
	Enabled   bool
	Threshold float64
	Algorithm string // jaro-winkler, levenshtein, cosine
}

// StemmingConfig holds stemming configuration
type StemmingConfig struct {
	Enabled    bool
	Algorithm  string
	MinLength  int
	Exclusions map[string]bool
}

// PropagationConfig holds graph propagation configuration
type PropagationConfig struct {
	Enabled       bool
	FollowAliases bool
	MaxHops       int
	Decay         float64
}

// LoadTranslationDictionary loads from .lci-semantics.kdl or returns defaults
func LoadTranslationDictionary(projectRoot string) (*TranslationDictionary, error) {
	configPath := filepath.Join(projectRoot, ".lci-semantics.kdl")

	// Check if file exists, if not use defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return DefaultTranslationDictionary(), nil
	}

	// For now, return defaults (KDL parsing can be added later)
	// TODO: Implement KDL config parsing
	return DefaultTranslationDictionary(), nil
}

// variableAbbreviations is the set of abbreviations used for variables/fields and common tech terms
// Package-level to avoid allocation - these are common code abbreviations
var variableAbbreviations = map[string]bool{
	"idx": true, "cnt": true, "len": true, "str": true, "num": true, "val": true,
	"cfg": true, "env": true, "ctx": true, "txn": true, "conn": true,
	"mgr": true, "svc": true, "req": true, "resp": true, "res": true,
	"err": true, "msg": true, "obj": true, "arr": true, "ptr": true, "ref": true,
	"param": true, "arg": true, "func": true, "var": true, "const": true,
	"impl": true, "prod": true, "dev": true, "temp": true, "async": true,
	"util": true, "app": true, "i": true, "j": true, "k": true, "v": true, "n": true,
	// Common technology abbreviations that are also used as variable names
	"db": true, "auth": true, "api": true, "http": true, "url": true, "uri": true,
}

// Singleton cache for default translation dictionary
// Avoids expensive buildReverseIndexes() on every call (~144MB allocation)
var (
	defaultDictCache     *TranslationDictionary
	defaultDictCacheOnce sync.Once
)

// DefaultTranslationDictionary returns built-in defaults with pre-built indexes
// Uses a cached singleton to avoid repeated expensive index building
func DefaultTranslationDictionary() *TranslationDictionary {
	defaultDictCacheOnce.Do(func() {
		defaultDictCache = buildDefaultTranslationDictionary()
	})
	return defaultDictCache
}

// buildDefaultTranslationDictionary creates the dictionary (called once via sync.Once)
func buildDefaultTranslationDictionary() *TranslationDictionary {
	td := &TranslationDictionary{
		Abbreviations: defaultAbbreviations,
		Domains:       defaultDomains,
		Languages:     defaultLanguages,
		FuzzyConfig: FuzzyConfig{
			Enabled:   true,
			Threshold: 0.80,
			Algorithm: "jaro-winkler",
		},
		StemmingConfig: StemmingConfig{
			Enabled:   true,
			Algorithm: "porter2",
			MinLength: 3,
			Exclusions: map[string]bool{
				"api": true, "db": true, "uri": true, "url": true,
				"dao": true, "dto": true, "orm": true, "id": true,
				"io": true, "ok": true,
			},
		},
		PropagationConfig: PropagationConfig{
			Enabled:       true,
			FollowAliases: true,
			MaxHops:       3,
			Decay:         0.8,
		},
		TagMappings: defaultTagMappings,
	}

	// Build performance-critical reverse indexes
	td.buildReverseIndexes()

	return td
}

// Expand returns all expansions for a term
func (td *TranslationDictionary) Expand(term string) []string {
	expansions := []string{term} // Always include original

	// Check abbreviations (forward: auth → authenticate, authorization)
	if abbrevs, ok := td.Abbreviations[term]; ok {
		expansions = append(expansions, abbrevs...)
	}

	// Check reverse abbreviations (reverse: authenticate → auth)
	// ONLY for variable/field abbreviations, NOT technology abbreviations
	// Prevent noise: "user" should NOT match "udp" (user datagram protocol)
	// Variable abbreviations are single-word and typically 3-4 characters
	// Check reverse abbreviations using pre-built index (O(1) lookup)
	if td.reverseAbbrev != nil {
		if abbrevs, ok := td.reverseAbbrev[term]; ok {
			expansions = append(expansions, abbrevs...)
		}
	}

	// Check domains using pre-built index (O(1) lookup)
	if td.termToDomain != nil {
		if domainTerms, ok := td.termToDomain[term]; ok {
			expansions = append(expansions, domainTerms...)
		}
	}

	return removeDuplicates(expansions)
}

// buildReverseIndexes builds O(1) lookup indexes for reverse abbreviations and domains
// This eliminates O(n*m) nested loops in Expand(), trading one-time setup cost for massive runtime speedup
func (td *TranslationDictionary) buildReverseIndexes() {
	// Build reverse abbreviation index: fullForm → [abbreviations]
	// Only for variable abbreviations, not technology terms
	td.reverseAbbrev = make(map[string][]string)
	for abbrev, fullForms := range td.Abbreviations {
		// Only index variable abbreviations to prevent noise
		if !variableAbbreviations[abbrev] {
			continue
		}
		for _, fullForm := range fullForms {
			// Only single-word forms
			if !strings.Contains(fullForm, " ") {
				td.reverseAbbrev[fullForm] = append(td.reverseAbbrev[fullForm], abbrev)
			}
		}
	}

	// Build domain term index: term → [all other terms in same domain]
	// This eliminates O(n*m*k) triple nested loop in Expand()
	td.termToDomain = make(map[string][]string)
	for _, domainTerms := range td.Domains {
		// For each term in the domain, create index entry with all OTHER terms
		for i, term := range domainTerms {
			others := make([]string, 0, len(domainTerms)-1)
			for j, other := range domainTerms {
				if i != j {
					others = append(others, other)
				}
			}
			td.termToDomain[term] = others
		}
	}
}

// ExpandLanguageSpecific returns language-specific synonyms
func (td *TranslationDictionary) ExpandLanguageSpecific(lang, term string) []string {
	expansions := []string{term}

	if langMap, ok := td.Languages[lang]; ok {
		if synonyms, ok := langMap[term]; ok {
			expansions = append(expansions, synonyms...)
		}
	}

	return removeDuplicates(expansions)
}

// removeDuplicates removes duplicate strings while preserving order
// Pool for reusing deduplication maps (performance optimization)
var dedupMapPool = sync.Pool{
	New: func() interface{} {
		return make(map[string]bool, 32) // Pre-allocate reasonable size
	},
}

func removeDuplicates(items []string) []string {
	// Reuse map from pool instead of allocating every time
	seen := dedupMapPool.Get().(map[string]bool)
	defer func() {
		// Clear and return to pool
		for k := range seen {
			delete(seen, k)
		}
		dedupMapPool.Put(seen)
	}()

	unique := make([]string, 0, len(items)) // Pre-allocate with exact capacity
	for _, item := range items {
		if !seen[item] {
			unique = append(unique, item)
			seen[item] = true
		}
	}
	return unique
}

// Default abbreviation mappings
var defaultAbbreviations = map[string][]string{
	// Authentication/Authorization
	"auth":  {"authenticate", "authorization", "authorized", "login", "signin"},
	"auth0": {"authentication", "authorization"},
	"oauth": {"oauth", "authorization", "framework"},
	"jwt":   {"json", "web", "token"},
	"sso":   {"single", "sign", "on"},
	"mfa":   {"multi", "factor", "authentication"},
	"ldap":  {"lightweight", "directory", "access", "protocol"},

	// API/Interface
	"api":     {"application", "programming", "interface"},
	"rest":    {"representational", "state", "transfer"},
	"graphql": {"graphql", "query", "language"},
	"grpc":    {"grpc", "remote", "procedure", "call"},
	"rpc":     {"remote", "procedure", "call"},
	"webhook": {"webhook", "callback"},

	// Data/Persistence
	"db":      {"database", "databases"},
	"sql":     {"structured", "query", "language"},
	"orm":     {"object", "relational", "mapping"},
	"dao":     {"data", "access", "object"},
	"dto":     {"data", "transfer", "object"},
	"repo":    {"repository", "repositories"},
	"cache":   {"caching", "cached"},
	"redis":   {"redis", "cache"},
	"nosql":   {"nosql", "database"},
	"mongodb": {"mongodb", "document", "database"},

	// Files/IO
	"io":   {"input", "output"},
	"fs":   {"filesystem", "file", "system"},
	"path": {"filepath", "pathname"},
	"dir":  {"directory"},
	"tmp":  {"temporary"},

	// Network
	"http":  {"hypertext", "transfer", "protocol"},
	"https": {"hypertext", "transfer", "protocol", "secure"},
	"tcp":   {"transmission", "control", "protocol"},
	"udp":   {"user", "datagram", "protocol"},
	"dns":   {"domain", "name", "system"},
	"tls":   {"transport", "layer", "security"},
	"ssl":   {"secure", "sockets", "layer"},
	"cert":  {"certificate"},
	"pem":   {"privacy", "enhanced", "mail"},

	// Web/Frontend
	"ui":   {"user", "interface"},
	"ux":   {"user", "experience"},
	"html": {"hypertext", "markup", "language"},
	"css":  {"cascading", "style", "sheets"},
	"dom":  {"document", "object", "model"},
	"spa":  {"single", "page", "application"},
	"ssr":  {"server", "side", "rendering"},
	"csr":  {"client", "side", "rendering"},
	"pwa":  {"progressive", "web", "application"},

	// DevOps/CI-CD
	"ci":         {"continuous", "integration"},
	"cd":         {"continuous", "deployment"},
	"devops":     {"devops", "development", "operations"},
	"kubernetes": {"kubernetes", "container", "orchestration"},
	"k8s":        {"kubernetes"},
	"docker":     {"docker", "container"},
	"container":  {"container", "containerization"},
	"vm":         {"virtual", "machine"},
	"vcs":        {"version", "control", "system"},
	"git":        {"git", "version", "control"},
	"svn":        {"subversion", "version", "control"},

	// Code Quality
	"test":     {"testing", "unit", "test"},
	"tst":      {"test", "testing"},
	"unit":     {"unit", "test"},
	"e2e":      {"end", "to", "end"},
	"bdd":      {"behavior", "driven", "development"},
	"tdd":      {"test", "driven", "development"},
	"crud":     {"create", "read", "update", "delete"},
	"lint":     {"linting", "code", "quality"},
	"coverage": {"code", "coverage", "test"},

	// Identifiers
	"id":   {"identifier", "identity"},
	"uuid": {"universally", "unique", "identifier"},
	"guid": {"globally", "unique", "identifier"},
	"oid":  {"object", "identifier"},

	// Other Common
	"cli":        {"command", "line", "interface"},
	"gui":        {"graphical", "user", "interface"},
	"url":        {"uniform", "resource", "locator"},
	"uri":        {"uniform", "resource", "identifier"},
	"mime":       {"multipurpose", "internet", "mail", "extensions"},
	"json":       {"javascript", "object", "notation"},
	"xml":        {"extensible", "markup", "language"},
	"yaml":       {"yaml", "data", "serialization"},
	"csv":        {"comma", "separated", "values"},
	"regex":      {"regular", "expression"},
	"utf":        {"unicode", "transformation", "format"},
	"ascii":      {"american", "standard", "code", "information", "interchange"},
	"base64":     {"base64", "encoding"},
	"hash":       {"hashing", "hash", "function"},
	"md5":        {"md5", "hash"},
	"sha":        {"sha", "hash"},
	"bcrypt":     {"bcrypt", "password", "hashing"},
	"log":        {"logging", "log"},
	"metric":     {"metrics", "metric"},
	"trace":      {"tracing", "distributed", "trace"},
	"profile":    {"profiling", "performance"},
	"benchmark":  {"benchmark", "performance", "test"},
	"timeout":    {"timeout", "time", "limit"},
	"retry":      {"retry", "attempt"},
	"queue":      {"queue", "message", "queue"},
	"stream":     {"stream", "streaming"},
	"event":      {"event", "event", "driven"},
	"listener":   {"listener", "observer"},
	"handler":    {"handler", "callback"},
	"middleware": {"middleware", "interceptor"},
	"plugin":     {"plugin", "extension"},
	"lib":        {"library", "libraries"},
	"pkg":        {"package"},
	"mod":        {"module"},
	"vendor":     {"vendor", "vendoring"},
	"deps":       {"dependencies", "dependency"},
	"lock":       {"lockfile", "dependency", "lock"},

	// Common variable/field abbreviations
	"idx":   {"index"},
	"cnt":   {"count"},
	"len":   {"length"},
	"str":   {"string"},
	"num":   {"number"},
	"val":   {"value"},
	"cfg":   {"config", "configuration"},
	"prod":  {"production"},
	"dist":  {"distribution", "distribute"},
	"temp":  {"temporary"},
	"avg":   {"average"},
	"min":   {"minimum"},
	"max":   {"maximum"},
	"err":   {"error"},
	"msg":   {"message"},
	"req":   {"request"},
	"resp":  {"response"},
	"res":   {"response", "result", "resource"},
	"async": {"asynchronous"},
	"util":  {"utility", "utilities"},
	"app":   {"application"},
	"obj":   {"object"},
	"arr":   {"array"},
	"ptr":   {"pointer"},
	"ref":   {"reference"},
	"txn":   {"transaction"},
	"conn":  {"connection"},
	"mgr":   {"manager"},
	"svc":   {"service"},
	"ctx":   {"context"},
	"param": {"parameter"},
	"arg":   {"argument"},
	"func":  {"function"},
	"var":   {"variable"},
	"const": {"constant"},
	"env":   {"environment"},
	"dev":   {"development"},
	"impl":  {"implementation"},

	// Loop counters and common single-letter variables
	"i": {"index", "iterator"},
	"j": {"index", "iterator"},
	"k": {"index", "iterator", "key"},
	"v": {"value"},
	"n": {"number", "count"},
	"x": {"value", "variable"},
	"y": {"value", "variable"},
	"z": {"value", "variable"},

	// C/C++ naming conventions
	"g": {"global"},
	"s": {"static"},
	"m": {"member"},
}

// Domain-specific semantic mappings
var defaultDomains = map[string][]string{
	"authentication": {"signin", "login", "authenticate", "verify", "authorize", "auth"},
	"authorization":  {"permission", "access", "role", "grant", "deny"},
	"user":           {"user", "account", "member", "customer", "profile", "identity"},
	"persistence":    {"store", "save", "persist", "repository", "database", "cache"},
	// Database cluster: all database-related terms
	"database": {
		// Generic terms
		"db", "database", "sql", "nosql", "datastore", "storage",
		// SQL databases
		"postgres", "postgresql", "mysql", "mariadb", "sqlite", "mssql", "oracle",
		// NoSQL databases
		"mongodb", "mongo", "redis", "cassandra", "couchdb", "dynamodb", "elasticsearch",
		// ORM/Query builders
		"orm", "dao", "repository", "activerecord", "sequelize", "typeorm", "prisma",
		"gorm", "sqlx", "ent", "diesel", "sqlalchemy", "mongoose",
		// SQL concepts
		"query", "table", "schema", "migration", "transaction", "index", "constraint",
		"join", "select", "insert", "update", "delete", "where", "having",
	},
	"creation":       {"factory", "builder", "construct", "create", "instantiate", "new"},
	"error-handling": {"error", "exception", "failure", "fault", "recovery", "catch"},
	"communication":  {"send", "transmit", "emit", "publish", "dispatch", "notify"},
	"validation":     {"validate", "verify", "check", "sanitize", "parse", "sanitizer"},
	"transformation": {"convert", "map", "transform", "serialize", "deserialize", "encode"},
	"retrieval":      {"fetch", "get", "retrieve", "load", "query", "find"},
	"deletion":       {"delete", "remove", "purge", "destroy", "drop"},
	"update":         {"update", "modify", "change", "edit", "patch"},
	"search":         {"search", "find", "query", "lookup", "locate"},
	"filtering":      {"filter", "filter", "select", "where", "condition"},
	"sorting":        {"sort", "order", "rank", "arrange"},
	"grouping":       {"group", "aggregate", "collect", "cluster"},
	"counting":       {"count", "sum", "total", "accumulate"},
	"comparison":     {"compare", "equal", "match", "differ"},
	"conversion":     {"convert", "cast", "coerce", "parse"},
	"formatting":     {"format", "template", "render", "display"},
	"logging":        {"log", "trace", "debug", "info", "warn", "error"},
	"monitoring":     {"monitor", "observe", "track", "measure", "metric"},
	"cleanup":        {"cleanup", "cleanup", "teardown", "finalize", "close"},
	"initialization": {"init", "setup", "prepare", "configure", "initialize"},
}

// Language-specific terminology
var defaultLanguages = map[string]map[string][]string{
	"go": {
		"interface":   {"contract", "specification", "abstraction"},
		"goroutine":   {"concurrent", "parallel", "async", "thread", "lightweight"},
		"channel":     {"queue", "pipe", "stream", "messaging", "communication"},
		"defer":       {"cleanup", "teardown", "finalize", "defer"},
		"panic":       {"crash", "fatal", "error", "exception", "panic"},
		"recover":     {"recover", "recovery", "catch", "exception"},
		"mutex":       {"lock", "synchronization", "mutual", "exclusion"},
		"select":      {"multiplex", "choose", "select", "wait"},
		"package":     {"namespace", "module", "package"},
		"struct":      {"record", "class", "type", "data", "structure"},
		"method":      {"function", "member", "operation", "method"},
		"receiver":    {"receiver", "this", "self", "context"},
		"context":     {"context", "scope", "timeout", "cancellation"},
		"error":       {"error", "exception", "failure", "result"},
		"interface{}": {"any", "object", "polymorphic", "generic"},
	},
	"javascript": {
		"promise":     {"async", "future", "thenable", "asynchronous", "promise"},
		"callback":    {"handler", "listener", "observer", "callback"},
		"closure":     {"scope", "context", "binding", "enclosure"},
		"prototype":   {"inheritance", "delegation", "parent", "prototype"},
		"this":        {"context", "receiver", "self", "scope"},
		"async":       {"asynchronous", "promise", "await", "concurrent"},
		"await":       {"wait", "then", "promise", "synchronous"},
		"event":       {"event", "listener", "handler", "emitter"},
		"module":      {"export", "import", "package", "namespace"},
		"class":       {"constructor", "object", "type", "definition"},
		"inheritance": {"extend", "parent", "super", "prototype"},
		"method":      {"function", "member", "operation", "property"},
		"property":    {"attribute", "field", "getter", "accessor"},
		"static":      {"class", "namespace", "global", "shared"},
		"arrow":       {"function", "lambda", "closure", "anonymous"},
	},
	"python": {
		"decorator":  {"wrapper", "aspect", "interceptor", "middleware"},
		"context":    {"resource", "state", "scope", "context", "manager"},
		"generator":  {"iterator", "lazy", "stream", "sequence", "generator"},
		"property":   {"attribute", "field", "getter", "accessor"},
		"magic":      {"special", "dunder", "metamethod", "operator"},
		"metaclass":  {"type", "class", "factory", "metatype"},
		"descriptor": {"property", "accessor", "protocol", "descriptor"},
		"exception":  {"error", "exception", "failure", "raise"},
		"yield":      {"generator", "iterator", "lazy", "coroutine"},
		"lambda":     {"function", "anonymous", "closure", "expression"},
		"module":     {"import", "package", "namespace", "library"},
		"class":      {"type", "object", "definition", "class"},
		"method":     {"function", "member", "operation", "callable"},
	},
}

// Tag mapping for semantic labels
var defaultTagMappings = map[string][]string{
	"critical":      {"high-priority", "important", "security", "performance", "production"},
	"deprecated":    {"old", "obsolete", "legacy", "remove", "unmaintained"},
	"experimental":  {"beta", "unstable", "new", "prototype", "alpha"},
	"todo":          {"fixme", "hack", "todo", "incomplete", "wip"},
	"security":      {"auth", "encryption", "vulnerability", "permission"},
	"performance":   {"optimized", "fast", "slow", "optimization", "benchmark"},
	"bug":           {"issue", "problem", "defect", "broken"},
	"feature":       {"enhancement", "new", "improvement", "capability"},
	"documentation": {"docs", "comment", "example", "readme"},
	"test":          {"testing", "unit", "integration", "e2e"},
	"public":        {"api", "exported", "external", "interface"},
	"private":       {"internal", "hidden", "unexported", "encapsulated"},
}
