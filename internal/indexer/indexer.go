package indexer

// =============================================================================
// INDEXER PHILOSOPHY: TRUST THE EXTRACTOR, VALIDATE WITH CUE
// =============================================================================
//
// The indexer sits between extraction and policy evaluation. Its job is to:
// 1. Aggregate facts from multiple files into a unified view
// 2. Build the cross-file symbol table
// 3. Resolve dependencies between files
// 4. Prepare normalized data for Rust policy evaluation
//
// IMPORTANT: The indexer should NOT work around extraction bugs!
//
// If the indexer needs to "fix" or "clean up" extracted data, that's a sign
// that either:
// - The GRAMMAR is missing a construct (fix grammar.js first!)
// - The EXTRACTOR is missing logic (fix extractor.go second!)
//
// The CUE validator (internal/validator) catches schema mismatches between
// what we produce here and what the Rust policy engine expects. If validation fails, it means
// our contract is broken - fix the source, don't suppress the error.
//
// See: AGENTS.md "The Grammar Improvement Cycle"
// =============================================================================

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/facts"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/policy"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/validator"
)

// Indexer is the cross-file linker that builds the symbol table
// and resolves dependencies between VHDL files.
type Indexer struct {
	// Configuration loaded from vhdl_lint.json
	Config *config.Config

	// Library map: logical name -> physical path
	Libraries map[string]string

	// Global symbol table: qualified name -> location
	Symbols *SymbolTable

	// Extracted facts from all files
	Facts []extractor.FileFacts

	// KeepFacts retains extracted facts in memory (tests/debugging only).
	KeepFacts bool

	// Resolved library information (file -> library mapping)
	FileLibraries map[string]config.FileLibraryInfo

	// Third-party files (for suppressing warnings)
	ThirdPartyFiles map[string]bool

	// Verbose output
	Verbose bool

	// Progress output (lightweight, streaming)
	Progress bool

	// Trace output (progress + per-file fact summaries)
	Trace bool

	// JSON output mode
	JSONOutput bool

	// SymbolsJSON includes symbol index in JSON output (for LSP)
	SymbolsJSON bool

	// Timing output (JSONL)
	Timing     bool
	TimingPath string

	// Optional extractor factory (for tests)
	extractorFactory func() FactsExtractor

	// Optional cache version override (for tests)
	cacheVersionOverride *cacheVersions

	// Optional output sink for JSON output (defaults to stdout)
	Output io.Writer
}

// LintResult is the structured result of running the linter
// This can be serialized to JSON for programmatic consumption
type LintResult struct {
	// Violations found by policy evaluation
	Violations []policy.Violation `json:"violations"`

	// Structured missing-check tasks
	MissingChecks []policy.MissingCheckTask `json:"missing_checks,omitempty"`

	// Ambiguous construct warnings (structured)
	AmbiguousConstructs []policy.AmbiguousConstruct `json:"ambiguous_constructs,omitempty"`

	// Structured waiver records
	Waivers []policy.Waiver `json:"waivers,omitempty"`

	// Summary counts
	Summary ResultSummary `json:"summary"`

	// Extraction statistics
	Stats ExtractionStats `json:"stats"`

	// Per-file breakdown
	Files []FileResult `json:"files"`

	// Parse errors encountered
	ParseErrors []ParseError `json:"parse_errors,omitempty"`

	// Symbol index for IDE navigation (populated when SymbolsJSON is set)
	SymbolIndex *SymbolIndex `json:"symbol_index,omitempty"`
}

// ResultSummary provides aggregate violation counts
type ResultSummary struct {
	TotalViolations int `json:"total_violations"`
	Errors          int `json:"errors"`
	Warnings        int `json:"warnings"`
	Info            int `json:"info"`
}

// ExtractionStats provides counts of extracted elements
type ExtractionStats struct {
	Files     int `json:"files"`
	Symbols   int `json:"symbols"`
	Entities  int `json:"entities"`
	Packages  int `json:"packages"`
	Signals   int `json:"signals"`
	Ports     int `json:"ports"`
	Processes int `json:"processes"`
	Instances int `json:"instances"`
	Generates int `json:"generates"`
}

// FileResult provides per-file violation counts
type FileResult struct {
	Path     string `json:"path"`
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
	Info     int    `json:"info"`
}

// ParseError represents a file that failed to parse
type ParseError struct {
	File    string `json:"file"`
	Message string `json:"message"`
}

// SymbolTable holds all exported symbols across files
type SymbolTable struct {
	mu      sync.RWMutex
	symbols map[string]Symbol
}

// Symbol represents an exported VHDL construct
type Symbol struct {
	Name string // Qualified name: work.my_entity
	Kind string // entity, package, component, etc.
	File string // Source file path
	Line int    // Line number
}

// FactsExtractor abstracts extraction for caching tests
type FactsExtractor interface {
	Extract(path string) (extractor.FileFacts, error)
}

type cacheVersions struct {
	parser    string
	extractor string
}

// New creates a new Indexer with default configuration
func New() *Indexer {
	return &Indexer{
		Config: config.DefaultConfig(),
		Libraries: map[string]string{
			"work": ".", // Default: work library is current directory
		},
		Symbols: &SymbolTable{
			symbols: make(map[string]Symbol),
		},
		FileLibraries:   make(map[string]config.FileLibraryInfo),
		ThirdPartyFiles: make(map[string]bool),
	}
}

// NewWithConfig creates a new Indexer with the given configuration
func NewWithConfig(cfg *config.Config) *Indexer {
	idx := New()
	idx.Config = cfg
	return idx
}

func (idx *Indexer) newExtractor() FactsExtractor {
	if idx.extractorFactory != nil {
		return idx.extractorFactory()
	}
	return extractor.New()
}

func (idx *Indexer) cacheVersions(rootPath string) cacheVersions {
	if idx.cacheVersionOverride != nil {
		return *idx.cacheVersionOverride
	}
	return computeCacheVersions(rootPath)
}

func (idx *Indexer) registerSymbolsForFacts(facts extractor.FileFacts, filePath string) {
	// Determine the library name for this file
	// Use actual library from config, or fall back to "work"
	libName := "work"
	if libInfo, ok := idx.FileLibraries[filePath]; ok && libInfo.LibraryName != "" {
		libName = strings.ToLower(libInfo.LibraryName)
	}

	// Register exports in symbol table with proper library prefix
	for _, entity := range facts.Entities {
		idx.Symbols.Add(Symbol{
			Name: fmt.Sprintf("%s.%s", libName, strings.ToLower(entity.Name)),
			Kind: "entity",
			File: filePath,
			Line: entity.Line,
		})
	}
	for _, pkg := range facts.Packages {
		idx.Symbols.Add(Symbol{
			Name: fmt.Sprintf("%s.%s", libName, strings.ToLower(pkg.Name)),
			Kind: "package",
			File: filePath,
			Line: pkg.Line,
		})
	}
	for _, ctx := range facts.ContextDecls {
		if isValidIdentifierName(ctx.Name) {
			idx.Symbols.Add(Symbol{
				Name: fmt.Sprintf("%s.%s", libName, strings.ToLower(ctx.Name)),
				Kind: "context",
				File: filePath,
				Line: ctx.Line,
			})
		}
	}

	// Register package contents in symbol table for cross-file resolution
	// Format: library.package.item (e.g., work.my_pkg.state_t)
	for _, t := range facts.Types {
		if t.InPackage != "" && isValidIdentifierName(t.Name) {
			idx.Symbols.Add(Symbol{
				Name: fmt.Sprintf("%s.%s.%s", libName, strings.ToLower(t.InPackage), strings.ToLower(t.Name)),
				Kind: "type",
				File: filePath,
				Line: t.Line,
			})
		}
	}
	for _, c := range facts.ConstantDecls {
		if c.InPackage != "" && isValidIdentifierName(c.Name) {
			idx.Symbols.Add(Symbol{
				Name: fmt.Sprintf("%s.%s.%s", libName, strings.ToLower(c.InPackage), strings.ToLower(c.Name)),
				Kind: "constant",
				File: filePath,
				Line: c.Line,
			})
		}
	}
	for _, fn := range facts.Functions {
		if fn.InPackage != "" && isValidIdentifierName(fn.Name) {
			idx.Symbols.Add(Symbol{
				Name: fmt.Sprintf("%s.%s.%s", libName, strings.ToLower(fn.InPackage), strings.ToLower(fn.Name)),
				Kind: "function",
				File: filePath,
				Line: fn.Line,
			})
		}
	}
	for _, pr := range facts.Procedures {
		if pr.InPackage != "" && isValidIdentifierName(pr.Name) {
			idx.Symbols.Add(Symbol{
				Name: fmt.Sprintf("%s.%s.%s", libName, strings.ToLower(pr.InPackage), strings.ToLower(pr.Name)),
				Kind: "procedure",
				File: filePath,
				Line: pr.Line,
			})
		}
	}
}

