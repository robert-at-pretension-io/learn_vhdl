package indexer

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/robert-at-pretension-io/vhdl-lint/internal/config"
	"github.com/robert-at-pretension-io/vhdl-lint/internal/extractor"
)

type proMapping struct {
	fileLibraries     map[string]string
	generatedPackages map[string]map[string]bool
	libraries         map[string]bool
	entrypoints       []string
}

type proMappingStats struct {
	files     int
	libraries int
	packages  int
}

type proEnv struct {
	vars map[string]string
}

func loadProLibraryMapping(rootPath string, cfg *config.Config) (*proMapping, error) {
	rootDir := rootPath
	if info, err := os.Stat(rootPath); err == nil && !info.IsDir() {
		rootDir = filepath.Dir(rootPath)
	}
	absRoot, err := filepath.Abs(rootDir)
	if err != nil {
		absRoot = filepath.Clean(rootDir)
	}

	mapping := &proMapping{
		fileLibraries:     make(map[string]string),
		generatedPackages: make(map[string]map[string]bool),
		libraries:         make(map[string]bool),
	}

	env := defaultProEnv(cfg)
	entrypoints := detectProEntrypoints(rootPath, absRoot)
	if len(entrypoints) > 0 {
		mapping.entrypoints = entrypoints
	}

	var errs []error
	if len(entrypoints) > 0 {
		for _, ep := range entrypoints {
			lib := ""
			stack := make(map[string]bool)
			if err := applyProFileWithContext(ep, mapping, &lib, stack, env.clone()); err != nil {
				errs = append(errs, fmt.Errorf("parse %s: %w", ep, err))
			}
		}
	} else {
		walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				errs = append(errs, err)
				return nil
			}
			if d.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(path), ".pro") {
				return nil
			}
			lib := ""
			stack := make(map[string]bool)
			if err := applyProFileWithContext(path, mapping, &lib, stack, env.clone()); err != nil {
				errs = append(errs, fmt.Errorf("parse %s: %w", path, err))
			}
			return nil
		})
		if walkErr != nil {
			errs = append(errs, walkErr)
		}
	}
	if len(mapping.fileLibraries) == 0 && len(mapping.generatedPackages) == 0 {
		if len(errs) > 0 {
			return nil, joinProErrors(errs)
		}
		return nil, nil
	}
	if len(errs) > 0 {
		return mapping, joinProErrors(errs)
	}
	return mapping, nil
}

func applyProFileWithContext(path string, mapping *proMapping, currentLib *string, stack map[string]bool, env *proEnv) error {
	absPath, err := filepath.Abs(path)
	if err == nil {
		path = absPath
	}
	if stack[path] {
		return nil
	}
	stack[path] = true
	defer delete(stack, path)

	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	scriptDir := filepath.Dir(path)
	cwd := scriptDir
	if env != nil {
		if v := strings.TrimSpace(env.vars["CurrentWorkingDirectory"]); v != "" {
			cwd = resolveProDir(scriptDir, v)
		}
	}

	var condStack []proCondFrame
	isActive := func() bool {
		if len(condStack) == 0 {
			return true
		}
		return condStack[len(condStack)-1].active
	}

	processLine := func(line string) error {
		if line == "" {
			return nil
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			return nil
		}
		cmd := strings.ToLower(fields[0])
		switch cmd {
		case "library":
			if len(fields) > 1 {
				lib := cleanProToken(fields[1])
				if lib != "" {
					*currentLib = lib
					mapping.libraries[strings.ToLower(lib)] = true
				}
			}
		case "include", "build":
			if len(fields) < 2 {
				return nil
			}
			target := expandProToken(fields[1], env, cwd)
			if target == "" {
				return nil
			}
			if handled, err := handleProIncludeDir(target, cwd, scriptDir, path, mapping, currentLib, stack, env); err != nil {
				return err
			} else if handled {
				return nil
			}
			resolved, err := resolveProIncludeWithFallback(cwd, scriptDir, target)
			if err != nil {
				return err
			}
			if err := applyProFileWithContext(resolved, mapping, currentLib, stack, env.clone()); err != nil {
				return err
			}
		case "changeworkingdirectory":
			if len(fields) < 2 {
				return nil
			}
			target := expandProToken(fields[1], env, cwd)
			if target == "" {
				return nil
			}
			cwd = resolveProDir(cwd, target)
			if env != nil {
				env.vars["CurrentWorkingDirectory"] = cwd
			}
		case "set":
			name, value, ok := parseProSetLine(line)
			if !ok {
				return nil
			}
			normalized := normalizeProVarName(name)
			if normalized != "" && env != nil {
				env.vars[normalized] = substituteProVars(value, env, cwd)
			}
			if strings.EqualFold(normalized, "CurrentWorkingDirectory") {
				if next, ok := evalProWorkingDir(value, cwd, scriptDir, env); ok {
					cwd = next
					if env != nil {
						env.vars["CurrentWorkingDirectory"] = cwd
					}
				}
			}
		default:
			if *currentLib == "" {
				return nil
			}
			if pkg := extractCreatePkg(fields); pkg != "" {
				addGeneratedPackage(mapping, *currentLib, pkg)
			}
			for _, token := range fields[1:] {
				file := expandProToken(token, env, cwd)
				if !looksLikeVHDLFile(file) {
					continue
				}
				path := resolveProDir(cwd, file)
				if existing, ok := mapping.fileLibraries[path]; ok {
					if strings.EqualFold(existing, *currentLib) {
						continue
					}
					continue
				}
				mapping.fileLibraries[path] = *currentLib
			}
		}
		return nil
	}

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if idx := strings.Index(trimmed, "#"); idx >= 0 {
			trimmed = strings.TrimSpace(trimmed[:idx])
		}
		if trimmed == "" {
			continue
		}
		if handled, err := handleProConditional(trimmed, env, cwd, &condStack, processLine); handled {
			if err != nil {
				return err
			}
			continue
		}
		if !isActive() {
			continue
		}
		if err := processLine(trimmed); err != nil {
			return err
		}
	}
	return scanner.Err()
}

