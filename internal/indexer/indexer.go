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

	// Timing output (JSONL)
	Timing     bool
	TimingPath string

	// Optional extractor factory (for tests)
	extractorFactory func() FactsExtractor

	// Optional cache version override (for tests)
	cacheVersionOverride *cacheVersions
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

	// Summary counts
	Summary ResultSummary `json:"summary"`

	// Extraction statistics
	Stats ExtractionStats `json:"stats"`

	// Per-file breakdown
	Files []FileResult `json:"files"`

	// Parse errors encountered
	ParseErrors []ParseError `json:"parse_errors,omitempty"`
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

	// Reset per-run state
	idx.Symbols = &SymbolTable{symbols: make(map[string]Symbol)}
	idx.Facts = nil
	idx.FileLibraries = make(map[string]config.FileLibraryInfo)
	idx.ThirdPartyFiles = make(map[string]bool)

	// 1. Find all VHDL files using configuration
	stepStart := time.Now()
	var files []string
	var err error

	// Check if config has library definitions
	if len(idx.Config.Libraries) > 0 {
		libs, resolveErr := idx.Config.ResolveLibraries(rootPath)
		if resolveErr != nil {
			return fmt.Errorf("resolve libraries: %w", resolveErr)
		}

		// Collect all files and track library info
		fileSet := make(map[string]bool)
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
	var wg sync.WaitGroup
	var progressMu sync.Mutex
	progress := 0
	progressEnabled := (idx.Verbose || idx.Progress || idx.Trace) && !idx.JSONOutput
	if progressEnabled {
		fmt.Printf("\n=== Extraction Progress ===\n")
	}
	factsChan := make(chan extractor.FileFacts, len(files))
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
					factsChan <- facts
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
			factsChan <- facts
			idx.registerSymbolsForFacts(facts, f)
		}(file)
	}

	wg.Wait()
	close(factsChan)
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

	// Collect facts
	factsByFile := make(map[string]extractor.FileFacts)
	for facts := range factsChan {
		idx.Facts = append(idx.Facts, facts)
		factsByFile[facts.File] = facts
	}
	if cache != nil {
		if err := cache.Save(); err != nil {
			recordPipelineErr(fmt.Errorf("cache save failed: %w", err))
		}
	}
	extractDuration := time.Since(stepStart)
	timing.RecordStage("extract", stepStart, extractDuration, "")

	// Cache impact visualization (verbose/progress/trace)
	if cache != nil && progressEnabled && len(changedFiles) > 0 {
		fmt.Printf("\n=== Cache Impact ===\n")
		dependents := buildDependentsGraph(factsByFile, idx.Symbols, idx.FileLibraries)
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

	// Elaborate generate statements using constant values
	stepStart = time.Now()
	// Build a global constant map from all extracted constants
	globalConstants := make(map[string]int)
	for _, facts := range idx.Facts {
		constMap := extractor.BuildConstantMap(facts.ConstantDecls)
		for k, v := range constMap {
			globalConstants[k] = v
		}
	}

	// Elaborate generates in all files
	elaboratedCount := 0
	for i := range idx.Facts {
		elaboratedCount += extractor.ElaborateGenerates(idx.Facts[i].Generates, globalConstants)
	}
	if elaboratedCount > 0 && idx.Verbose {
		fmt.Printf("\n=== Verbose: Generate Elaboration ===\n")
		fmt.Printf("  Elaborated %d for-generates using %d constants\n", elaboratedCount, len(globalConstants))
	}
	elabDuration := time.Since(stepStart)
	timing.RecordStage("elaborate", stepStart, elabDuration, "")
	var factsValidateDuration time.Duration

	// Validate relational fact tables for Datalog ingestion
	stepStart = time.Now()
	factTables := facts.BuildTables(idx.Facts, idx.FileLibraries, idx.ThirdPartyFiles, idx.buildSymbolRows())
	factFiles := sortedFactFiles(factTables)
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
		fmt.Printf("\n=== Verbose: Extracted Ports ===\n")
		for _, facts := range idx.Facts {
			for _, p := range facts.Ports {
				fmt.Printf("  %s.%s: direction=%q type=%q\n", p.InEntity, p.Name, p.Direction, p.Type)
			}
		}
		fmt.Printf("\n=== Verbose: Extracted Processes ===\n")
		for _, facts := range idx.Facts {
			for _, p := range facts.Processes {
				kind := "combinational"
				if p.IsSequential {
					kind = "sequential"
				}
				fmt.Printf("  %s.%s: %s, sensitivity=%v\n", p.InArch, p.Label, kind, p.SensitivityList)
				if p.ClockSignal != "" {
					fmt.Printf("    clock: %s (%s_edge)\n", p.ClockSignal, p.ClockEdge)
				}
				if p.HasReset {
					asyncStr := "sync"
					if p.ResetAsync {
						asyncStr = "async"
					}
					fmt.Printf("    reset: %s (%s)\n", p.ResetSignal, asyncStr)
				}
				if len(p.AssignedSignals) > 0 {
					fmt.Printf("    writes: %v\n", p.AssignedSignals)
				}
				if len(p.ReadSignals) > 0 {
					fmt.Printf("    reads: %v\n", p.ReadSignals)
				}
			}
		}
		fmt.Printf("\n=== Verbose: Clock Domains ===\n")
		for _, facts := range idx.Facts {
			for _, cd := range facts.ClockDomains {
				fmt.Printf("  %s (%s): drives %v\n", cd.Clock, cd.Edge, cd.Registers)
			}
		}
		fmt.Printf("\n=== Verbose: Instances ===\n")
		for _, facts := range idx.Facts {
			for _, inst := range facts.Instances {
				fmt.Printf("  %s: %s\n", inst.Name, inst.Target)
				if len(inst.GenericMap) > 0 {
					fmt.Printf("    generics: %v\n", inst.GenericMap)
				}
				if len(inst.PortMap) > 0 {
					fmt.Printf("    ports: %v\n", inst.PortMap)
				}
			}
		}
		fmt.Printf("\n=== Verbose: Case Statements ===\n")
		for _, facts := range idx.Facts {
			for _, cs := range facts.CaseStatements {
				status := "INCOMPLETE (potential latch)"
				if cs.HasOthers {
					status = "complete (has others)"
				}
				fmt.Printf("  case %s [%s] line %d\n", cs.Expression, status, cs.Line)
				if cs.InProcess != "" {
					fmt.Printf("    in process: %s\n", cs.InProcess)
				}
				fmt.Printf("    choices: %v\n", cs.Choices)
			}
		}
		fmt.Printf("\n=== Verbose: Concurrent Assignments ===\n")
		for _, facts := range idx.Facts {
			for _, ca := range facts.ConcurrentAssignments {
				fmt.Printf("  %s <= [%s] (kind: %s, line %d)\n", ca.Target, strings.Join(ca.ReadSignals, ", "), ca.Kind, ca.Line)
			}
		}
		fmt.Printf("\n=== Verbose: Comparisons (Security Analysis) ===\n")
		for _, facts := range idx.Facts {
			for _, comp := range facts.Comparisons {
				litInfo := ""
				if comp.IsLiteral {
					litInfo = fmt.Sprintf(" [LITERAL: %s, %d bits]", comp.LiteralValue, comp.LiteralBits)
				}
				fmt.Printf("  %s %s %s%s (line %d, drives: %s)\n", comp.LeftOperand, comp.Operator, comp.RightOperand, litInfo, comp.Line, comp.ResultDrives)
			}
		}
		fmt.Printf("\n=== Verbose: Arithmetic Ops (Power Analysis) ===\n")
		for _, facts := range idx.Facts {
			for _, op := range facts.ArithmeticOps {
				guardInfo := "unguarded"
				if op.IsGuarded {
					guardInfo = fmt.Sprintf("guarded by %s", op.GuardSignal)
				}
				fmt.Printf("  %s: %s (%s, line %d)\n", op.Operator, strings.Join(op.Operands, ", "), guardInfo, op.Line)
			}
		}
		fmt.Printf("\n=== Verbose: Signal Dependencies (Loop Detection) ===\n")
		for _, facts := range idx.Facts {
			for _, dep := range facts.SignalDeps {
				seqInfo := "combinational"
				if dep.IsSequential {
					seqInfo = "sequential"
				}
				fmt.Printf("  %s -> %s (%s, line %d)\n", dep.Source, dep.Target, seqInfo, dep.Line)
			}
		}
		fmt.Printf("\n=== Verbose: Type Declarations ===\n")
		for _, facts := range idx.Facts {
			for _, t := range facts.Types {
				scope := t.InPackage
				if scope == "" {
					scope = t.InArch
				}
				fmt.Printf("  %s.%s: kind=%s line=%d\n", scope, t.Name, t.Kind, t.Line)
				if t.Kind == "enum" && len(t.EnumLiterals) > 0 {
					fmt.Printf("    literals: %v\n", t.EnumLiterals)
				}
				if t.Kind == "record" && len(t.Fields) > 0 {
					fmt.Printf("    fields:\n")
					for _, f := range t.Fields {
						fmt.Printf("      %s: %s\n", f.Name, f.Type)
					}
				}
				if t.Kind == "array" {
					unc := ""
					if t.Unconstrained {
						unc = " (unconstrained)"
					}
					fmt.Printf("    element: %s%s\n", t.ElementType, unc)
				}
			}
		}
		fmt.Printf("\n=== Verbose: Subtype Declarations ===\n")
		for _, facts := range idx.Facts {
			for _, st := range facts.Subtypes {
				scope := st.InPackage
				if scope == "" {
					scope = st.InArch
				}
				constraint := ""
				if st.Constraint != "" {
					constraint = " " + st.Constraint
				}
				fmt.Printf("  %s.%s: %s%s\n", scope, st.Name, st.BaseType, constraint)
			}
		}
		fmt.Printf("\n=== Verbose: Function Declarations ===\n")
		for _, facts := range idx.Facts {
			for _, fn := range facts.Functions {
				scope := fn.InPackage
				if scope == "" {
					scope = fn.InArch
				}
				purity := "pure"
				if !fn.IsPure {
					purity = "impure"
				}
				hasBody := ""
				if fn.HasBody {
					hasBody = " [body]"
				}
				fmt.Printf("  %s.%s: %s returns %s%s\n", scope, fn.Name, purity, fn.ReturnType, hasBody)
				if len(fn.Parameters) > 0 {
					fmt.Printf("    params:\n")
					for _, p := range fn.Parameters {
						dir := p.Direction
						if dir == "" {
							dir = "in"
						}
						fmt.Printf("      %s: %s %s\n", p.Name, dir, p.Type)
					}
				}
			}
		}
		fmt.Printf("\n=== Verbose: Procedure Declarations ===\n")
		for _, facts := range idx.Facts {
			for _, pr := range facts.Procedures {
				scope := pr.InPackage
				if scope == "" {
					scope = pr.InArch
				}
				hasBody := ""
				if pr.HasBody {
					hasBody = " [body]"
				}
				fmt.Printf("  %s.%s%s\n", scope, pr.Name, hasBody)
				if len(pr.Parameters) > 0 {
					fmt.Printf("    params:\n")
					for _, p := range pr.Parameters {
						dir := p.Direction
						if dir == "" {
							dir = "in"
						}
						fmt.Printf("      %s: %s %s\n", p.Name, dir, p.Type)
					}
				}
			}
		}
		fmt.Printf("\n=== Verbose: Constant Declarations ===\n")
		for _, facts := range idx.Facts {
			for _, c := range facts.ConstantDecls {
				scope := c.InPackage
				if scope == "" {
					scope = c.InArch
				}
				value := ""
				if c.Value != "" {
					value = fmt.Sprintf(" := %s", c.Value)
				}
				fmt.Printf("  %s.%s: %s%s\n", scope, c.Name, c.Type, value)
			}
		}
		fmt.Printf("\n=== Verbose: Generate Statements ===\n")
		for _, facts := range idx.Facts {
			for _, gen := range facts.Generates {
				switch gen.Kind {
				case "for":
					elaboration := "cannot elaborate"
					if gen.CanElaborate {
						elaboration = fmt.Sprintf("%d iterations", gen.IterationCount)
					}
					fmt.Printf("  %s: for %s in %s %s %s (%s)\n",
						gen.Label, gen.LoopVar, gen.RangeLow, gen.RangeDir, gen.RangeHigh, elaboration)
				case "if":
					fmt.Printf("  %s: if %s\n", gen.Label, gen.Condition)
				case "case":
					fmt.Printf("  %s: case %s\n", gen.Label, gen.Condition)
				default:
					fmt.Printf("  %s: %s\n", gen.Label, gen.Kind)
				}
				if len(gen.Signals) > 0 || len(gen.Instances) > 0 || len(gen.Processes) > 0 {
					fmt.Printf("    contains: %d signals, %d instances, %d processes\n",
						len(gen.Signals), len(gen.Instances), len(gen.Processes))
				}
			}
		}

		// CDC crossings
		fmt.Printf("\n=== Verbose: CDC Crossings ===\n")
		for _, facts := range idx.Facts {
			for _, cdc := range facts.CDCCrossings {
				syncStatus := "unsynchronized"
				if cdc.IsSynchronized {
					syncStatus = fmt.Sprintf("synchronized (%d stages)", cdc.SyncStages)
				}
				bitWidth := "single-bit"
				if cdc.IsMultiBit {
					bitWidth = "multi-bit"
				}
				fmt.Printf("  %s: %s -> %s (%s, %s) [%s]\n",
					cdc.Signal, cdc.SourceClock, cdc.DestClock, bitWidth, syncStatus,
					fmt.Sprintf("%s writes, %s reads", cdc.SourceProc, cdc.DestProc))
			}
		}
	}

	// 3. Pass 2: Resolution (check imports)
	stepStart = time.Now()
	// Note: "work" in VHDL is a relative reference to the file's own library.
	// We translate "work.x" to the file's actual library name for resolution.
	var missing []string
	for _, facts := range idx.Facts {
		// Get the actual library name for this file
		fileLib := "work"
		if libInfo, ok := idx.FileLibraries[facts.File]; ok && libInfo.LibraryName != "" {
			fileLib = strings.ToLower(libInfo.LibraryName)
		}

		for _, dep := range facts.Dependencies {
			qualName := strings.ToLower(dep.Target)

			// Translate "work.x" to the file's actual library
			if strings.HasPrefix(qualName, "work.") {
				qualName = fileLib + qualName[4:] // Replace "work" with actual library
			}
			// Unqualified names resolve in the local library by default
			if !strings.Contains(qualName, ".") {
				qualName = fileLib + "." + qualName
			}

			if !idx.Symbols.Has(qualName) && !isStandardLibrary(qualName) {
				missing = append(missing, fmt.Sprintf("%s: missing import %q", facts.File, dep.Target))
			}
		}
	}
	resolveDuration := time.Since(stepStart)
	timing.RecordStage("resolve", stepStart, resolveDuration, "")

	// 4. Build policy engine input
	stepStart = time.Now()
	policyInput := idx.buildPolicyInput()
	buildDuration := time.Since(stepStart)
	timing.RecordStage("build_input", stepStart, buildDuration, "")

	// 5. Validate data structure before policy evaluation (CUE contract enforcement)
	stepStart = time.Now()
	v, err := validator.New()
	if err != nil {
		return fmt.Errorf("CRITICAL: Failed to initialize CUE validator: %w", err)
	}
	if err := validateVerificationTags(v, &policyInput); err != nil {
		return fmt.Errorf("CRITICAL: Failed to validate verification tags: %w", err)
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

	// Output results
	if idx.JSONOutput {
		// JSON output mode
		enc := json.NewEncoder(os.Stdout)
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

func applyPolicyResult(lintResult *LintResult, result *policy.Result) {
	if lintResult == nil || result == nil {
		return
	}
	lintResult.Violations = result.Violations
	lintResult.MissingChecks = result.MissingChecks
	lintResult.AmbiguousConstructs = result.AmbiguousConstructs
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

// buildPolicyInput converts extracted facts to the policy engine input format
func (idx *Indexer) buildPolicyInput() policy.Input {
	// Initialize all slices to empty (not nil) to ensure JSON serialization
	// produces [] instead of null - the CUE contract requires arrays
	input := policy.Input{
		Standard:              idx.Config.Standard,
		FileCount:             len(idx.Facts),
		Entities:              []policy.Entity{},
		Architectures:         []policy.Architecture{},
		Packages:              []policy.Package{},
		Components:            []policy.Component{},
		UseClauses:            []policy.UseClause{},
		LibraryClauses:        []policy.LibraryClause{},
		ContextClauses:        []policy.ContextClause{},
		Signals:               []policy.Signal{},
		Ports:                 []policy.Port{},
		Dependencies:          []policy.Dependency{},
		Symbols:               []policy.Symbol{},
		Scopes:                []policy.Scope{},
		SymbolDefs:            []policy.SymbolDef{},
		NameUses:              []policy.NameUse{},
		Files:                 []policy.FileInfo{},
		VerificationBlocks:    []policy.VerificationBlock{},
		VerificationTags:      []policy.VerificationTag{},
		VerificationTagErrors: []policy.VerificationTagError{},
		Instances:             []policy.Instance{},
		CaseStatements:        []policy.CaseStatement{},
		Processes:             []policy.Process{},
		ConcurrentAssignments: []policy.ConcurrentAssignment{},
		Generates:             []policy.GenerateStatement{},
		Configurations:        []policy.Configuration{},
		// Type system
		Types:         []policy.TypeDeclaration{},
		Subtypes:      []policy.SubtypeDeclaration{},
		Functions:     []policy.FunctionDeclaration{},
		Procedures:    []policy.ProcedureDeclaration{},
		ConstantDecls: []policy.ConstantDeclaration{},
		// Type system info for filtering (LEGACY)
		EnumLiterals:    []string{},
		Constants:       []string{},
		SharedVariables: []string{},
		// Advanced analysis
		Comparisons:   []policy.Comparison{},
		ArithmeticOps: []policy.ArithmeticOp{},
		SignalDeps:    []policy.SignalDep{},
		CDCCrossings:  []policy.CDCCrossing{},
		SignalUsages:  []policy.SignalUsage{},
		// Configuration
		LintConfig: policy.LintRuleConfig{
			Rules: idx.Config.Lint.Rules,
		},
		ThirdPartyFiles: []string{},
	}

	// Add third-party files list
	for f := range idx.ThirdPartyFiles {
		input.ThirdPartyFiles = append(input.ThirdPartyFiles, f)
	}

	// Add file/library mappings
	fileList := make([]string, 0, len(idx.Facts))
	for _, facts := range idx.Facts {
		fileList = append(fileList, facts.File)
	}
	sort.Strings(fileList)
	for _, file := range fileList {
		lib := "work"
		if info, ok := idx.FileLibraries[file]; ok && info.LibraryName != "" {
			lib = strings.ToLower(info.LibraryName)
		}
		input.Files = append(input.Files, policy.FileInfo{
			Path:         file,
			Library:      lib,
			IsThirdParty: idx.ThirdPartyFiles[file],
		})
	}

	// Aggregate facts from all files
	for _, facts := range idx.Facts {
		for _, e := range facts.Entities {
			// Find ports for this entity (initialize to empty, not nil)
			ports := []policy.Port{}
			generics := []policy.GenericDecl{}
			for _, p := range facts.Ports {
				if p.InEntity == e.Name {
					ports = append(ports, policy.Port{
						Name:      p.Name,
						Direction: p.Direction,
						Type:      p.Type,
						Default:   p.Default,
						Line:      p.Line,
						InEntity:  p.InEntity,
						Width:     extractor.CalculateWidth(p.Type),
					})
				}
			}
			for _, g := range e.Generics {
				generics = append(generics, policy.GenericDecl{
					Name:        g.Name,
					Kind:        g.Kind,
					Type:        g.Type,
					Class:       g.Class,
					Default:     g.Default,
					Line:        g.Line,
					InEntity:    g.InEntity,
					InComponent: g.InComponent,
				})
			}
			input.Entities = append(input.Entities, policy.Entity{
				Name:     e.Name,
				File:     facts.File,
				Line:     e.Line,
				Ports:    ports,
				Generics: generics,
			})
		}

		for _, a := range facts.Architectures {
			input.Architectures = append(input.Architectures, policy.Architecture{
				Name:       a.Name,
				EntityName: a.EntityName,
				File:       facts.File,
				Line:       a.Line,
			})
		}

		for _, p := range facts.Packages {
			input.Packages = append(input.Packages, policy.Package{
				Name: p.Name,
				File: facts.File,
				Line: p.Line,
			})
		}

		for _, u := range facts.UseClauses {
			input.UseClauses = append(input.UseClauses, policy.UseClause{
				Items: u.Items,
				File:  facts.File,
				Line:  u.Line,
			})
		}
		for _, l := range facts.LibraryClauses {
			input.LibraryClauses = append(input.LibraryClauses, policy.LibraryClause{
				Libraries: l.Libraries,
				File:      facts.File,
				Line:      l.Line,
			})
		}
		for _, c := range facts.ContextClauses {
			input.ContextClauses = append(input.ContextClauses, policy.ContextClause{
				Name: c.Name,
				File: facts.File,
				Line: c.Line,
			})
		}

		for _, sharedName := range facts.SharedVariables {
			input.SharedVariables = append(input.SharedVariables, sharedName)
		}

		for _, c := range facts.Components {
			componentPorts := []policy.Port{}
			componentGenerics := []policy.GenericDecl{}
			for _, p := range c.Ports {
				componentPorts = append(componentPorts, policy.Port{
					Name:      p.Name,
					Direction: p.Direction,
					Type:      p.Type,
					Line:      p.Line,
					InEntity:  p.InEntity,
					Width:     extractor.CalculateWidth(p.Type),
				})
			}
			for _, g := range c.Generics {
				componentGenerics = append(componentGenerics, policy.GenericDecl{
					Name:        g.Name,
					Kind:        g.Kind,
					Type:        g.Type,
					Class:       g.Class,
					Default:     g.Default,
					Line:        g.Line,
					InEntity:    g.InEntity,
					InComponent: g.InComponent,
				})
			}
			input.Components = append(input.Components, policy.Component{
				Name:       c.Name,
				EntityRef:  c.EntityRef,
				File:       facts.File,
				Line:       c.Line,
				IsInstance: c.IsInstance,
				Ports:      componentPorts,
				Generics:   componentGenerics,
			})
		}

		for _, s := range facts.Signals {
			// Skip signals with empty types (e.g., malformed declarations like "signal x: (range)")
			if s.Type == "" {
				continue
			}
			input.Signals = append(input.Signals, policy.Signal{
				Name:     s.Name,
				Type:     s.Type,
				File:     facts.File,
				Line:     s.Line,
				InEntity: s.InEntity,
				Width:    extractor.CalculateWidth(s.Type),
			})
		}

		for _, p := range facts.Ports {
			input.Ports = append(input.Ports, policy.Port{
				Name:      p.Name,
				Direction: p.Direction,
				Type:      p.Type,
				Line:      p.Line,
				InEntity:  p.InEntity,
				Width:     extractor.CalculateWidth(p.Type),
			})
		}

		for _, d := range facts.Dependencies {
			// Get the actual library name for this file
			fileLib := "work"
			if libInfo, ok := idx.FileLibraries[facts.File]; ok && libInfo.LibraryName != "" {
				fileLib = strings.ToLower(libInfo.LibraryName)
			}

			// Translate "work.x" to the file's actual library for resolution
			qualName := strings.ToLower(d.Target)
			if strings.HasPrefix(qualName, "work.") {
				qualName = fileLib + qualName[4:]
			}
			if !strings.Contains(qualName, ".") {
				qualName = fileLib + "." + qualName
			}

			resolved := idx.Symbols.Has(qualName) || isStandardLibrary(d.Target)
			if !resolved && d.Kind == "instantiation" {
				baseName := qualName
				if idxDot := strings.LastIndex(baseName, "."); idxDot != -1 {
					baseName = baseName[idxDot+1:]
				}
				if baseName != "" && idx.Symbols.HasSuffix(baseName) {
					resolved = true
				}
			}
			input.Dependencies = append(input.Dependencies, policy.Dependency{
				Source:   d.Source,
				Target:   d.Target,
				Kind:     d.Kind,
				Line:     d.Line,
				Resolved: resolved,
			})
		}

		for _, inst := range facts.Instances {
			// Ensure maps are not nil (CUE requires objects, not null)
			portMap := inst.PortMap
			if portMap == nil {
				portMap = make(map[string]string)
			}
			genericMap := inst.GenericMap
			if genericMap == nil {
				genericMap = make(map[string]string)
			}
			associations := []policy.Association{}
			for _, assoc := range inst.Associations {
				associations = append(associations, policy.Association{
					Kind:          assoc.Kind,
					Formal:        assoc.Formal,
					Actual:        assoc.Actual,
					IsPositional:  assoc.IsPositional,
					ActualKind:    assoc.ActualKind,
					ActualBase:    assoc.ActualBase,
					ActualFull:    assoc.ActualFull,
					Line:          assoc.Line,
					PositionIndex: assoc.PositionIndex,
				})
			}
			input.Instances = append(input.Instances, policy.Instance{
				Name:         inst.Name,
				Target:       inst.Target,
				PortMap:      portMap,
				GenericMap:   genericMap,
				Associations: associations,
				File:         facts.File,
				Line:         inst.Line,
				InArch:       inst.InArch,
			})
		}

		for _, cs := range facts.CaseStatements {
			// Ensure choices is not nil
			choices := cs.Choices
			if choices == nil {
				choices = []string{}
			}
			input.CaseStatements = append(input.CaseStatements, policy.CaseStatement{
				Expression: cs.Expression,
				Choices:    choices,
				HasOthers:  cs.HasOthers,
				File:       facts.File,
				Line:       cs.Line,
				InProcess:  cs.InProcess,
				InArch:     cs.InArch,
				IsComplete: cs.IsComplete,
			})
		}

		for _, proc := range facts.Processes {
			// Ensure slices are not nil
			sensList := proc.SensitivityList
			if sensList == nil {
				sensList = []string{}
			}
			assigned := proc.AssignedSignals
			if assigned == nil {
				assigned = []string{}
			}
			read := proc.ReadSignals
			if read == nil {
				read = []string{}
			}
			vars := []policy.VariableDecl{}
			for _, v := range proc.Variables {
				vars = append(vars, policy.VariableDecl{
					Name: v.Name,
					Type: v.Type,
					Line: v.Line,
				})
			}
			procCalls := []policy.ProcedureCall{}
			for _, c := range proc.ProcedureCalls {
				args := c.Args
				if args == nil {
					args = []string{}
				}
				procCalls = append(procCalls, policy.ProcedureCall{
					Name:      c.Name,
					FullName:  c.FullName,
					Args:      args,
					Line:      c.Line,
					InProcess: c.InProcess,
					InArch:    c.InArch,
				})
			}
			funcCalls := []policy.FunctionCall{}
			for _, c := range proc.FunctionCalls {
				args := c.Args
				if args == nil {
					args = []string{}
				}
				funcCalls = append(funcCalls, policy.FunctionCall{
					Name:      c.Name,
					Args:      args,
					Line:      c.Line,
					InProcess: c.InProcess,
					InArch:    c.InArch,
				})
			}
			waitStmts := []policy.WaitStatement{}
			for _, w := range proc.WaitStatements {
				onSignals := w.OnSignals
				if onSignals == nil {
					onSignals = []string{}
				}
				waitStmts = append(waitStmts, policy.WaitStatement{
					OnSignals: onSignals,
					UntilExpr: w.UntilExpr,
					ForExpr:   w.ForExpr,
					Line:      w.Line,
				})
			}
			input.Processes = append(input.Processes, policy.Process{
				Label:           proc.Label,
				SensitivityList: sensList,
				IsSequential:    proc.IsSequential,
				IsCombinational: proc.IsCombinational,
				ClockSignal:     proc.ClockSignal,
				ClockEdge:       proc.ClockEdge,
				HasReset:        proc.HasReset,
				ResetSignal:     proc.ResetSignal,
				ResetAsync:      proc.ResetAsync,
				AssignedSignals: assigned,
				ReadSignals:     read,
				Variables:       vars,
				ProcedureCalls:  procCalls,
				FunctionCalls:   funcCalls,
				WaitStatements:  waitStmts,
				File:            facts.File,
				Line:            proc.Line,
				InArch:          proc.InArch,
			})
		}

		for _, ca := range facts.ConcurrentAssignments {
			// Skip assignments with empty targets (edge cases from parsing errors)
			if ca.Target == "" {
				continue
			}
			// Ensure ReadSignals is not nil
			readSigs := ca.ReadSignals
			if readSigs == nil {
				readSigs = []string{}
			}
			input.ConcurrentAssignments = append(input.ConcurrentAssignments, policy.ConcurrentAssignment{
				Target:      ca.Target,
				ReadSignals: readSigs,
				File:        facts.File,
				Line:        ca.Line,
				InArch:      ca.InArch,
				Kind:        ca.Kind,
			})
		}

		// Advanced analysis: Comparisons for trojan/trigger detection
		for _, comp := range facts.Comparisons {
			input.Comparisons = append(input.Comparisons, policy.Comparison{
				LeftOperand:  comp.LeftOperand,
				Operator:     comp.Operator,
				RightOperand: comp.RightOperand,
				IsLiteral:    comp.IsLiteral,
				LiteralValue: comp.LiteralValue,
				LiteralBits:  comp.LiteralBits,
				ResultDrives: comp.ResultDrives,
				File:         facts.File,
				Line:         comp.Line,
				InProcess:    comp.InProcess,
				InArch:       comp.InArch,
			})
		}

		// Advanced analysis: Arithmetic operations for power analysis
		for _, arith := range facts.ArithmeticOps {
			// Ensure operands is not nil
			operands := arith.Operands
			if operands == nil {
				operands = []string{}
			}
			input.ArithmeticOps = append(input.ArithmeticOps, policy.ArithmeticOp{
				Operator:    arith.Operator,
				Operands:    operands,
				Result:      arith.Result,
				IsGuarded:   arith.IsGuarded,
				GuardSignal: arith.GuardSignal,
				File:        facts.File,
				Line:        arith.Line,
				InProcess:   arith.InProcess,
				InArch:      arith.InArch,
			})
		}

		// Advanced analysis: Signal dependencies for loop detection
		for _, dep := range facts.SignalDeps {
			input.SignalDeps = append(input.SignalDeps, policy.SignalDep{
				Source:       dep.Source,
				Target:       dep.Target,
				InProcess:    dep.InProcess,
				IsSequential: dep.IsSequential,
				File:         facts.File,
				Line:         dep.Line,
				InArch:       dep.InArch,
			})
		}

		// CDC crossings: signals crossing clock domains
		for _, cdc := range facts.CDCCrossings {
			input.CDCCrossings = append(input.CDCCrossings, policy.CDCCrossing{
				Signal:         cdc.Signal,
				SourceClock:    cdc.SourceClock,
				SourceProc:     cdc.SourceProc,
				DestClock:      cdc.DestClock,
				DestProc:       cdc.DestProc,
				IsSynchronized: cdc.IsSynchronized,
				SyncStages:     cdc.SyncStages,
				IsMultiBit:     cdc.IsMultiBit,
				File:           cdc.File,
				Line:           cdc.Line,
				InArch:         cdc.InArch,
			})
		}

		// Signal usages: tracking reads, writes, and port map connections
		for _, usage := range facts.SignalUsages {
			input.SignalUsages = append(input.SignalUsages, policy.SignalUsage{
				Signal:       usage.Signal,
				IsRead:       usage.IsRead,
				IsWritten:    usage.IsWritten,
				InProcess:    usage.InProcess,
				InPortMap:    usage.InPortMap,
				InstanceName: usage.InstanceName,
				InPSL:        usage.InPSL,
				Line:         usage.Line,
			})
		}

		// Generate statements (for-generate, if-generate, case-generate)
		for _, gen := range facts.Generates {
			input.Generates = append(input.Generates, policy.GenerateStatement{
				Label:          gen.Label,
				Kind:           gen.Kind,
				File:           facts.File,
				Line:           gen.Line,
				InArch:         gen.InArch,
				LoopVar:        gen.LoopVar,
				RangeLow:       gen.RangeLow,
				RangeHigh:      gen.RangeHigh,
				RangeDir:       gen.RangeDir,
				IterationCount: gen.IterationCount,
				CanElaborate:   gen.CanElaborate,
				Condition:      gen.Condition,
				SignalCount:    len(gen.Signals),
				InstanceCount:  len(gen.Instances),
				ProcessCount:   len(gen.Processes),
			})
		}

		// Configuration declarations
		for _, cfg := range facts.Configurations {
			input.Configurations = append(input.Configurations, policy.Configuration{
				Name:       cfg.Name,
				EntityName: cfg.EntityName,
				File:       facts.File,
				Line:       cfg.Line,
			})
		}

		for _, block := range facts.VerificationBlocks {
			input.VerificationBlocks = append(input.VerificationBlocks, policy.VerificationBlock{
				Label:     block.Label,
				LineStart: block.LineStart,
				LineEnd:   block.LineEnd,
				File:      facts.File,
				InArch:    block.InArch,
			})
		}

		for _, tag := range facts.VerificationTags {
			bindings := tag.Bindings
			if bindings == nil {
				bindings = map[string]string{}
			}
			input.VerificationTags = append(input.VerificationTags, policy.VerificationTag{
				ID:       tag.ID,
				Scope:    tag.Scope,
				Bindings: bindings,
				File:     facts.File,
				Line:     tag.Line,
				Raw:      tag.Raw,
				InArch:   tag.InArch,
			})
		}

		for _, terr := range facts.VerificationTagErrors {
			input.VerificationTagErrors = append(input.VerificationTagErrors, policy.VerificationTagError{
				File:    facts.File,
				Line:    terr.Line,
				Raw:     terr.Raw,
				Message: terr.Message,
				InArch:  terr.InArch,
			})
		}

		// Type system: Types
		for _, t := range facts.Types {
			// Convert enum literals (ensure not nil)
			enumLits := t.EnumLiterals
			if enumLits == nil {
				enumLits = []string{}
			}
			// Convert record fields (ensure not nil)
			var fields []policy.RecordField
			for _, f := range t.Fields {
				fields = append(fields, policy.RecordField{
					Name: f.Name,
					Type: f.Type,
					Line: f.Line,
				})
			}
			if fields == nil {
				fields = []policy.RecordField{}
			}
			// Convert index types (ensure not nil)
			indexTypes := t.IndexTypes
			if indexTypes == nil {
				indexTypes = []string{}
			}
			input.Types = append(input.Types, policy.TypeDeclaration{
				Name:          t.Name,
				Kind:          t.Kind,
				File:          facts.File,
				Line:          t.Line,
				InPackage:     t.InPackage,
				InArch:        t.InArch,
				EnumLiterals:  enumLits,
				Fields:        fields,
				ElementType:   t.ElementType,
				IndexTypes:    indexTypes,
				Unconstrained: t.Unconstrained,
				BaseUnit:      t.BaseUnit,
				RangeLow:      t.RangeLow,
				RangeHigh:     t.RangeHigh,
				RangeDir:      t.RangeDir,
			})
		}

		// Type system: Subtypes
		for _, st := range facts.Subtypes {
			input.Subtypes = append(input.Subtypes, policy.SubtypeDeclaration{
				Name:       st.Name,
				BaseType:   st.BaseType,
				Constraint: st.Constraint,
				Resolution: st.Resolution,
				File:       facts.File,
				Line:       st.Line,
				InPackage:  st.InPackage,
				InArch:     st.InArch,
			})
		}

		// Type system: Functions
		for _, fn := range facts.Functions {
			// Convert parameters (ensure not nil)
			var params []policy.SubprogramParameter
			for _, p := range fn.Parameters {
				params = append(params, policy.SubprogramParameter{
					Name:      p.Name,
					Direction: p.Direction,
					Type:      p.Type,
					Class:     p.Class,
					Default:   p.Default,
					Line:      p.Line,
				})
			}
			if params == nil {
				params = []policy.SubprogramParameter{}
			}
			input.Functions = append(input.Functions, policy.FunctionDeclaration{
				Name:       fn.Name,
				ReturnType: fn.ReturnType,
				Parameters: params,
				IsPure:     fn.IsPure,
				HasBody:    fn.HasBody,
				File:       facts.File,
				Line:       fn.Line,
				InPackage:  fn.InPackage,
				InArch:     fn.InArch,
			})
		}

		// Type system: Procedures
		for _, pr := range facts.Procedures {
			// Convert parameters (ensure not nil)
			var params []policy.SubprogramParameter
			for _, p := range pr.Parameters {
				params = append(params, policy.SubprogramParameter{
					Name:      p.Name,
					Direction: p.Direction,
					Type:      p.Type,
					Class:     p.Class,
					Default:   p.Default,
					Line:      p.Line,
				})
			}
			if params == nil {
				params = []policy.SubprogramParameter{}
			}
			input.Procedures = append(input.Procedures, policy.ProcedureDeclaration{
				Name:       pr.Name,
				Parameters: params,
				HasBody:    pr.HasBody,
				File:       facts.File,
				Line:       pr.Line,
				InPackage:  pr.InPackage,
				InArch:     pr.InArch,
			})
		}

		// Type system: Constants
		for _, c := range facts.ConstantDecls {
			input.ConstantDecls = append(input.ConstantDecls, policy.ConstantDeclaration{
				Name:      c.Name,
				Type:      c.Type,
				Value:     c.Value,
				File:      facts.File,
				Line:      c.Line,
				InPackage: c.InPackage,
				InArch:    c.InArch,
			})
		}

		// Type system info (LEGACY): collect enum literals and constants for filtering
		input.EnumLiterals = append(input.EnumLiterals, facts.EnumLiterals...)
		input.Constants = append(input.Constants, facts.Constants...)
	}

	// Add symbols
	for name, sym := range idx.Symbols.All() {
		input.Symbols = append(input.Symbols, policy.Symbol{
			Name: name,
			Kind: sym.Kind,
			File: sym.File,
			Line: sym.Line,
		})
	}

	idx.populateScopesDefsUses(&input)

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
	fmt.Printf("  [%d/%d] %s (%s, %s)\n", *progress, total, facts.File, status, formatDuration(duration))
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
	lines := []string{
		fmt.Sprintf("facts: entities=%d packages=%d arch=%d signals=%d ports=%d processes=%d instances=%d generates=%d deps=%d",
			len(facts.Entities), len(facts.Packages), len(facts.Architectures), len(facts.Signals), len(facts.Ports),
			len(facts.Processes), len(facts.Instances), len(facts.Generates), len(facts.Dependencies)),
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

func isAlpha(b byte) bool {
	return (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z')
}