// Run executes the indexing pipeline
func (idx *Indexer) Run(rootPath string) error {
	runStart := time.Now()
	pipelineErrs := make([]error, 0)
	recordPipelineErr := func(err error) {
		pipelineErrs = append(pipelineErrs, err)
	}
	timing := newTimingRecorder(runStart, idx.resolveTimingPath(rootPath))
	if err := timing.Err(); err != nil {
		recordPipelineErr(fmt.Errorf("timing output disabled: %w", err))
	}
	defer timing.Close()

	// 0. Load configuration if not already loaded
	if idx.Config == nil {
		cfg, err := config.Load(rootPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		idx.Config = cfg
	}
	if idx.Config != nil {
		if project, ok := idx.Config.ApplyProjectOverrides(rootPath); ok && !idx.JSONOutput {
			fmt.Printf("Applied project config: %s\n", project)
		}
	}
	if !idx.JSONOutput {
		emitPolicyRuleConfigSummary(idx.Config)
		if idx.Config != nil {
			mode := strings.ToLower(strings.TrimSpace(idx.Config.Analysis.PolicyInputMode))
			if mode != "" && mode != "full" {
				fmt.Printf("Policy input mode: %s (heavy facts omitted)\n", mode)
			}
		}
	}

	// Reset per-run state
	idx.Symbols = &SymbolTable{symbols: make(map[string]Symbol)}
	idx.Facts = nil
	idx.FileLibraries = make(map[string]config.FileLibraryInfo)
	idx.ThirdPartyFiles = make(map[string]bool)

	// 1. Find all VHDL files using configuration
	stepStart := time.Now()
	var files []string
	var err error
	var proMap *proMapping
	fileSet := make(map[string]bool)

	// Check if config has library definitions
	if len(idx.Config.Libraries) > 0 {
		libs, resolveErr := idx.Config.ResolveLibraries(rootPath)
		if resolveErr != nil {
			return fmt.Errorf("resolve libraries: %w", resolveErr)
		}

		// Collect all files and track library info
		for _, lib := range libs {
			for _, f := range lib.Files {
				if !fileSet[f] {
					fileSet[f] = true
					files = append(files, f)

					// Track library info for each file
					idx.FileLibraries[f] = config.FileLibraryInfo{
						LibraryName:  lib.Name,
						IsThirdParty: lib.IsThirdParty,
					}

					// Track third-party files
					if lib.IsThirdParty {
						idx.ThirdPartyFiles[f] = true
					}
				}
			}
		}

		// Report library info (only in text mode)
		if !idx.JSONOutput {
			fmt.Printf("Loaded configuration with %d libraries\n", len(libs))
			for _, lib := range libs {
				thirdParty := ""
				if lib.IsThirdParty {
					thirdParty = " (third-party)"
				}
				fmt.Printf("  %s: %d files%s\n", lib.Name, len(lib.Files), thirdParty)
			}
		}
	}

	if idx.Config != nil && idx.Config.Analysis.UseProLibraries {
		mapping, mapErr := loadProLibraryMapping(rootPath, idx.Config)
		if mapping != nil {
			proMap = mapping
			if !idx.JSONOutput && len(mapping.entrypoints) > 0 {
				fmt.Printf("Using .pro entrypoint(s): %s\n", strings.Join(mapping.entrypoints, ", "))
			}
			stats := applyProLibraryMapping(idx, mapping, fileSet, &files)
			if !idx.JSONOutput && (stats.files > 0 || stats.packages > 0) {
				fmt.Printf("Applied .pro library mapping: %d files, %d libraries, %d generated packages\n",
					stats.files, stats.libraries, stats.packages)
			}
		}
		if mapErr != nil {
			recordPipelineErr(fmt.Errorf("pro library mapping partial: %w", mapErr))
		}
	}

	if proMap != nil && idx.Config != nil && idx.Config.Analysis.UseProLibraries {
		proLibs := make(map[string]bool)
		for _, lib := range proMap.fileLibraries {
			if lib == "" {
				continue
			}
			proLibs[strings.ToLower(lib)] = true
		}
		if len(proLibs) > 0 {
			filtered := make([]string, 0, len(files))
			for _, f := range files {
				libInfo, ok := idx.FileLibraries[f]
				if !ok {
					filtered = append(filtered, f)
					continue
				}
				libName := strings.ToLower(libInfo.LibraryName)
				if proLibs[libName] {
					if _, ok := proMap.fileLibraries[f]; ok {
						filtered = append(filtered, f)
					}
					continue
				}
				filtered = append(filtered, f)
			}
			if !idx.JSONOutput && len(filtered) != len(files) {
				fmt.Printf("Filtered files via .pro mapping: %d -> %d\n", len(files), len(filtered))
			}
			files = filtered
		}
	}

	// Fallback to directory scan if no files from config
	if len(files) == 0 {
		files, err = idx.findVHDLFiles(rootPath)
		if err != nil {
			return fmt.Errorf("scanning files: %w", err)
		}
	}

	// Filter out ignored files
	var filteredFiles []string
	for _, f := range files {
		if !idx.Config.ShouldIgnoreFile(f) {
			filteredFiles = append(filteredFiles, f)
		}
	}
	files = filteredFiles

	if !idx.JSONOutput {
		fmt.Printf("Found %d VHDL files\n", len(files))
	}
	scanDuration := time.Since(stepStart)
	timing.RecordStage("scan", stepStart, scanDuration, "")
	if len(files) == 0 {
		msg := "No VHDL files found. Nothing to lint. " +
			"Action: verify the lint path and your config (libraries/files globs or ignorePatterns) include *.vhd/*.vhdl."
		if idx.JSONOutput {
			fmt.Fprintln(os.Stderr, msg)
		} else {
			fmt.Println(msg)
		}
		return nil
	}

	// 2. Pass 1: Parallel extraction (with optional cache)
	stepStart = time.Now()
	ext := idx.newExtractor()
	var cache *factsCache
	var cacheDir string
	if cacheEnabled(idx.Config) {
		cacheDir = resolveCacheDir(rootPath, idx.Config)
		versions := idx.cacheVersions(rootPath)
		cache = newFactsCache(cacheDir, versions.parser, versions.extractor)
		if err := cache.Load(); err != nil {
			recordPipelineErr(fmt.Errorf("cache disabled: %w", err))
			cache = nil
		}
	}
	store, err := newFactsStore(cacheDir)
	if err != nil {
		return fmt.Errorf("facts store: %w", err)
	}
	defer func() {
		if err := store.Close(); err != nil {
			recordPipelineErr(fmt.Errorf("facts store cleanup failed: %w", err))
		}
	}()
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	progress := 0
	progressEnabled := (idx.Verbose || idx.Progress || idx.Trace) && !idx.JSONOutput
	var factsMu sync.Mutex
	if progressEnabled {
		fmt.Printf("\n=== Extraction Progress ===\n")
	}
	errChan := make(chan error, len(files))
	pipelineErrChan := make(chan error, len(files))
	var changedMu sync.Mutex
	changedFiles := make(map[string]bool)

	for _, file := range files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			fileStart := time.Now()
			var contentHash string
			if cache != nil {
				h, err := hashFile(f)
				if err != nil {
					errChan <- fmt.Errorf("%s: %w", f, err)
					return
				}
				contentHash = h
				if facts, ok, err := cache.Get(f, contentHash); err == nil && ok {
					if err := store.Put(f, facts); err != nil {
						errChan <- fmt.Errorf("%s: store facts: %w", f, err)
						return
					}
					if idx.KeepFacts {
						factsMu.Lock()
						idx.Facts = append(idx.Facts, facts)
						factsMu.Unlock()
					}
					idx.registerSymbolsForFacts(facts, f)
					fileDuration := time.Since(fileStart)
					timing.RecordFile("extract", f, "cache_hit", fileStart, fileDuration)
					if progressEnabled {
						emitProgress(&progressMu, &progress, len(files), facts, "cache hit", idx.Trace, fileDuration)
					}
					return
				} else if err != nil {
					pipelineErrChan <- fmt.Errorf("cache read failed for %s: %w", f, err)
				}
			}

			facts, err := ext.Extract(f)
			if err != nil {
				errChan <- fmt.Errorf("%s: %w", f, err)
				return
			}
			if cache != nil && contentHash != "" {
				if err := cache.Put(f, contentHash, facts); err != nil {
					pipelineErrChan <- fmt.Errorf("cache write failed for %s: %w", f, err)
				}
			}
			if err := store.Put(f, facts); err != nil {
				errChan <- fmt.Errorf("%s: store facts: %w", f, err)
				return
			}
			if idx.KeepFacts {
				factsMu.Lock()
				idx.Facts = append(idx.Facts, facts)
				factsMu.Unlock()
			}
			if cache != nil {
				changedMu.Lock()
				changedFiles[f] = true
				changedMu.Unlock()
			}
			fileDuration := time.Since(fileStart)
			timing.RecordFile("extract", f, "extracted", fileStart, fileDuration)
			if progressEnabled {
				emitProgress(&progressMu, &progress, len(files), facts, "extracted", idx.Trace, fileDuration)
			}
			idx.registerSymbolsForFacts(facts, f)
		}(file)
	}

	wg.Wait()
	close(errChan)
	close(pipelineErrChan)

	// Collect errors
	var errs []error
	for err := range errChan {
		errs = append(errs, err)
	}
	for err := range pipelineErrChan {
		recordPipelineErr(err)
	}

	if added, err := injectGeneratedPackagesStreaming(idx, proMap, store); err != nil {
		return fmt.Errorf("inject generated packages: %w", err)
	} else if added > 0 && !idx.JSONOutput {
		fmt.Printf("Injected %d generated package(s) from .pro scripts\n", added)
	}
	if cache != nil {
		if err := cache.Save(); err != nil {
			recordPipelineErr(fmt.Errorf("cache save failed: %w", err))
		}
	}
	extractDuration := time.Since(stepStart)
	timing.RecordStage("extract", stepStart, extractDuration, "")
	factFiles := store.Files()

	// Cache impact visualization (verbose/progress/trace)
	if cache != nil && progressEnabled && len(changedFiles) > 0 {
		fmt.Printf("\n=== Cache Impact ===\n")
		if len(factFiles) > 2000 || len(changedFiles) > 500 {
			fmt.Printf("  skipped: %d files changed (too large for impact graph)\n", len(changedFiles))
			fmt.Printf("  hint: rerun on a smaller subset or enable only --trace on scoped paths\n")
		} else {
			dependents, err := buildDependentsGraphFromStore(store, factFiles, idx.Symbols, idx.FileLibraries)
			if err != nil {
				recordPipelineErr(fmt.Errorf("cache impact skipped: %w", err))
			} else {
				changedList := make([]string, 0, len(changedFiles))
				for f := range changedFiles {
					changedList = append(changedList, f)
				}
				sort.Strings(changedList)
				for _, f := range changedList {
					report := computeImpact(f, dependents)
					fmt.Print(formatImpactReport(report))
				}
			}
		}
	}

	// Elaborate generate statements using constant values
	stepStart = time.Now()
	// Build a global constant map from all extracted constants
	globalConstants := make(map[string]int)
	for _, file := range factFiles {
		facts, err := store.Get(file)
		if err != nil {
			return fmt.Errorf("load facts for constants: %w", err)
		}
		constMap := extractor.BuildConstantMap(facts.ConstantDecls)
		for k, v := range constMap {
			globalConstants[k] = v
		}
	}

	// Elaborate generates in all files and persist to store
	elaboratedCount := 0
	for _, file := range factFiles {
		facts, err := store.Get(file)
		if err != nil {
			return fmt.Errorf("load facts for elaboration: %w", err)
		}
		elaboratedCount += extractor.ElaborateGenerates(facts.Generates, globalConstants)
		if err := store.Put(file, facts); err != nil {
			return fmt.Errorf("store elaborated facts: %w", err)
		}
	}
	if elaboratedCount > 0 && idx.Verbose {
		fmt.Printf("\n=== Verbose: Generate Elaboration ===\n")
		fmt.Printf("  Elaborated %d for-generates using %d constants\n", elaboratedCount, len(globalConstants))
	}
	elabDuration := time.Since(stepStart)
	timing.RecordStage("elaborate", stepStart, elabDuration, "")
	var factsValidateDuration time.Duration

	// Build policy engine input and fact tables from store
	stepStart = time.Now()
	policyInput, factTables, err := idx.buildPolicyInputAndTables(store, factFiles)
	if err != nil {
		return err
	}
	buildDuration := time.Since(stepStart)
	timing.RecordStage("build_input", stepStart, buildDuration, "")

	// Validate relational fact tables for Datalog ingestion
	stepStart = time.Now()
	factsValidator, err := validator.NewFactsValidator()
	if err != nil {
		return fmt.Errorf("CRITICAL: Failed to initialize facts validator: %w", err)
	}
	if err := factsValidator.Validate(factTables); err != nil {
		return fmt.Errorf("CRITICAL: Fact table contract violation: %w", err)
	}
	factsValidateDuration = time.Since(stepStart)
	timing.RecordStage("facts_validate", stepStart, factsValidateDuration, "")

	// Verbose output for debugging
	if idx.Verbose {
		if err := idx.emitVerboseFromStore(store, factFiles); err != nil {
			return err
		}
	}

	// Require explicit library mapping for any referenced libraries
	if idx.Config != nil && idx.Config.RequiresLibraryMapping() {
		if missing := idx.findMissingLibraries(policyInput.LibraryClauses); len(missing) > 0 {
			configLabel := "vhdl_lint.json"
			if idx.Config != nil && idx.Config.ConfigPath() != "" {
				configLabel = idx.Config.ConfigPath()
			}
			return fmt.Errorf("library mapping required: referenced libraries not linked in config: %s. "+
				"Add them under libraries/files in %s before running the linter.", strings.Join(missing, ", "), configLabel)
		}
	}
	// 3. Pass 2: Resolution (check imports)
	stepStart = time.Now()
	resolveDuration := time.Since(stepStart)
	timing.RecordStage("resolve", stepStart, resolveDuration, "")

	// 5. Validate data structure before policy evaluation (CUE contract enforcement)
	stepStart = time.Now()
	v, err := validator.New()
	if err != nil {
		return fmt.Errorf("CRITICAL: Failed to initialize CUE validator: %w", err)
	}
	if err := validateVerificationTags(v, &policyInput); err != nil {
		return fmt.Errorf("CRITICAL: Failed to validate verification tags: %w", err)
	}
	if err := validateVerificationWaivers(v, &policyInput); err != nil {
		return fmt.Errorf("CRITICAL: Failed to validate verification waivers: %w", err)
	}
	if err := v.Validate(policyInput); err != nil {
		return fmt.Errorf("CRITICAL: Data contract violation (Go -> policy engine mismatch): %w", err)
	}
	validateDuration := time.Since(stepStart)
	timing.RecordStage("validate", stepStart, validateDuration, "")

	// 6. Run policy evaluation and build result
	stepStart = time.Now()
	lintResult := LintResult{
		Violations:  []policy.Violation{},
		ParseErrors: []ParseError{},
		Stats: ExtractionStats{
			Files:     len(files),
			Symbols:   idx.Symbols.Len(),
			Entities:  len(policyInput.Entities),
			Packages:  len(policyInput.Packages),
			Signals:   len(policyInput.Signals),
			Ports:     len(policyInput.Ports),
			Processes: len(policyInput.Processes),
			Instances: len(policyInput.Instances),
			Generates: len(policyInput.Generates),
		},
		Files: []FileResult{},
	}

	// Add parse errors
	for _, e := range errs {
		lintResult.ParseErrors = append(lintResult.ParseErrors, ParseError{
			File:    "",
			Message: e.Error(),
		})
	}

	policyCached := false
	policyUsedDaemon := false
	policyDelta := false

	if envBool("VHDL_POLICY_DAEMON") {
		if cache == nil {
			recordPipelineErr(fmt.Errorf("policy daemon requested but cache disabled"))
		}
		if result, usedDelta, err := runPolicyDaemon(cacheDir, cache != nil, factTables, changedFiles); err != nil {
			recordPipelineErr(fmt.Errorf("policy daemon failed: %w", err))
		} else {
			applyPolicyResult(&lintResult, result)
			policyUsedDaemon = true
			policyDelta = usedDelta
		}
	}

	cacheHash := ""
	if !policyUsedDaemon && cache != nil {
		if hash, err := policyConfigHash(policyInput); err == nil {
			cacheHash = hash
		} else {
			recordPipelineErr(fmt.Errorf("policy cache disabled: %w", err))
		}
	}
	if !policyUsedDaemon && cache != nil && len(changedFiles) == 0 && cacheHash != "" {
		if entry, err := loadPolicyCache(cacheDir); err != nil {
			recordPipelineErr(fmt.Errorf("policy cache load failed: %w", err))
		} else if ok, err := policyCacheValid(entry, policyInput, factFiles); err == nil && ok {
			applyPolicyResult(&lintResult, &entry.Result)
			policyCached = true
		} else if err != nil {
			recordPipelineErr(fmt.Errorf("policy cache disabled: %w", err))
		}
	}

	if !policyCached && !policyUsedDaemon {
		policyEngine, err := policy.New(".")
		if err != nil {
			return fmt.Errorf("initialize policy engine: %w", err)
		}
		result, err := policyEngine.Evaluate(policyInput)
		if err != nil {
			return fmt.Errorf("policy evaluation failed: %w", err)
		}
		applyPolicyResult(&lintResult, result)
		if cache != nil && cacheHash != "" {
			if err := savePolicyCache(cacheDir, policyCacheEntry{
				Version:    policyCacheVersion,
				ConfigHash: cacheHash,
				Files:      factFiles,
				Result:     *result,
			}); err != nil {
				recordPipelineErr(fmt.Errorf("policy cache save failed: %w", err))
			}
		}
	}

	if cache != nil {
		if err := saveFactTablesCache(cacheDir, factTables); err != nil {
			recordPipelineErr(fmt.Errorf("fact tables cache save failed: %w", err))
		}
	}

	// Populate symbol index for LSP if requested
	if idx.SymbolsJSON {
		lintResult.SymbolIndex = buildSymbolIndex(&policyInput)
	}

	// Output results
	if idx.JSONOutput {
		// JSON output mode
		out := idx.Output
		if out == nil {
			out = os.Stdout
		}
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(lintResult); err != nil {
			return fmt.Errorf("failed to encode JSON output: %w", err)
		}
	} else {
		// Text output mode (original behavior)
		if len(lintResult.Violations) > 0 {
			fmt.Printf("\n=== Policy Violations ===\n")
			for _, v := range lintResult.Violations {
				icon := "ℹ"
				if v.Severity == "error" {
					icon = "✗"
				} else if v.Severity == "warning" {
					icon = "⚠"
				}
				fmt.Printf("%s [%s] %s:%d - %s\n", icon, v.Rule, v.File, v.Line, v.Message)
			}
		}

		fmt.Printf("\n=== Policy Summary ===\n")
		fmt.Printf("  Errors:   %d\n", lintResult.Summary.Errors)
		fmt.Printf("  Warnings: %d\n", lintResult.Summary.Warnings)
		fmt.Printf("  Info:     %d\n", lintResult.Summary.Info)

		fmt.Printf("\n=== Extraction Summary ===\n")
		fmt.Printf("  Files:    %d\n", lintResult.Stats.Files)
		fmt.Printf("  Symbols:  %d\n", lintResult.Stats.Symbols)
		fmt.Printf("  Entities: %d\n", lintResult.Stats.Entities)
		fmt.Printf("  Packages: %d\n", lintResult.Stats.Packages)
		fmt.Printf("  Signals:  %d\n", lintResult.Stats.Signals)
		fmt.Printf("  Ports:    %d\n", lintResult.Stats.Ports)

		if len(lintResult.ParseErrors) > 0 {
			fmt.Printf("\n=== Parse Errors ===\n")
			for _, e := range lintResult.ParseErrors {
				fmt.Printf("  %s\n", e.Message)
			}
		}
	}
	policyDuration := time.Since(stepStart)
	policyStatus := ""
	if policyUsedDaemon {
		if policyDelta {
			policyStatus = "daemon_delta"
		} else {
			policyStatus = "daemon_init"
		}
	} else if policyCached {
		policyStatus = "cached"
	}
	timing.RecordStage("policy", stepStart, policyDuration, policyStatus)

	if (idx.Verbose || idx.Progress || idx.Trace) && !idx.JSONOutput {
		fmt.Printf("\n=== Timing Summary ===\n")
		fmt.Printf("  scan:        %s\n", formatDuration(scanDuration))
		fmt.Printf("  extract:     %s\n", formatDuration(extractDuration))
		fmt.Printf("  elaborate:   %s\n", formatDuration(elabDuration))
		if factsValidateDuration > 0 {
			fmt.Printf("  facts:       %s\n", formatDuration(factsValidateDuration))
		}
		fmt.Printf("  resolve:     %s\n", formatDuration(resolveDuration))
		fmt.Printf("  build input: %s\n", formatDuration(buildDuration))
		fmt.Printf("  validate:    %s\n", formatDuration(validateDuration))
		if policyUsedDaemon {
			label := "daemon (init)"
			if policyDelta {
				label = "daemon (delta)"
			}
			fmt.Printf("  policy:      %s (%s)\n", label, formatDuration(policyDuration))
		} else if policyCached {
			fmt.Printf("  policy:      cached (%s)\n", formatDuration(policyDuration))
		} else {
			fmt.Printf("  policy:      %s\n", formatDuration(policyDuration))
		}
		fmt.Printf("  total:       %s\n", formatDuration(time.Since(runStart)))
	}
	timing.RecordStage("total", runStart, time.Since(runStart), "")

	if len(pipelineErrs) > 0 {
		return fmt.Errorf("pipeline errors:\n%s", formatPipelineErrors(pipelineErrs))
	}
	return nil
}