func applyProLibraryMapping(
	idx *Indexer,
	mapping *proMapping,
	fileSet map[string]bool,
	files *[]string,
) proMappingStats {
	stats := proMappingStats{}
	if mapping == nil {
		return stats
	}
	for file, lib := range mapping.fileLibraries {
		if !fileSet[file] {
			fileSet[file] = true
			if files != nil {
				*files = append(*files, file)
			}
		}
		info := idx.FileLibraries[file]
		info.LibraryName = lib
		idx.FileLibraries[file] = info
		stats.files++
	}
	stats.libraries = len(mapping.libraries)
	for _, pkgs := range mapping.generatedPackages {
		stats.packages += len(pkgs)
	}
	return stats
}

func injectGeneratedPackagesStreaming(
	idx *Indexer,
	mapping *proMapping,
	store *factsStore,
) (int, error) {
	if mapping == nil {
		return 0, nil
	}
	added := 0
	for lib, pkgs := range mapping.generatedPackages {
		for pkg := range pkgs {
			symName := fmt.Sprintf("%s.%s", strings.ToLower(lib), strings.ToLower(pkg))
			if idx.Symbols.Has(symName) {
				continue
			}
			genFile := fmt.Sprintf("<generated>/%s/%s.vhd", strings.ToLower(lib), pkg)
			facts := extractor.FileFacts{
				File: genFile,
				Packages: []extractor.Package{
					{Name: pkg, Line: 1},
				},
			}
			idx.FileLibraries[genFile] = config.FileLibraryInfo{
				LibraryName: lib,
			}
			if err := store.Put(genFile, facts); err != nil {
				return added, err
			}
			idx.registerSymbolsForFacts(facts, genFile)
			added++
		}
	}
	return added, nil
}

func extractCreatePkg(fields []string) string {
	for i, token := range fields {
		clean := cleanProToken(token)
		if strings.EqualFold(clean, "CreateTestCaseCommonPkg") && i+1 < len(fields) {
			return cleanProToken(fields[i+1])
		}
	}
	return ""
}

func addGeneratedPackage(mapping *proMapping, lib, pkg string) {
	if mapping.generatedPackages[lib] == nil {
		mapping.generatedPackages[lib] = make(map[string]bool)
	}
	mapping.generatedPackages[lib][pkg] = true
}

func cleanProToken(token string) string {
	clean := strings.TrimSpace(token)
	clean = strings.Trim(clean, "[]")
	clean = strings.TrimSuffix(clean, ";")
	clean = strings.TrimSuffix(clean, ")")
	clean = strings.TrimSuffix(clean, "]")
	return strings.TrimSpace(clean)
}

func looksLikeVHDLFile(token string) bool {
	lower := strings.ToLower(token)
	return strings.HasSuffix(lower, ".vhd") || strings.HasSuffix(lower, ".vhdl")
}

func joinProErrors(errs []error) error {
	if len(errs) == 0 {
		return nil
	}
	if len(errs) == 1 {
		return errs[0]
	}
	var parts []string
	for _, err := range errs {
		if err != nil {
			parts = append(parts, err.Error())
		}
	}
	if len(parts) == 0 {
		return nil
	}
	return fmt.Errorf("multiple .pro parse errors: %s", strings.Join(parts, "; "))
}

func resolveProInclude(baseDir, target string) (string, error) {
	path := target
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, target)
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}

	if strings.EqualFold(filepath.Ext(path), ".pro") {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}

	if info, err := os.Stat(path); err == nil && info.IsDir() {
		build := filepath.Join(path, "build.pro")
		if info, err := os.Stat(build); err == nil && !info.IsDir() {
			return build, nil
		}
		testbench := filepath.Join(path, "testbench.pro")
		if info, err := os.Stat(testbench); err == nil && !info.IsDir() {
			return testbench, nil
		}
		base := filepath.Base(path)
		if base != "" {
			named := filepath.Join(path, base+".pro")
			if info, err := os.Stat(named); err == nil && !info.IsDir() {
				return named, nil
			}
		}
		entries, err := os.ReadDir(path)
		if err != nil {
			return "", err
		}
		var candidates []string
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.EqualFold(filepath.Ext(name), ".pro") {
				candidates = append(candidates, filepath.Join(path, name))
			}
		}
		if len(candidates) == 1 {
			return candidates[0], nil
		}
		if len(candidates) > 1 {
			return "", fmt.Errorf("include %q has multiple .pro files; add explicit filename", target)
		}
		return "", fmt.Errorf("include %q has no .pro files", target)
	}

	if !strings.EqualFold(filepath.Ext(path), ".pro") {
		withExt := path + ".pro"
		if info, err := os.Stat(withExt); err == nil && !info.IsDir() {
			return withExt, nil
		}
	}

	return "", fmt.Errorf("include %q not found", target)
}

func resolveProIncludeWithFallback(baseDir, scriptDir, target string) (string, error) {
	if resolved, err := resolveProInclude(baseDir, target); err == nil {
		return resolved, nil
	}
	if scriptDir != "" && !samePath(baseDir, scriptDir) {
		return resolveProInclude(scriptDir, target)
	}
	return resolveProInclude(baseDir, target)
}

func handleProIncludeDir(target, cwd, scriptDir, scriptPath string, mapping *proMapping, currentLib *string, stack map[string]bool, env *proEnv) (bool, error) {
	for _, base := range []string{cwd, scriptDir} {
		if base == "" {
			continue
		}
		resolved := resolveProDir(base, target)
		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			continue
		}
		proPath, hasPro, err := selectProFileFromDir(resolved, filepath.Base(scriptPath))
		if err != nil {
			return true, err
		}
		if hasPro {
			if err := applyProFileWithContext(proPath, mapping, currentLib, stack, env.clone()); err != nil {
				return true, err
			}
			return true, nil
		}
		if *currentLib == "" {
			return true, nil
		}
		if err := includeVHDLFilesInDir(resolved, mapping, *currentLib); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

func selectProFileFromDir(dir, contextFile string) (string, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", true, err
	}
	var candidates []string
	var build string
	var named string
	var contextMatch string
	dirBase := strings.TrimSuffix(filepath.Base(dir), filepath.Ext(filepath.Base(dir)))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.EqualFold(filepath.Ext(name), ".pro") {
			continue
		}
		path := filepath.Join(dir, name)
		candidates = append(candidates, path)
		if strings.EqualFold(name, contextFile) {
			contextMatch = path
		}
		if strings.EqualFold(name, "build.pro") {
			build = path
		}
		if strings.EqualFold(strings.TrimSuffix(name, filepath.Ext(name)), dirBase) {
			named = path
		}
	}
	if contextMatch != "" {
		return contextMatch, true, nil
	}
	if build != "" {
		return build, true, nil
	}
	if named != "" {
		return named, true, nil
	}
	if len(candidates) == 1 {
		return candidates[0], true, nil
	}
	if len(candidates) > 1 {
		return "", true, fmt.Errorf("include %q has multiple .pro files; add explicit filename", dir)
	}
	return "", false, nil
}