func formatPipelineErrors(errs []error) string {
	var b strings.Builder
	for i, err := range errs {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("- ")
		b.WriteString(err.Error())
	}
	return b.String()
}

type policyRuleConfigSummary struct {
	ConfigPath         string
	ConfigLabel        string
	DisabledByConfig   []string
	OptionalNotEnabled []string
}

func emitPolicyRuleConfigSummary(cfg *config.Config) {
	if cfg == nil {
		return
	}
	summary := buildPolicyRuleConfigSummary(cfg)
	fmt.Printf("\n=== Policy Configuration ===\n")
	fmt.Printf("  config: %s\n", summary.ConfigLabel)
	if len(summary.DisabledByConfig) == 0 {
		fmt.Printf("  rules disabled via config: none\n")
	} else {
		fmt.Printf("  rules disabled via config (%d):\n", len(summary.DisabledByConfig))
		for _, rule := range summary.DisabledByConfig {
			fmt.Printf("    - %s\n", rule)
		}
	}
	if len(summary.OptionalNotEnabled) == 0 {
		fmt.Printf("  optional rules not enabled: none\n")
	} else {
		fmt.Printf("  optional rules not enabled (%d):\n", len(summary.OptionalNotEnabled))
		for _, rule := range summary.OptionalNotEnabled {
			fmt.Printf("    - %s\n", rule)
		}
	}
	if summary.ConfigPath == "" {
		fmt.Printf("  action: create vhdl_lint.json or .vhdl_lint.json to enable optional rules\n")
	} else if len(summary.OptionalNotEnabled) > 0 {
		fmt.Printf("  action: edit %s (lint.rules.<id> = \"warning\"|\"error\")\n", summary.ConfigPath)
	}
}