func includeVHDLFilesInDir(dir string, mapping *proMapping, lib string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !looksLikeVHDLFile(name) {
			continue
		}
		path := filepath.Join(dir, name)
		if existing, ok := mapping.fileLibraries[path]; ok {
			if strings.EqualFold(existing, lib) {
				continue
			}
			continue
		}
		mapping.fileLibraries[path] = lib
	}
	return nil
}

func samePath(a, b string) bool {
	if a == b {
		return true
	}
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false
	}
	return aa == bb
}

func (e *proEnv) clone() *proEnv {
	if e == nil {
		return &proEnv{vars: map[string]string{}}
	}
	out := make(map[string]string, len(e.vars))
	for k, v := range e.vars {
		out[k] = v
	}
	return &proEnv{vars: out}
}

func defaultProEnv(cfg *config.Config) *proEnv {
	vhdlVersion := "2019"
	if cfg != nil && cfg.Standard != "" {
		vhdlVersion = cfg.Standard
	}
	env := &proEnv{vars: map[string]string{}}
	env.vars["ToolName"] = envOr("VHDL_PRO_TOOL_NAME", "GHDL")
	env.vars["ToolVendor"] = envOr("VHDL_PRO_TOOL_VENDOR", env.vars["ToolName"])
	env.vars["ToolSupportsGenericPackages"] = envOr("VHDL_PRO_TOOL_SUPPORTS_GENERIC_PACKAGES", "true")
	env.vars["ToolSupportsDeferredConstants"] = envOr("VHDL_PRO_TOOL_SUPPORTS_DEFERRED_CONSTANTS", "true")
	env.vars["Support2019FilePath"] = envOr("VHDL_PRO_SUPPORT_2019_FILE_PATH", "true")
	env.vars["VhdlVersion"] = envOr("VHDL_PRO_VHDL_VERSION", vhdlVersion)
	env.vars["ClockResetVersion"] = envOr("VHDL_PRO_CLOCK_RESET_VERSION", "9999.99")
	env.vars["FunctionalCoverageIntegratedInSimulator"] = envOr("VHDL_PRO_FUNCTIONAL_COVERAGE", "default")
	return env
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func detectProEntrypoints(rootPath, rootDir string) []string {
	if info, err := os.Stat(rootPath); err == nil && !info.IsDir() && strings.EqualFold(filepath.Ext(rootPath), ".pro") {
		if abs, err := filepath.Abs(rootPath); err == nil {
			return []string{abs}
		}
		return []string{rootPath}
	}
	var entrypoints []string
	add := func(path string) {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if abs, err := filepath.Abs(path); err == nil {
				entrypoints = append(entrypoints, abs)
			} else {
				entrypoints = append(entrypoints, path)
			}
		}
	}
	add(filepath.Join(rootDir, "OsvvmLibraries.pro"))
	add(filepath.Join(rootDir, "RunAllTests.pro"))
	if len(entrypoints) > 0 {
		return entrypoints
	}
	add(filepath.Join(rootDir, "RunDemoTests.pro"))
	if len(entrypoints) > 0 {
		return entrypoints
	}
	add(filepath.Join(rootDir, "build.pro"))
	if len(entrypoints) > 0 {
		return entrypoints
	}
	add(filepath.Join(rootDir, "testbench.pro"))
	return entrypoints
}

type proCondFrame struct {
	parentActive bool
	active       bool
	branchTaken  bool
}

func handleProConditional(
	line string,
	env *proEnv,
	cwd string,
	stack *[]proCondFrame,
	processLine func(string) error,
) (bool, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false, nil
	}
	lower := strings.ToLower(trimmed)

	if cond, body, inline, ok := parseProIfLine(trimmed); ok {
		parentActive := true
		if len(*stack) > 0 {
			parentActive = (*stack)[len(*stack)-1].active
		}
		condVal := false
		if parentActive {
			condVal = evalProCondition(cond, env, cwd)
		}
		frame := proCondFrame{
			parentActive: parentActive,
			active:       parentActive && condVal,
			branchTaken:  condVal,
		}
		*stack = append(*stack, frame)
		if inline {
			if frame.active {
				if err := processLine(body); err != nil {
					return true, err
				}
			}
			*stack = (*stack)[:len(*stack)-1]
		}
		return true, nil
	}

	if cond, body, inline, ok := parseProElseIfLine(trimmed); ok {
		if len(*stack) == 0 {
			return true, nil
		}
		idx := len(*stack) - 1
		frame := (*stack)[idx]
		if !frame.parentActive {
			frame.active = false
		} else if frame.branchTaken {
			frame.active = false
		} else {
			condVal := evalProCondition(cond, env, cwd)
			frame.active = frame.parentActive && condVal
			if condVal {
				frame.branchTaken = true
			}
		}
		(*stack)[idx] = frame
		if inline {
			if frame.active {
				if err := processLine(body); err != nil {
					return true, err
				}
			}
			*stack = (*stack)[:len(*stack)-1]
		}
		return true, nil
	}

	if body, inline, ok := parseProElseLine(trimmed); ok {
		if len(*stack) == 0 {
			return true, nil
		}
		idx := len(*stack) - 1
		frame := (*stack)[idx]
		if !frame.parentActive || frame.branchTaken {
			frame.active = false
		} else {
			frame.active = true
			frame.branchTaken = true
		}
		(*stack)[idx] = frame
		if inline {
			if frame.active {
				if err := processLine(body); err != nil {
					return true, err
				}
			}
			*stack = (*stack)[:len(*stack)-1]
		}
		return true, nil
	}

	if isProEndLine(lower) {
		if len(*stack) > 0 {
			*stack = (*stack)[:len(*stack)-1]
		}
		return true, nil
	}

	return false, nil
}

func parseProIfLine(line string) (string, string, bool, bool) {
	lower := strings.ToLower(strings.TrimSpace(line))
	if !strings.HasPrefix(lower, "if") {
		return "", "", false, false
	}
	cond, rest, ok := extractProBraceSection(line)
	if !ok {
		return "", "", false, false
	}
	body, _, okBody := extractProBraceSection(rest)
	if okBody {
		return cond, strings.TrimSpace(body), true, true
	}
	return cond, "", false, true
}

func parseProElseIfLine(line string) (string, string, bool, bool) {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "}")
	trimmed = strings.TrimSpace(trimmed)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "elseif") {
		return "", "", false, false
	}
	cond, rest, ok := extractProBraceSection(trimmed)
	if !ok {
		return "", "", false, false
	}
	body, _, okBody := extractProBraceSection(rest)
	if okBody {
		return cond, strings.TrimSpace(body), true, true
	}
	return cond, "", false, true
}

func parseProElseLine(line string) (string, bool, bool) {
	trimmed := strings.TrimSpace(line)
	trimmed = strings.TrimPrefix(trimmed, "}")
	trimmed = strings.TrimSpace(trimmed)
	lower := strings.ToLower(trimmed)
	if !strings.HasPrefix(lower, "else") {
		return "", false, false
	}
	body, _, okBody := extractProBraceSection(trimmed)
	if okBody {
		return strings.TrimSpace(body), true, true
	}
	return "", false, true
}

func isProEndLine(lower string) bool {
	trimmed := strings.TrimSpace(lower)
	return trimmed == "}"
}

func extractProBraceSection(line string) (string, string, bool) {
	start := strings.Index(line, "{")
	if start == -1 {
		return "", "", false
	}
	depth := 0
	for i := start; i < len(line); i++ {
		switch line[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				content := line[start+1 : i]
				rest := strings.TrimSpace(line[i+1:])
				return strings.TrimSpace(content), rest, true
			}
		}
	}
	return "", "", false
}

func expandProToken(token string, env *proEnv, cwd string) string {
	clean := cleanProToken(token)
	if clean == "" {
		return ""
	}
	clean = substituteProVars(clean, env, cwd)
	clean = strings.Trim(clean, "\"")
	return strings.TrimSpace(clean)
}

func substituteProVars(raw string, env *proEnv, cwd string) string {
	if env == nil {
		return raw
	}
	out := raw
	for {
		start := strings.Index(out, "${")
		if start == -1 {
			break
		}
		end := strings.Index(out[start:], "}")
		if end == -1 {
			break
		}
		end = start + end
		name := normalizeProVarName(out[start+2 : end])
		repl := resolveProVarValue(name, env, cwd)
		out = out[:start] + repl + out[end+1:]
	}
	for i := 0; i < len(out); i++ {
		if out[i] != '$' {
			continue
		}
		j := i + 1
		for j < len(out) && (out[j] == ':' || out[j] == '_' || out[j] == '-' || (out[j] >= 'A' && out[j] <= 'Z') || (out[j] >= 'a' && out[j] <= 'z') || (out[j] >= '0' && out[j] <= '9')) {
			j++
		}
		name := normalizeProVarName(out[i+1 : j])
		repl := resolveProVarValue(name, env, cwd)
		out = out[:i] + repl + out[j:]
		i += len(repl)
	}
	return out
}