func buildPolicyRuleConfigSummary(cfg *config.Config) policyRuleConfigSummary {
	summary := policyRuleConfigSummary{}
	if cfg == nil {
		summary.ConfigLabel = "default (no config file found)"
		return summary
	}
	summary.ConfigPath = cfg.ConfigPath()
	if summary.ConfigPath == "" {
		summary.ConfigLabel = "default (no config file found)"
	} else {
		summary.ConfigLabel = summary.ConfigPath
	}

	rules := cfg.Lint.Rules
	if rules == nil {
		rules = map[string]string{}
	}

	for rule, severity := range rules {
		if strings.EqualFold(strings.TrimSpace(severity), "off") {
			summary.DisabledByConfig = append(summary.DisabledByConfig, rule)
		}
	}

	optionalRules := policy.OptionalRuleIDs()
	for _, rule := range optionalRules {
		if _, ok := rules[rule]; !ok {
			summary.OptionalNotEnabled = append(summary.OptionalNotEnabled, rule)
		}
	}

	sort.Strings(summary.DisabledByConfig)
	sort.Strings(summary.OptionalNotEnabled)
	return summary
}

func applyPolicyResult(lintResult *LintResult, result *policy.Result) {
	if lintResult == nil || result == nil {
		return
	}
	lintResult.Violations = result.Violations
	lintResult.MissingChecks = result.MissingChecks
	lintResult.AmbiguousConstructs = result.AmbiguousConstructs
	lintResult.Waivers = result.Waivers
	lintResult.Summary = ResultSummary{
		TotalViolations: result.Summary.TotalViolations,
		Errors:          result.Summary.Errors,
		Warnings:        result.Summary.Warnings,
		Info:            result.Summary.Info,
	}

	fileViolations := make(map[string]*FileResult)
	for _, v := range result.Violations {
		fr, ok := fileViolations[v.File]
		if !ok {
			fr = &FileResult{Path: v.File}
			fileViolations[v.File] = fr
		}
		switch v.Severity {
		case "error":
			fr.Errors++
		case "warning":
			fr.Warnings++
		case "info":
			fr.Info++
		}
	}
	lintResult.Files = lintResult.Files[:0]
	for _, fr := range fileViolations {
		lintResult.Files = append(lintResult.Files, *fr)
	}
}