func resolveProVarValue(name string, env *proEnv, cwd string) string {
	if strings.EqualFold(name, "CurrentWorkingDirectory") {
		if cwd != "" {
			return cwd
		}
	}
	if env == nil {
		return "$" + name
	}
	if v, ok := env.vars[name]; ok {
		return v
	}
	return "$" + name
}

func normalizeProVarName(name string) string {
	clean := strings.TrimSpace(name)
	clean = strings.TrimPrefix(clean, "$")
	clean = strings.Trim(clean, "{}")
	clean = strings.TrimPrefix(clean, "::osvvm::")
	return strings.TrimSpace(clean)
}

func parseProSetLine(line string) (string, string, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(strings.ToLower(trimmed), "set ") {
		return "", "", false
	}
	parts := strings.Fields(trimmed)
	if len(parts) < 3 {
		return "", "", false
	}
	name := parts[1]
	idx := strings.Index(trimmed, name)
	if idx == -1 {
		return "", "", false
	}
	value := strings.TrimSpace(trimmed[idx+len(name):])
	if value == "" {
		return "", "", false
	}
	return name, value, true
}

func evalProWorkingDir(value, cwd, scriptDir string, env *proEnv) (string, bool) {
	clean := strings.TrimSpace(value)
	clean = strings.Trim(clean, "[]")
	clean = substituteProVars(clean, env, cwd)
	clean = strings.TrimSpace(clean)
	if clean == "" {
		return "", false
	}
	lower := strings.ToLower(clean)
	if strings.HasPrefix(lower, "file join") {
		parts := strings.Fields(clean)
		if len(parts) >= 3 {
			base := parts[2]
			joinParts := []string{base}
			if len(parts) > 3 {
				joinParts = append(joinParts, parts[3:]...)
			}
			return resolveProDir(scriptDir, filepath.Join(joinParts...)), true
		}
	}
	clean = strings.Trim(clean, "\"")
	return resolveProDir(scriptDir, clean), true
}