func runPolicyDaemon(cacheDir string, cacheEnabled bool, tables facts.Tables, changedFiles map[string]bool) (*policy.Result, bool, error) {
	daemon, err := policy.NewDaemon(".")
	if err != nil {
		return nil, false, err
	}
	defer func() {
		_ = daemon.Close()
	}()

	if cacheEnabled {
		if prev, ok, err := loadFactTablesCache(cacheDir); err != nil {
			return nil, false, err
		} else if ok {
			if _, err := daemon.Init(prev); err != nil {
				return nil, false, err
			}
			delta := facts.ComputeDelta(prev, tables)
			if len(changedFiles) > 0 {
				delta = facts.FilterDeltaByFiles(delta, changedFiles)
			}
			result, err := daemon.Delta(delta)
			return result, true, err
		}
	}

	result, err := daemon.Init(tables)
	return result, false, err
}

func envBool(key string) bool {
	val := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	return val == "1" || val == "true" || val == "yes" || val == "on"
}

func sortedFactFiles(tables facts.Tables) []string {
	files := make([]string, 0, len(tables.Files))
	for _, file := range tables.Files {
		files = append(files, file.Path)
	}
	sort.Strings(files)
	return files
}

func fileListFromFacts(factsList []extractor.FileFacts) []string {
	seen := make(map[string]bool)
	files := make([]string, 0, len(factsList))
	for _, facts := range factsList {
		if facts.File == "" || seen[facts.File] {
			continue
		}
		seen[facts.File] = true
		files = append(files, facts.File)
	}
	sort.Strings(files)
	return files
}

// buildPolicyInput converts extracted facts to the policy engine input format
func (idx *Indexer) buildPolicyInput() policy.Input {
	files := fileListFromFacts(idx.Facts)
	input := idx.newPolicyInput(files)
	mode := "full"
	if idx.Config != nil {
		if m := strings.TrimSpace(idx.Config.Analysis.PolicyInputMode); m != "" {
			mode = m
		}
	}
	for _, facts := range idx.Facts {
		idx.appendFactsToPolicyInput(&input, facts, strings.ToLower(mode))
	}
	idx.finalizePolicyInput(&input, strings.ToLower(mode))
	return input
}

func validateVerificationTags(v *validator.Validator, input *policy.Input) error {
	if len(input.VerificationTags) == 0 {
		return nil
	}
	valid := input.VerificationTags[:0]
	for _, tag := range input.VerificationTags {
		if err := v.ValidateVerificationTag(tag); err != nil {
			input.VerificationTagErrors = append(input.VerificationTagErrors, policy.VerificationTagError{
				File:    tag.File,
				Line:    tag.Line,
				Raw:     tag.Raw,
				Message: err.Error(),
				InArch:  tag.InArch,
			})
			continue
		}
		if tag.Bindings == nil {
			tag.Bindings = map[string]string{}
		}
		valid = append(valid, tag)
	}
	input.VerificationTags = valid
	return nil
}

func validateVerificationWaivers(v *validator.Validator, input *policy.Input) error {
	if len(input.VerificationWaivers) == 0 {
		return nil
	}
	valid := input.VerificationWaivers[:0]
	for _, waiver := range input.VerificationWaivers {
		if err := v.ValidateVerificationWaiver(waiver); err != nil {
			input.VerificationWaiverErrors = append(input.VerificationWaiverErrors, policy.VerificationWaiverError{
				File:    waiver.File,
				Line:    waiver.Line,
				Raw:     waiver.Raw,
				Message: err.Error(),
				InArch:  waiver.InArch,
			})
			continue
		}
		valid = append(valid, waiver)
	}
	input.VerificationWaivers = valid
	return nil
}

func (idx *Indexer) populateScopesDefsUses(input *policy.Input) {
	scopeByID := make(map[string]policy.Scope)
	fileScope := make(map[string]string)
	entityScopes := make(map[string]map[string]string)
	packageScopes := make(map[string]map[string]string)
	archScopes := make(map[string]map[string]string)
	archPathScopes := make(map[string]map[string]string)

	normalize := func(name string) string {
		return strings.ToLower(strings.TrimSpace(name))
	}

	ensureMap := func(m map[string]map[string]string, file string) map[string]string {
		if m[file] == nil {
			m[file] = make(map[string]string)
		}
		return m[file]
	}

	addScope := func(id, kind, file string, line int, parent string) string {
		if id == "" {
			return ""
		}
		if existing, ok := scopeByID[id]; ok {
			if line > 0 && (existing.Line < 1 || (existing.Line == 1 && line != 1)) {
				existing.Line = line
				scopeByID[id] = existing
			}
			return id
		}
		if line < 1 {
			line = 1
		}
		path := []string{id}
		if parent != "" {
			if parentScope, ok := scopeByID[parent]; ok && len(parentScope.Path) > 0 {
				path = append(append([]string{}, parentScope.Path...), id)
			} else {
				path = []string{parent, id}
			}
		}
		scopeByID[id] = policy.Scope{
			Name:   id,
			Kind:   kind,
			File:   file,
			Line:   line,
			Parent: parent,
			Path:   path,
		}
		return id
	}

	ensureFileScope := func(file string) string {
		if file == "" {
			return ""
		}
		if id, ok := fileScope[file]; ok {
			return id
		}
		id := "file:" + file
		addScope(id, "file", file, 1, "")
		fileScope[file] = id
		return id
	}

	ensureEntityScope := func(file, name string, line int) string {
		if name == "" {
			return ensureFileScope(file)
		}
		m := ensureMap(entityScopes, file)
		key := normalize(name)
		if id, ok := m[key]; ok {
			return id
		}
		parent := ensureFileScope(file)
		id := parent + "::entity:" + key
		addScope(id, "entity", file, line, parent)
		m[key] = id
		return id
	}

	ensurePackageScope := func(file, name string, line int) string {
		if name == "" {
			return ensureFileScope(file)
		}
		m := ensureMap(packageScopes, file)
		key := normalize(name)
		if id, ok := m[key]; ok {
			return id
		}
		parent := ensureFileScope(file)
		id := parent + "::package:" + key
		addScope(id, "package", file, line, parent)
		m[key] = id
		return id
	}

	ensureArchScope := func(file, name string, line int) string {
		if name == "" {
			return ensureFileScope(file)
		}
		m := ensureMap(archScopes, file)
		key := normalize(name)
		if id, ok := m[key]; ok {
			return id
		}
		parent := ensureFileScope(file)
		id := parent + "::arch:" + key
		addScope(id, "architecture", file, line, parent)
		m[key] = id
		ensureMap(archPathScopes, file)[key] = id
		return id
	}

	splitScopePath := func(path string) []string {
		raw := strings.Split(path, ".")
		parts := make([]string, 0, len(raw))
		for _, part := range raw {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			parts = append(parts, part)
		}
		return parts
	}

	ensureArchPathScope := func(file, archPath string) string {
		parts := splitScopePath(archPath)
		if len(parts) == 0 {
			return ensureFileScope(file)
		}
		archName := parts[0]
		parent := ensureArchScope(file, archName, 1)
		currentKey := normalize(archName)
		ensureMap(archPathScopes, file)[currentKey] = parent
		for _, seg := range parts[1:] {
			if seg == "" {
				continue
			}
			segKey := normalize(seg)
			currentKey = currentKey + "." + segKey
			if id, ok := ensureMap(archPathScopes, file)[currentKey]; ok {
				parent = id
				continue
			}
			id := parent + "::generate:" + segKey
			addScope(id, "generate", file, 1, parent)
			ensureMap(archPathScopes, file)[currentKey] = id
			parent = id
		}
		return parent
	}

	ensureGenerateScope := func(file string, gen policy.GenerateStatement) string {
		parent := ensureArchPathScope(file, gen.InArch)
		label := strings.TrimSpace(gen.Label)
		if label == "" {
			label = fmt.Sprintf("gen@%d", gen.Line)
		}
		pathKey := normalize(strings.Trim(strings.Join([]string{gen.InArch, label}, "."), "."))
		if pathKey == "" {
			return parent
		}
		if id, ok := ensureMap(archPathScopes, file)[pathKey]; ok {
			if scope, ok := scopeByID[id]; ok && gen.Line > 0 && (scope.Line < 1 || (scope.Line == 1 && gen.Line != 1)) {
				scope.Line = gen.Line
				scopeByID[id] = scope
			}
			return id
		}
		id := parent + "::generate:" + normalize(label)
		addScope(id, "generate", file, gen.Line, parent)
		ensureMap(archPathScopes, file)[pathKey] = id
		return id
	}

	scopeForContext := func(file, context string) string {
		ctx := normalize(context)
		if ctx == "" {
			return ensureFileScope(file)
		}
		if scopes := packageScopes[file]; scopes != nil {
			if id, ok := scopes[ctx]; ok {
				return id
			}
		}
		if scopes := entityScopes[file]; scopes != nil {
			if id, ok := scopes[ctx]; ok {
				return id
			}
		}
		if scopes := archPathScopes[file]; scopes != nil {
			if id, ok := scopes[ctx]; ok {
				return id
			}
		}
		if scopes := archScopes[file]; scopes != nil {
			if id, ok := scopes[ctx]; ok {
				return id
			}
		}
		return ensureFileScope(file)
	}

	// Seed file scopes
	for _, fileInfo := range input.Files {
		ensureFileScope(fileInfo.Path)
	}

	// Build scopes for entities, packages, architectures, and generates
	for _, ent := range input.Entities {
		ensureEntityScope(ent.File, ent.Name, ent.Line)
	}
	for _, pkg := range input.Packages {
		ensurePackageScope(pkg.File, pkg.Name, pkg.Line)
	}
	for _, arch := range input.Architectures {
		ensureArchScope(arch.File, arch.Name, arch.Line)
	}
	for _, gen := range input.Generates {
		ensureGenerateScope(gen.File, gen)
	}

	// Export scopes in deterministic order
	scopeIDs := make([]string, 0, len(scopeByID))
	for id := range scopeByID {
		scopeIDs = append(scopeIDs, id)
	}
	sort.Strings(scopeIDs)
	for _, id := range scopeIDs {
		input.Scopes = append(input.Scopes, scopeByID[id])
	}

	// Symbol definitions
	for _, ent := range input.Entities {
		fileScopeID := ensureFileScope(ent.File)
		input.SymbolDefs = append(input.SymbolDefs, policy.SymbolDef{
			Name:  ent.Name,
			Kind:  "entity",
			File:  ent.File,
			Line:  ent.Line,
			Scope: fileScopeID,
		})
		entityScopeID := ensureEntityScope(ent.File, ent.Name, ent.Line)
		for _, port := range ent.Ports {
			input.SymbolDefs = append(input.SymbolDefs, policy.SymbolDef{
				Name:  port.Name,
				Kind:  "port",
				File:  ent.File,
				Line:  port.Line,
				Scope: entityScopeID,
			})
		}
		for _, gen := range ent.Generics {
			input.SymbolDefs = append(input.SymbolDefs, policy.SymbolDef{
				Name:  gen.Name,
				Kind:  "generic",
				File:  ent.File,
				Line:  gen.Line,
				Scope: entityScopeID,
			})
		}
	}

	for _, arch := range input.Architectures {
		fileScopeID := ensureFileScope(arch.File)
		input.SymbolDefs = append(input.SymbolDefs, policy.SymbolDef{
			Name:  arch.Name,
			Kind:  "architecture",
			File:  arch.File,
			Line:  arch.Line,
			Scope: fileScopeID,
		})
	}

	for _, pkg := range input.Packages {
		pkgScopeID := ensurePackageScope(pkg.File, pkg.Name, pkg.Line)
		input.SymbolDefs = append(input.SymbolDefs, policy.SymbolDef{
			Name:  pkg.Name,
			Kind:  "package",
			File:  pkg.File,
			Line:  pkg.Line,
			Scope: pkgScopeID,
		})
	}

	for _, sig := range input.Signals {
		scopeID := scopeForContext(sig.File, sig.InEntity)
		input.SymbolDefs = append(input.SymbolDefs, policy.SymbolDef{
			Name:  sig.Name,
			Kind:  "signal",
			File:  sig.File,
			Line:  sig.Line,
			Scope: scopeID,
		})
	}

	for _, typ := range input.Types {
		scopeName := typ.InPackage
		if scopeName == "" {
			scopeName = typ.InArch
		}
		scopeID := scopeForContext(typ.File, scopeName)
		input.SymbolDefs = append(input.SymbolDefs, policy.SymbolDef{
			Name:  typ.Name,
			Kind:  "type",
			File:  typ.File,
			Line:  typ.Line,
			Scope: scopeID,
		})
	}

	for _, st := range input.Subtypes {
		scopeName := st.InPackage
		if scopeName == "" {
			scopeName = st.InArch
		}
		scopeID := scopeForContext(st.File, scopeName)
		input.SymbolDefs = append(input.SymbolDefs, policy.SymbolDef{
			Name:  st.Name,
			Kind:  "subtype",
			File:  st.File,
			Line:  st.Line,
			Scope: scopeID,
		})
	}

	for _, fn := range input.Functions {
		scopeName := fn.InPackage
		if scopeName == "" {
			scopeName = fn.InArch
		}
		scopeID := scopeForContext(fn.File, scopeName)
		input.SymbolDefs = append(input.SymbolDefs, policy.SymbolDef{
			Name:  fn.Name,
			Kind:  "function",
			File:  fn.File,
			Line:  fn.Line,
			Scope: scopeID,
		})
	}

	for _, pr := range input.Procedures {
		scopeName := pr.InPackage
		if scopeName == "" {
			scopeName = pr.InArch
		}
		scopeID := scopeForContext(pr.File, scopeName)
		input.SymbolDefs = append(input.SymbolDefs, policy.SymbolDef{
			Name:  pr.Name,
			Kind:  "procedure",
			File:  pr.File,
			Line:  pr.Line,
			Scope: scopeID,
		})
	}

	for _, c := range input.ConstantDecls {
		scopeName := c.InPackage
		if scopeName == "" {
			scopeName = c.InArch
		}
		scopeID := scopeForContext(c.File, scopeName)
		input.SymbolDefs = append(input.SymbolDefs, policy.SymbolDef{
			Name:  c.Name,
			Kind:  "constant",
			File:  c.File,
			Line:  c.Line,
			Scope: scopeID,
		})
	}

	// Name uses from processes
	for _, proc := range input.Processes {
		scopeID := scopeForContext(proc.File, proc.InArch)
		context := proc.Label
		if context == "" {
			context = fmt.Sprintf("process@%d", proc.Line)
		}
		for _, call := range proc.FunctionCalls {
			name := strings.TrimSpace(call.Name)
			if name == "" {
				continue
			}
			input.NameUses = append(input.NameUses, policy.NameUse{
				Name:    name,
				Kind:    "function_call",
				File:    proc.File,
				Line:    call.Line,
				Scope:   scopeID,
				Context: context,
			})
		}
		for _, call := range proc.ProcedureCalls {
			name := strings.TrimSpace(call.FullName)
			if name == "" {
				name = strings.TrimSpace(call.Name)
			}
			if name == "" {
				continue
			}
			input.NameUses = append(input.NameUses, policy.NameUse{
				Name:    name,
				Kind:    "procedure_call",
				File:    proc.File,
				Line:    call.Line,
				Scope:   scopeID,
				Context: context,
			})
		}
	}

	// Name uses from signal dependencies
	for _, dep := range input.SignalDeps {
		scopeID := scopeForContext(dep.File, dep.InArch)
		context := dep.InProcess
		if dep.Source != "" {
			input.NameUses = append(input.NameUses, policy.NameUse{
				Name:    dep.Source,
				Kind:    "signal_read",
				File:    dep.File,
				Line:    dep.Line,
				Scope:   scopeID,
				Context: context,
			})
		}
		if dep.Target != "" {
			input.NameUses = append(input.NameUses, policy.NameUse{
				Name:    dep.Target,
				Kind:    "signal_write",
				File:    dep.File,
				Line:    dep.Line,
				Scope:   scopeID,
				Context: context,
			})
		}
	}
}