func resolveProDir(baseDir, value string) string {
	if value == "" {
		return baseDir
	}
	path := value
	if !filepath.IsAbs(path) {
		path = filepath.Join(baseDir, path)
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return path
}

func evalProCondition(expr string, env *proEnv, cwd string) bool {
	trimmed := strings.TrimSpace(expr)
	if trimmed == "" {
		return false
	}
	trimmed = replaceProBracketExpr(trimmed, env, cwd)
	trimmed = substituteProVars(trimmed, env, cwd)
	trimmed = strings.TrimSpace(trimmed)
	return evalProBoolExpr(trimmed)
}

func replaceProBracketExpr(expr string, env *proEnv, cwd string) string {
	out := expr
	for {
		start := strings.Index(out, "[")
		if start == -1 {
			break
		}
		end := strings.Index(out[start:], "]")
		if end == -1 {
			break
		}
		end = start + end
		content := strings.TrimSpace(out[start+1 : end])
		repl := ""
		lower := strings.ToLower(content)
		switch {
		case strings.HasPrefix(lower, "directoryexists"):
			path := strings.TrimSpace(content[len("directoryexists"):])
			path = substituteProVars(path, env, cwd)
			repl = boolString(isDir(resolveProDir(cwd, path)))
		case strings.HasPrefix(lower, "fileexists"):
			path := strings.TrimSpace(content[len("fileexists"):])
			path = substituteProVars(path, env, cwd)
			repl = boolString(isFile(resolveProDir(cwd, path)))
		case strings.HasPrefix(lower, "string compare"):
			parts := strings.Fields(content)
			if len(parts) >= 4 {
				left := substituteProVars(parts[2], env, cwd)
				right := substituteProVars(parts[3], env, cwd)
				left = strings.Trim(left, "\"")
				right = strings.Trim(right, "\"")
				repl = fmt.Sprintf("%d", strings.Compare(left, right))
			}
		}
		out = out[:start] + repl + out[end+1:]
	}
	return out
}

func evalProBoolExpr(expr string) bool {
	expr = strings.TrimSpace(stripOuterParens(expr))
	if expr == "" {
		return false
	}
	if parts := splitTopLevel(expr, "||"); len(parts) > 1 {
		for _, part := range parts {
			if evalProBoolExpr(part) {
				return true
			}
		}
		return false
	}
	if parts := splitTopLevel(expr, "&&"); len(parts) > 1 {
		for _, part := range parts {
			if !evalProBoolExpr(part) {
				return false
			}
		}
		return true
	}
	expr = strings.TrimSpace(expr)
	if strings.HasPrefix(expr, "!") {
		return !evalProBoolExpr(strings.TrimSpace(expr[1:]))
	}
	if val, ok := parseProBool(expr); ok {
		return val
	}
	if lhs, op, rhs, ok := splitProComparison(expr); ok {
		return compareProValues(lhs, op, rhs)
	}
	return false
}

func parseProBool(expr string) (bool, bool) {
	lower := strings.ToLower(strings.TrimSpace(expr))
	switch lower {
	case "true":
		return true, true
	case "false":
		return false, true
	}
	return false, false
}

func splitProComparison(expr string) (string, string, string, bool) {
	ops := []string{"==", "!=", ">=", "<=", ">", "<"}
	for _, op := range ops {
		if parts := splitTopLevel(expr, op); len(parts) == 2 {
			return strings.TrimSpace(parts[0]), op, strings.TrimSpace(parts[1]), true
		}
	}
	if idx := strings.Index(strings.ToLower(expr), " eq "); idx != -1 {
		return strings.TrimSpace(expr[:idx]), "eq", strings.TrimSpace(expr[idx+4:]), true
	}
	if idx := strings.Index(strings.ToLower(expr), " ne "); idx != -1 {
		return strings.TrimSpace(expr[:idx]), "ne", strings.TrimSpace(expr[idx+4:]), true
	}
	return "", "", "", false
}

func compareProValues(lhs, op, rhs string) bool {
	lhs = strings.Trim(lhs, "\"")
	rhs = strings.Trim(rhs, "\"")
	lhsNum, lhsOK := parseProNumber(lhs)
	rhsNum, rhsOK := parseProNumber(rhs)
	if lhsOK && rhsOK {
		switch op {
		case "==":
			return lhsNum == rhsNum
		case "!=":
			return lhsNum != rhsNum
		case ">":
			return lhsNum > rhsNum
		case "<":
			return lhsNum < rhsNum
		case ">=":
			return lhsNum >= rhsNum
		case "<=":
			return lhsNum <= rhsNum
		}
	}
	switch op {
	case "eq", "==":
		return strings.EqualFold(lhs, rhs)
	case "ne", "!=":
		return !strings.EqualFold(lhs, rhs)
	case ">":
		return strings.Compare(lhs, rhs) > 0
	case "<":
		return strings.Compare(lhs, rhs) < 0
	case ">=":
		return strings.Compare(lhs, rhs) >= 0
	case "<=":
		return strings.Compare(lhs, rhs) <= 0
	}
	return false
}

func parseProNumber(raw string) (float64, bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false
	}
	n, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func splitTopLevel(expr, sep string) []string {
	var parts []string
	start := 0
	depth := 0
	inString := false
	for i := 0; i < len(expr); i++ {
		ch := expr[i]
		switch ch {
		case '"':
			inString = !inString
		case '(':
			if !inString {
				depth++
			}
		case ')':
			if !inString && depth > 0 {
				depth--
			}
		}
		if depth == 0 && !inString && strings.HasPrefix(expr[i:], sep) {
			part := strings.TrimSpace(expr[start:i])
			parts = append(parts, part)
			i += len(sep) - 1
			start = i + 1
		}
	}
	if start <= len(expr) {
		part := strings.TrimSpace(expr[start:])
		if part != "" {
			parts = append(parts, part)
		}
	}
	if len(parts) == 0 {
		return []string{expr}
	}
	return parts
}

func stripOuterParens(expr string) string {
	trimmed := strings.TrimSpace(expr)
	for strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
		inner := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		if inner == "" {
			break
		}
		trimmed = inner
	}
	return trimmed
}

func boolString(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func isFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}