func (idx *Indexer) buildSymbolRows() []facts.SymbolRow {
	if idx.Symbols == nil {
		return nil
	}
	all := idx.Symbols.All()
	rows := make([]facts.SymbolRow, 0, len(all))
	for _, sym := range all {
		rows = append(rows, facts.SymbolRow{
			Name: sym.Name,
			Kind: sym.Kind,
			File: sym.File,
			Line: sym.Line,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Name == rows[j].Name {
			return rows[i].File < rows[j].File
		}
		return rows[i].Name < rows[j].Name
	})
	return rows
}

func (idx *Indexer) findVHDLFiles(root string) ([]string, error) {
	var files []string

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext == ".vhd" || ext == ".vhdl" {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}

// SymbolTable methods

func (st *SymbolTable) Add(sym Symbol) {
	st.mu.Lock()
	defer st.mu.Unlock()
	st.symbols[sym.Name] = sym
}

func (st *SymbolTable) Has(name string) bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	_, ok := st.symbols[name]
	return ok
}

func (st *SymbolTable) HasSuffix(base string) bool {
	st.mu.RLock()
	defer st.mu.RUnlock()
	suffix := "." + base
	for name := range st.symbols {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

func (st *SymbolTable) Get(name string) (Symbol, bool) {
	st.mu.RLock()
	defer st.mu.RUnlock()
	sym, ok := st.symbols[name]
	return sym, ok
}

func (st *SymbolTable) All() map[string]Symbol {
	st.mu.RLock()
	defer st.mu.RUnlock()
	// Return a copy
	result := make(map[string]Symbol)
	for k, v := range st.symbols {
		result[k] = v
	}
	return result
}

func (st *SymbolTable) Len() int {
	st.mu.RLock()
	defer st.mu.RUnlock()
	return len(st.symbols)
}

// isStandardLibrary checks if a library is a standard/vendor library
func isStandardLibrary(name string) bool {
	standard := []string{
		"ieee.", "std.", "std_logic_1164", "numeric_std",
		"textio", "math_real", "math_complex",
	}
	for _, prefix := range standard {
		if strings.HasPrefix(name, prefix) || strings.Contains(name, prefix) {
			return true
		}
	}
	return false
}

func isStandardLibraryName(name string) bool {
	lower := strings.ToLower(name)
	if lower == "ieee" || lower == "std" {
		return true
	}
	return isStandardLibrary(lower)
}

func normalizeUseTarget(target, fileLib string) string {
	trimmed := strings.TrimSpace(strings.ToLower(target))
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) == 1 {
		return fileLib + "." + parts[0]
	}
	if len(parts) == 2 && parts[1] == "all" {
		return fileLib + "." + parts[0]
	}
	lib := parts[0]
	pkg := parts[1]
	if lib == "work" {
		lib = fileLib
	}
	if lib == "" || pkg == "" {
		return ""
	}
	return lib + "." + pkg
}

func normalizeContextTarget(target, fileLib string) string {
	trimmed := strings.TrimSpace(strings.ToLower(target))
	if trimmed == "" {
		return ""
	}
	parts := strings.Split(trimmed, ".")
	if len(parts) == 1 {
		return fileLib + "." + parts[0]
	}
	lib := parts[0]
	name := parts[1]
	if lib == "work" {
		lib = fileLib
	}
	if lib == "" || name == "" {
		return ""
	}
	return lib + "." + name
}

func (idx *Indexer) hasLibrary(name string) bool {
	if name == "" || idx.Config == nil {
		return false
	}
	for lib := range idx.Config.Libraries {
		if strings.EqualFold(lib, name) {
			return true
		}
	}
	for _, entry := range idx.Config.Files {
		if entry.Library != "" && strings.EqualFold(entry.Library, name) {
			return true
		}
	}
	return false
}

func (idx *Indexer) findMissingLibraries(clauses []policy.LibraryClause) []string {
	if idx.Config == nil {
		return nil
	}
	known := make(map[string]bool)
	known["work"] = true
	for lib := range idx.Config.Libraries {
		if lib == "" {
			continue
		}
		known[strings.ToLower(lib)] = true
	}
	for _, entry := range idx.Config.Files {
		if entry.Library == "" {
			continue
		}
		known[strings.ToLower(entry.Library)] = true
	}
	for _, info := range idx.FileLibraries {
		if info.LibraryName == "" {
			continue
		}
		known[strings.ToLower(info.LibraryName)] = true
	}

	missing := make(map[string]bool)
	for _, clause := range clauses {
		for _, lib := range clause.Libraries {
			name := strings.ToLower(strings.TrimSpace(lib))
			if name == "" {
				continue
			}
			if isStandardLibraryName(name) || known[name] {
				continue
			}
			missing[name] = true
		}
	}

	if len(missing) == 0 {
		return nil
	}
	list := make([]string, 0, len(missing))
	for lib := range missing {
		list = append(list, lib)
	}
	sort.Strings(list)
	return list
}

func isValidIdentifierName(name string) bool {
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, "\\") && strings.HasSuffix(name, "\\") && len(name) > 2 {
		return true
	}
	first := name[0]
	if first != '_' && !isAlpha(first) {
		return false
	}
	for i := 1; i < len(name); i++ {
		b := name[i]
		if !isAlpha(b) && (b < '0' || b > '9') && b != '_' {
			return false
		}
	}
	return true
}

func formatDepTargets(deps []extractor.Dependency) string {
	if len(deps) == 0 {
		return ""
	}
	seen := make(map[string]bool)
	var targets []string
	for _, dep := range deps {
		if dep.Target == "" {
			continue
		}
		if !seen[dep.Target] {
			seen[dep.Target] = true
			targets = append(targets, dep.Target)
		}
	}
	if len(targets) == 0 {
		return ""
	}
	sort.Strings(targets)
	const max = 6
	if len(targets) > max {
		return fmt.Sprintf("%s, ... (+%d more)", strings.Join(targets[:max], ", "), len(targets)-max)
	}
	return strings.Join(targets, ", ")
}

func formatDuration(d time.Duration) string {
	switch {
	case d < time.Microsecond:
		return fmt.Sprintf("%dns", d.Nanoseconds())
	case d < time.Millisecond:
		return fmt.Sprintf("%dus", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%.2fms", float64(d)/float64(time.Millisecond))
	case d < time.Minute:
		return fmt.Sprintf("%.2fs", d.Seconds())
	case d < time.Hour:
		return fmt.Sprintf("%.2fm", d.Minutes())
	default:
		return fmt.Sprintf("%.2fh", d.Hours())
	}
}

func emitProgress(mu *sync.Mutex, progress *int, total int, facts extractor.FileFacts, status string, trace bool, duration time.Duration) {
	deps := formatDepTargets(facts.Dependencies)
	mu.Lock()
	defer mu.Unlock()
	*progress = *progress + 1
	extras := []string{}
	if facts.ErrorNodeCount > 0 {
		extras = append(extras, fmt.Sprintf("%d parse errors", facts.ErrorNodeCount))
	}
	if n := fallbackTotal(facts.FallbackStats); n > 0 {
		extras = append(extras, fmt.Sprintf("%d fallbacks", n))
	}
	errSuffix := ""
	if len(extras) > 0 {
		errSuffix = ", " + strings.Join(extras, ", ")
	}
	fmt.Printf("  [%d/%d] %s (%s, %s%s)\n", *progress, total, facts.File, status, formatDuration(duration), errSuffix)
	if deps != "" {
		fmt.Printf("    deps: %s\n", deps)
	}
	if trace {
		for _, line := range formatFactsSummary(facts) {
			fmt.Printf("    %s\n", line)
		}
	}
}

func formatFactsSummary(facts extractor.FileFacts) []string {
	errStr := ""
	if facts.ErrorNodeCount > 0 {
		errStr = fmt.Sprintf(" ERRORS=%d", facts.ErrorNodeCount)
	}
	if n := fallbackTotal(facts.FallbackStats); n > 0 {
		errStr += fmt.Sprintf(" FALLBACKS=%d", n)
	}
	lines := []string{
		fmt.Sprintf("facts: entities=%d packages=%d arch=%d signals=%d ports=%d processes=%d instances=%d generates=%d deps=%d%s",
			len(facts.Entities), len(facts.Packages), len(facts.Architectures), len(facts.Signals), len(facts.Ports),
			len(facts.Processes), len(facts.Instances), len(facts.Generates), len(facts.Dependencies), errStr),
	}
	if summary := fallbackSummary(facts.FallbackStats, 5); summary != "" {
		lines = append(lines, "fallbacks: "+summary)
	}

	if names := summarizeEntities(facts, 6); names != "" {
		lines = append(lines, "entities: "+names)
	}
	if names := summarizePackages(facts, 6); names != "" {
		lines = append(lines, "packages: "+names)
	}
	if names := summarizeArchitectures(facts, 6); names != "" {
		lines = append(lines, "architectures: "+names)
	}
	if names := summarizeInstances(facts, 4); names != "" {
		lines = append(lines, "instances: "+names)
	}

	return lines
}

func summarizeEntities(facts extractor.FileFacts, max int) string {
	var names []string
	for _, e := range facts.Entities {
		if e.Name != "" {
			names = append(names, e.Name)
		}
	}
	return summarizeList(names, max)
}

func summarizePackages(facts extractor.FileFacts, max int) string {
	var names []string
	for _, p := range facts.Packages {
		if p.Name != "" {
			names = append(names, p.Name)
		}
	}
	return summarizeList(names, max)
}

func summarizeArchitectures(facts extractor.FileFacts, max int) string {
	var names []string
	for _, a := range facts.Architectures {
		if a.Name != "" {
			names = append(names, a.Name)
		}
	}
	return summarizeList(names, max)
}

func summarizeInstances(facts extractor.FileFacts, max int) string {
	var names []string
	for _, inst := range facts.Instances {
		if inst.Name == "" {
			continue
		}
		if inst.Target != "" {
			names = append(names, fmt.Sprintf("%s->%s", inst.Name, inst.Target))
		} else {
			names = append(names, inst.Name)
		}
	}
	return summarizeList(names, max)
}

func summarizeList(items []string, max int) string {
	if len(items) == 0 {
		return ""
	}
	sort.Strings(items)
	if len(items) > max {
		return fmt.Sprintf("%s, ... (+%d more)", strings.Join(items[:max], ", "), len(items)-max)
	}
	return strings.Join(items, ", ")
}

func fallbackTotal(stats map[string]int) int {
	total := 0
	for _, n := range stats {
		if n > 0 {
			total += n
		}
	}
	return total
}

func fallbackSummary(stats map[string]int, max int) string {
	if len(stats) == 0 {
		return ""
	}
	type kv struct {
		key string
		n   int
	}
	items := make([]kv, 0, len(stats))
	for key, n := range stats {
		if n > 0 {
			items = append(items, kv{key: key, n: n})
		}
	}
	if len(items) == 0 {
		return ""
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].n == items[j].n {
			return items[i].key < items[j].key
		}
		return items[i].n > items[j].n
	})
	if max <= 0 || max > len(items) {
		max = len(items)
	}
	parts := make([]string, 0, max)
	for i := 0; i < max; i++ {
		parts = append(parts, fmt.Sprintf("%s=%d", items[i].key, items[i].n))
	}
	if len(items) > max {
		return fmt.Sprintf("%s, ... (+%d more)", strings.Join(parts, ", "), len(items)-max)
	}
	return strings.Join(parts, ", ")
}

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
