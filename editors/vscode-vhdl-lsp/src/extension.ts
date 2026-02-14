import * as vscode from 'vscode';
import * as fs from 'node:fs';
import * as path from 'node:path';
import {
  LanguageClient,
  LanguageClientOptions,
  ServerOptions,
  State,
  Trace
} from 'vscode-languageclient/node';

let client: LanguageClient | undefined;
let outputChannel: vscode.OutputChannel | undefined;
let statusBarItem: vscode.StatusBarItem | undefined;
let stateChangeDisposable: vscode.Disposable | undefined;
let diagnosticsDisposable: vscode.Disposable | undefined;
let visibleEditorsDisposable: vscode.Disposable | undefined;
let activeEditorDisposable: vscode.Disposable | undefined;
let currentClientState: State = State.Stopped;
let startupNotificationShown = false;
let problemsAutoRevealed = false;
let inlineErrorDecoration: vscode.TextEditorDecorationType | undefined;
let inlineWarningDecoration: vscode.TextEditorDecorationType | undefined;
let inlineInfoDecoration: vscode.TextEditorDecorationType | undefined;
let inlineHintDecoration: vscode.TextEditorDecorationType | undefined;
let ruleReferenceById = new Map<string, RuleReference>();
let ruleReferenceDocPath: string | undefined;

export async function activate(context: vscode.ExtensionContext): Promise<void> {
  outputChannel = vscode.window.createOutputChannel('VHDL LSP');
  context.subscriptions.push(outputChannel);

  diagnosticsDisposable = vscode.languages.onDidChangeDiagnostics(() => {
    updateStatusBar();
    updateInlineDiagnostics();
  });
  context.subscriptions.push(diagnosticsDisposable);

  visibleEditorsDisposable = vscode.window.onDidChangeVisibleTextEditors(() => {
    updateInlineDiagnostics();
  });
  context.subscriptions.push(visibleEditorsDisposable);

  activeEditorDisposable = vscode.window.onDidChangeActiveTextEditor(() => {
    updateInlineDiagnostics();
  });
  context.subscriptions.push(activeEditorDisposable);
  initializeRuleReferenceIndex(context);
  registerRichDiagnosticHover(context);

  context.subscriptions.push(
    vscode.commands.registerCommand('vhdlLsp.restartServer', async () => {
      await restartClient();
      void vscode.window.showInformationMessage('VHDL LSP server restarted');
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('vhdlLsp.showOutput', () => {
      outputChannel?.show(true);
    })
  );

  context.subscriptions.push(
    vscode.workspace.onDidChangeConfiguration(async event => {
      if (shouldRestartServer(event)) {
        await restartClient();
      }
      if (event.affectsConfiguration('vhdlLsp.trace.server')) {
        applyClientTraceSetting();
      }
      if (
        event.affectsConfiguration('vhdlLsp.ui') ||
        event.affectsConfiguration('vhdlLsp.trace.server') ||
        event.affectsConfiguration('problems.autoReveal')
      ) {
        updateStatusBar();
        updateInlineDiagnostics();
      }
      if (event.affectsConfiguration('vhdlLsp.ui.richDiagnostics')) {
        updateInlineDiagnostics();
      }
    })
  );

  ensureStatusBar(context);
  ensureInlineDecorations(context);
  updateStatusBar();
  updateInlineDiagnostics();

  await startClient();
}

export async function deactivate(): Promise<void> {
  await stopClient();
}

async function restartClient(): Promise<void> {
  await stopClient();
  await startClient();
}

async function startClient(): Promise<void> {
  const config = vscode.workspace.getConfiguration('vhdlLsp');
  const command = config.get<string>('server.path', 'vhdl-lsp');
  const args = sanitizeArgs(config.get<unknown>('server.args'));
  const cwd = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;

  const env: Record<string, string> = {
    ...coerceEnv(process.env)
  };

  const extraEnv = coerceEnv(config.get<unknown>('server.env', {}));
  for (const [key, value] of Object.entries(extraEnv)) {
    env[key] = value;
  }

  const lintPath = config.get<string>('lint.path', '').trim();
  if (lintPath !== '') {
    env.VHDL_LINT_BIN = lintPath;
  }

  const lintConfigPath = config.get<string>('lint.configPath', '').trim();
  if (lintConfigPath !== '') {
    env.VHDL_LINT_CONFIG = lintConfigPath;
  }

  const debounceMs = config.get<number>('server.debounceMs', 150);
  if (Number.isFinite(debounceMs) && debounceMs > 0) {
    env.VHDL_LSP_DEBOUNCE_MS = String(Math.floor(debounceMs));
  }

  const serverOptions: ServerOptions = {
    command,
    args,
    options: {
      cwd,
      env
    }
  };

  const clientOptions: LanguageClientOptions = {
    documentSelector: [
      { scheme: 'file', language: 'vhdl' },
      { scheme: 'untitled', language: 'vhdl' }
    ],
    outputChannel,
    traceOutputChannel: outputChannel
  };

  client = new LanguageClient('vhdl-lsp', 'VHDL LSP', serverOptions, clientOptions);
  installStateChangeHandler(client, command, lintPath);

  applyClientTraceSetting();

  currentClientState = State.Starting;
  startupNotificationShown = false;
  updateStatusBar();

  try {
    await client.start();
  } catch (error) {
    currentClientState = State.Stopped;
    updateStatusBar();
    void vscode.window.showErrorMessage(`Failed to start vhdl-lsp: ${String(error)}`);
    throw error;
  }
}

async function stopClient(): Promise<void> {
  if (stateChangeDisposable !== undefined) {
    stateChangeDisposable.dispose();
    stateChangeDisposable = undefined;
  }

  if (client === undefined) {
    currentClientState = State.Stopped;
    updateStatusBar();
    return;
  }

  const current = client;
  client = undefined;

  currentClientState = State.Stopped;
  updateStatusBar();
  await current.stop();
}

function ensureStatusBar(context: vscode.ExtensionContext): void {
  if (statusBarItem !== undefined) {
    return;
  }
  statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Left, 100);
  statusBarItem.name = 'VHDL LSP Status';
  context.subscriptions.push(statusBarItem);
}

function ensureInlineDecorations(context: vscode.ExtensionContext): void {
  if (inlineErrorDecoration === undefined) {
    inlineErrorDecoration = vscode.window.createTextEditorDecorationType({
      after: {
        color: new vscode.ThemeColor('editorError.foreground'),
        fontStyle: 'italic',
        margin: '0 0 0 1.5rem'
      }
    });
    context.subscriptions.push(inlineErrorDecoration);
  }

  if (inlineWarningDecoration === undefined) {
    inlineWarningDecoration = vscode.window.createTextEditorDecorationType({
      after: {
        color: new vscode.ThemeColor('editorWarning.foreground'),
        fontStyle: 'italic',
        margin: '0 0 0 1.5rem'
      }
    });
    context.subscriptions.push(inlineWarningDecoration);
  }

  if (inlineInfoDecoration === undefined) {
    inlineInfoDecoration = vscode.window.createTextEditorDecorationType({
      after: {
        color: new vscode.ThemeColor('editorInfo.foreground'),
        fontStyle: 'italic',
        margin: '0 0 0 1.5rem'
      }
    });
    context.subscriptions.push(inlineInfoDecoration);
  }

  if (inlineHintDecoration === undefined) {
    inlineHintDecoration = vscode.window.createTextEditorDecorationType({
      after: {
        color: new vscode.ThemeColor('descriptionForeground'),
        fontStyle: 'italic',
        margin: '0 0 0 1.5rem'
      }
    });
    context.subscriptions.push(inlineHintDecoration);
  }
}

function shouldRestartServer(event: vscode.ConfigurationChangeEvent): boolean {
  return (
    event.affectsConfiguration('vhdlLsp.server.path') ||
    event.affectsConfiguration('vhdlLsp.server.args') ||
    event.affectsConfiguration('vhdlLsp.server.env') ||
    event.affectsConfiguration('vhdlLsp.lint.path') ||
    event.affectsConfiguration('vhdlLsp.server.debounceMs')
  );
}

function applyClientTraceSetting(): void {
  if (client === undefined) {
    return;
  }
  const config = vscode.workspace.getConfiguration('vhdlLsp');
  const traceSetting = config.get<string>('trace.server', 'off');
  switch (traceSetting) {
    case 'messages':
      client.setTrace(Trace.Messages);
      break;
    case 'verbose':
      client.setTrace(Trace.Verbose);
      break;
    default:
      client.setTrace(Trace.Off);
      break;
  }
}

function installStateChangeHandler(lc: LanguageClient, command: string, lintPath: string): void {
  if (stateChangeDisposable !== undefined) {
    stateChangeDisposable.dispose();
    stateChangeDisposable = undefined;
  }

  stateChangeDisposable = lc.onDidChangeState(event => {
    currentClientState = event.newState;

    switch (event.newState) {
      case State.Starting:
        outputChannel?.appendLine('[vhdl-lsp] starting');
        break;
      case State.Running:
        outputChannel?.appendLine('[vhdl-lsp] running');
        maybeShowStartupNotification(command, lintPath);
        break;
      case State.Stopped:
        outputChannel?.appendLine('[vhdl-lsp] stopped');
        break;
    }

    updateStatusBar();
  });
}

function maybeShowStartupNotification(command: string, lintPath: string): void {
  const config = vscode.workspace.getConfiguration('vhdlLsp');
  const show = config.get<boolean>('ui.showStartupNotification', true);
  if (!show || startupNotificationShown) {
    return;
  }
  startupNotificationShown = true;

  const lintConfigPath = config.get<string>('lint.configPath', '').trim();
  const details: string[] = [command];
  if (lintPath !== '') {
    details.push(`lint: ${lintPath}`);
  }
  if (lintConfigPath !== '') {
    details.push(`config: ${lintConfigPath}`);
  }
  const msg = `VHDL LSP running (${details.join(', ')})`;
  void vscode.window
    .showInformationMessage(msg, 'Show Output', 'Show Problems')
    .then(selection => {
      if (selection === 'Show Output') {
        outputChannel?.show(true);
      } else if (selection === 'Show Problems') {
        void vscode.commands.executeCommand('workbench.actions.view.problems');
      }
    });
}

function updateStatusBar(): void {
  if (statusBarItem === undefined) {
    return;
  }

  const config = vscode.workspace.getConfiguration('vhdlLsp');
  const enabled = config.get<boolean>('ui.statusBar.enabled', true);
  if (!enabled) {
    statusBarItem.hide();
    return;
  }

  const counts = countVhdlDiagnostics();
  const hasIssues = counts.total > 0;

  statusBarItem.backgroundColor = undefined;
  statusBarItem.color = undefined;
  statusBarItem.command = 'vhdlLsp.showOutput';

  switch (currentClientState) {
    case State.Starting:
      statusBarItem.text = '$(sync~spin) VHDL LSP starting';
      statusBarItem.tooltip = 'vhdl-lsp is starting';
      break;
    case State.Running:
      if (hasIssues) {
        statusBarItem.text = `$(warning) VHDL E:${counts.errors} W:${counts.warnings} I:${counts.infos + counts.hints}`;
        statusBarItem.tooltip = 'VHDL diagnostics from vhdl-lsp. Click to open Problems.';
        statusBarItem.command = 'workbench.actions.view.problems';
        if (counts.errors > 0) {
          statusBarItem.backgroundColor = new vscode.ThemeColor('statusBarItem.errorBackground');
          statusBarItem.color = new vscode.ThemeColor('statusBarItem.errorForeground');
        } else {
          statusBarItem.backgroundColor = new vscode.ThemeColor('statusBarItem.warningBackground');
          statusBarItem.color = new vscode.ThemeColor('statusBarItem.warningForeground');
        }
      } else {
        statusBarItem.text = '$(check) VHDL LSP running';
        statusBarItem.tooltip = 'vhdl-lsp is running. No current VHDL diagnostics.';
      }
      maybeAutoRevealProblems(hasIssues);
      break;
    case State.Stopped:
    default:
      statusBarItem.text = '$(error) VHDL LSP stopped';
      statusBarItem.tooltip = 'vhdl-lsp is not running. Click to open output.';
      statusBarItem.backgroundColor = new vscode.ThemeColor('statusBarItem.errorBackground');
      statusBarItem.color = new vscode.ThemeColor('statusBarItem.errorForeground');
      break;
  }

  statusBarItem.show();
}

function maybeAutoRevealProblems(hasIssues: boolean): void {
  const config = vscode.workspace.getConfiguration('vhdlLsp');
  const autoReveal = config.get<boolean>('ui.autoRevealProblems', true);
  if (!autoReveal) {
    return;
  }

  if (hasIssues && !problemsAutoRevealed) {
    problemsAutoRevealed = true;
    void vscode.commands.executeCommand('workbench.actions.view.problems');
  } else if (!hasIssues) {
    problemsAutoRevealed = false;
  }
}

function initializeRuleReferenceIndex(context: vscode.ExtensionContext): void {
  refreshRuleReferenceIndex();
  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  if (workspaceRoot === undefined) {
    return;
  }
  const watcher = vscode.workspace.createFileSystemWatcher(new vscode.RelativePattern(workspaceRoot, 'docs/rules.md'));
  watcher.onDidCreate(() => refreshRuleReferenceIndex());
  watcher.onDidChange(() => refreshRuleReferenceIndex());
  watcher.onDidDelete(() => refreshRuleReferenceIndex());
  context.subscriptions.push(watcher);
}

function registerRichDiagnosticHover(context: vscode.ExtensionContext): void {
  const provider = vscode.languages.registerHoverProvider(
    [
      { scheme: 'file', language: 'vhdl' },
      { scheme: 'untitled', language: 'vhdl' }
    ],
    {
      provideHover(document, position) {
        const richConfig = vscode.workspace.getConfiguration('vhdlLsp.ui.richDiagnostics');
        const enabled = richConfig.get<boolean>('enabled', true);
        if (!enabled || !isVhdlDocument(document)) {
          return undefined;
        }

        const maxInHover = clampPositiveInt(richConfig.get<number>('maxInHover', 8), 8);
        const diagnostics = diagnosticsAtPosition(document.uri, position)
          .filter(isVhdlLintDiagnostic)
          .sort((a, b) => diagnosticDisplayPriority(a) - diagnosticDisplayPriority(b));

        if (diagnostics.length === 0) {
          return undefined;
        }

        const shown = diagnostics.slice(0, maxInHover);
        const hiddenCount = Math.max(0, diagnostics.length - shown.length);
        const markdown = buildRichDiagnosticsMarkdown(shown, hiddenCount);
        return new vscode.Hover(markdown);
      }
    }
  );
  context.subscriptions.push(provider);
}

function refreshRuleReferenceIndex(): void {
  ruleReferenceById = new Map();
  ruleReferenceDocPath = undefined;

  const workspaceRoot = vscode.workspace.workspaceFolders?.[0]?.uri.fsPath;
  if (workspaceRoot === undefined) {
    return;
  }

  const docsPath = path.join(workspaceRoot, 'docs', 'rules.md');
  if (!fs.existsSync(docsPath)) {
    return;
  }

  try {
    const markdown = fs.readFileSync(docsPath, 'utf8');
    ruleReferenceById = parseRuleReferenceMarkdown(markdown);
    ruleReferenceDocPath = docsPath;
  } catch (error) {
    outputChannel?.appendLine(`[vhdl-lsp] failed to read docs/rules.md: ${String(error)}`);
  }
}

function parseRuleReferenceMarkdown(markdown: string): Map<string, RuleReference> {
  const out = new Map<string, RuleReference>();
  const lines = markdown.split(/\r?\n/);
  let category = '';
  let index = 0;

  while (index < lines.length) {
    const line = lines[index];
    const categoryMatch = line.match(/^##\s+\d+\.\s+(.+)$/);
    if (categoryMatch !== null) {
      category = categoryMatch[1].trim();
      index++;
      continue;
    }

    const ruleMatch = line.match(/^###\s+`([a-z0-9_]+)`(?:\s+—\s+optional)?/i);
    if (ruleMatch === null) {
      index++;
      continue;
    }

    const ruleId = ruleMatch[1];
    const optional = /\boptional\b/i.test(line);
    let severity = '';
    let defaultState = '';

    let end = index + 1;
    while (end < lines.length) {
      const candidate = lines[end];
      if (/^###\s+`/.test(candidate) || /^##\s+/.test(candidate)) {
        break;
      }
      const tableMatch = candidate.trim().match(/^\|\s*(error|warning|info)\s*\|\s*(on|off)\s*\|$/i);
      if (tableMatch !== null) {
        severity = tableMatch[1].toLowerCase();
        defaultState = tableMatch[2].toLowerCase();
      }
      end++;
    }

    const bodyChunks: string[] = [];
    for (let i = index + 1; i < end; i++) {
      const raw = lines[i];
      const trimmed = raw.trim();
      if (trimmed === '') {
        bodyChunks.push(' ');
        continue;
      }
      if (trimmed.startsWith('|') && trimmed.endsWith('|')) {
        continue;
      }
      if (/^-{3,}$/.test(trimmed)) {
        continue;
      }
      bodyChunks.push(trimmed);
    }
    const body = bodyChunks.join(' ').replace(/\s+/g, ' ').trim();
    const summary = extractRuleSummary(body);

    out.set(ruleId, {
      ruleId,
      category,
      severity,
      defaultState,
      optional,
      summary
    });
    index = end;
  }

  return out;
}

function extractRuleSummary(text: string): string {
  const normalized = text.replace(/\s+/g, ' ').trim();
  if (normalized === '') {
    return '';
  }
  const sentences = normalized.match(/[^.!?]+[.!?]?/g) ?? [normalized];
  const summary = sentences.slice(0, 2).join(' ').trim();
  if (summary.length <= 360) {
    return summary;
  }
  return summary.slice(0, 357).trimEnd() + '...';
}

function buildRichDiagnosticsMarkdown(diagnostics: readonly vscode.Diagnostic[], hiddenCount: number): vscode.MarkdownString {
  const md = new vscode.MarkdownString();
  md.isTrusted = false;
  md.supportHtml = false;

  md.appendMarkdown('### VHDL Lint Guidance\n');
  md.appendText('Learner-focused explanation for diagnostics on this line.');
  md.appendMarkdown('\n');

  for (let i = 0; i < diagnostics.length; i++) {
    const diagnostic = diagnostics[i];
    const ruleId = diagnosticRuleId(diagnostic);
    const reference = ruleId !== undefined ? (ruleReferenceById.get(ruleId) ?? fallbackRuleReference(ruleId)) : undefined;

    md.appendMarkdown('\n---\n');
    md.appendMarkdown(`\n**${i + 1}. ${diagnosticSeverityLabel(diagnostic.severity)}${ruleId ? ` · \`${ruleId}\`` : ''}**\n\n`);

    md.appendMarkdown('**What happened:** ');
    md.appendText(diagnostic.message);
    md.appendMarkdown('\n\n');

    md.appendMarkdown('**Why this matters:** ');
    md.appendText(reference?.summary || fallbackWhyThisMatters(diagnostic.severity));
    md.appendMarkdown('\n\n');

    if (reference?.category) {
      md.appendMarkdown('**VHDL concept:** ');
      md.appendText(reference.category);
      md.appendMarkdown('\n\n');
    }

    md.appendMarkdown('**Suggested next step:** ');
    md.appendText(suggestNextStep(ruleId, reference?.category ?? '', diagnostic.message));
    md.appendMarkdown('\n');
  }

  if (hiddenCount > 0) {
    md.appendMarkdown('\n---\n');
    md.appendText(`${hiddenCount} additional diagnostics on this line are hidden. Increase vhdlLsp.ui.richDiagnostics.maxInHover to show more.`);
    md.appendMarkdown('\n');
  }

  if (ruleReferenceDocPath !== undefined) {
    md.appendMarkdown('\n---\n');
    md.appendText('Rule reference source: docs/rules.md');
  }

  return md;
}

function diagnosticsAtPosition(uri: vscode.Uri, position: vscode.Position): vscode.Diagnostic[] {
  const diagnostics = vscode.languages.getDiagnostics(uri);
  return diagnostics.filter(diagnostic => diagnosticTouchesLine(diagnostic, position.line));
}

function diagnosticTouchesLine(diagnostic: vscode.Diagnostic, line: number): boolean {
  return diagnostic.range.start.line <= line && diagnostic.range.end.line >= line;
}

function isVhdlLintDiagnostic(diagnostic: vscode.Diagnostic): boolean {
  const source = diagnostic.source?.toLowerCase() ?? '';
  if (source.includes('vhdl-lint')) {
    return true;
  }
  const code = diagnosticRuleId(diagnostic);
  return code !== undefined && code !== '';
}

function diagnosticRuleId(diagnostic: vscode.Diagnostic): string | undefined {
  const { code } = diagnostic;
  if (typeof code === 'string') {
    return code;
  }
  if (typeof code === 'number') {
    return String(code);
  }
  if (code !== undefined && typeof code === 'object' && 'value' in code) {
    const value = (code as { value: unknown }).value;
    if (typeof value === 'string') {
      return value;
    }
    if (typeof value === 'number') {
      return String(value);
    }
  }
  return undefined;
}

function diagnosticSeverityLabel(severity: vscode.DiagnosticSeverity): string {
  switch (severity) {
    case vscode.DiagnosticSeverity.Error:
      return 'Error';
    case vscode.DiagnosticSeverity.Warning:
      return 'Warning';
    case vscode.DiagnosticSeverity.Information:
      return 'Info';
    case vscode.DiagnosticSeverity.Hint:
    default:
      return 'Hint';
  }
}

function fallbackRuleReference(ruleId: string): RuleReference | undefined {
  if (ruleId === 'parse_error') {
    return {
      ruleId,
      category: 'Syntax & Parsing',
      severity: 'error',
      defaultState: 'on',
      optional: false,
      summary: 'The file could not be parsed as valid VHDL syntax, so downstream analysis may be incomplete or misleading.'
    };
  }
  if (ruleId === 'missing_check') {
    return {
      ruleId,
      category: 'Verification Contracts',
      severity: 'info',
      defaultState: 'on',
      optional: false,
      summary: 'Expected verification coverage/check tags were not found for this scope.'
    };
  }
  if (ruleId === 'ambiguous_construct') {
    return {
      ruleId,
      category: 'Verification Contracts',
      severity: 'info',
      defaultState: 'on',
      optional: false,
      summary: 'The parser found a construct that can be interpreted in multiple ways and needs manual review.'
    };
  }
  return undefined;
}

function fallbackWhyThisMatters(severity: vscode.DiagnosticSeverity): string {
  switch (severity) {
    case vscode.DiagnosticSeverity.Error:
      return 'Errors usually block compile/elaboration or indicate behavior that can fail in silicon.';
    case vscode.DiagnosticSeverity.Warning:
      return 'Warnings often compile but indicate hardware risk, portability issues, or maintainability debt.';
    case vscode.DiagnosticSeverity.Information:
      return 'Info diagnostics are guidance for robustness and code quality.';
    case vscode.DiagnosticSeverity.Hint:
    default:
      return 'Hints call out ambiguity or style issues worth validating with design intent.';
  }
}

function suggestNextStep(ruleId: string | undefined, category: string, message: string): string {
  const id = (ruleId ?? '').toLowerCase();
  const msg = message.toLowerCase();
  const cat = category.toLowerCase();

  if (id.includes('unused')) {
    return 'Either remove the unused declaration or connect it to real logic so intent is explicit.';
  }
  if (id.includes('missing') || id.includes('incomplete')) {
    return 'Add the missing branch/assignment/check so every legal state and path is handled deterministically.';
  }
  if (id.includes('unresolved') || id.includes('ambiguous')) {
    return 'Verify library/use clauses and explicit qualifiers so names resolve to exactly one design unit.';
  }
  if (id.includes('reset') || id.includes('clock')) {
    return 'Confirm sequential logic has a clear clock/reset strategy and deterministic startup behavior.';
  }
  if (id.includes('cdc') || id.includes('rdc')) {
    return 'Add proper synchronizers/handshakes and keep crossings explicit at domain boundaries.';
  }
  if (id.includes('latch') || id.includes('combinational')) {
    return 'Ensure combinational blocks assign all outputs on all paths and include default assignments where needed.';
  }
  if (id.includes('fsm') || cat.includes('state machine')) {
    return 'Make state transitions exhaustive and explicit, with a safe default/others path and reset state.';
  }
  if (id.includes('naming') || cat.includes('style')) {
    return 'Rename to the team convention to make intent discoverable to other engineers and tools.';
  }
  if (id.includes('port') || id.includes('signal') || cat.includes('ports') || cat.includes('signals')) {
    return 'Trace declarations, directions, and drivers to ensure each interface signal is connected and typed as intended.';
  }
  if (msg.includes('parse')) {
    return 'Fix syntax first; parse errors can hide real downstream lint findings.';
  }
  return 'Open the referenced declaration/process and make the hardware intent explicit in type, assignment, and connectivity.';
}

function updateInlineDiagnostics(): void {
  for (const editor of vscode.window.visibleTextEditors) {
    applyInlineDiagnostics(editor);
  }
}

function applyInlineDiagnostics(editor: vscode.TextEditor): void {
  const decorations = getInlineDecorations();
  if (decorations === null) {
    return;
  }

  const config = vscode.workspace.getConfiguration('vhdlLsp.ui.inlineDiagnostics');
  const enabled = config.get<boolean>('enabled', true);
  if (!enabled || !isVhdlDocument(editor.document)) {
    clearInlineDecorations(editor, decorations);
    return;
  }

  const maxPerFile = clampPositiveInt(config.get<number>('maxPerFile', 80), 80);
  const maxMessagesPerLine = clampPositiveInt(config.get<number>('maxMessagesPerLine', 2), 2);
  const allowed = parseInlineSeverityConfig(config.get<unknown>('includeSeverities'));
  const diagnostics = vscode.languages.getDiagnostics(editor.document.uri);

  const bySeverity: Record<InlineSeverity, vscode.DecorationOptions[]> = {
    error: [],
    warning: [],
    information: [],
    hint: []
  };

  const groupedByLine = new Map<number, LineInlineGroup>();
  let considered = 0;
  for (const diagnostic of diagnostics) {
    if (considered >= maxPerFile) {
      break;
    }
    const severity = diagnosticSeverityToInlineSeverity(diagnostic.severity);
    if (!allowed.has(severity)) {
      continue;
    }

    const line = diagnostic.range.start.line;
    if (line < 0 || line >= editor.document.lineCount) {
      continue;
    }
    const message = summarizeInlineMessage(diagnostic.message, 110);
    if (message === '') {
      continue;
    }
    considered++;

    const existing = groupedByLine.get(line);
    if (existing === undefined) {
      groupedByLine.set(line, {
        entries: [diagnostic],
        seen: new Set([`${severity}\u0000${diagnosticRuleId(diagnostic) ?? ''}\u0000${message}`])
      });
      continue;
    }
    const key = `${severity}\u0000${diagnosticRuleId(diagnostic) ?? ''}\u0000${message}`;
    if (!existing.seen.has(key)) {
      existing.entries.push(diagnostic);
      existing.seen.add(key);
    }
  }

  for (const [line, group] of groupedByLine) {
    const sorted = [...group.entries].sort((a, b) => diagnosticDisplayPriority(a) - diagnosticDisplayPriority(b));
    const counts = countInlineSeverityFromDiagnostics(sorted);
    const dominantSeverity = sorted.length > 0 ? diagnosticSeverityToInlineSeverity(sorted[0].severity) : 'hint';
    const preview = sorted.slice(0, maxMessagesPerLine);
    const hiddenCount = Math.max(0, sorted.length - preview.length);

    const lineText = editor.document.lineAt(line).text;
    const endChar = lineText.length;
    const countSummary = formatInlineCountSummary(counts);
    const previewSummary = preview
      .map(entry => `[${inlineSeverityPrefix(diagnosticSeverityToInlineSeverity(entry.severity))}] ${summarizeInlineMessage(entry.message, 72)}`)
      .join(' | ');
    const hiddenSuffix = hiddenCount > 0 ? ` (+${hiddenCount} more)` : '';
    const hover = buildRichDiagnosticsMarkdown(sorted, 0);

    bySeverity[dominantSeverity].push({
      range: new vscode.Range(line, endChar, line, endChar),
      hoverMessage: hover,
      renderOptions: {
        after: {
          contentText: `  ${countSummary} ${previewSummary}${hiddenSuffix}`
        }
      }
    });
  }

  editor.setDecorations(decorations.error, bySeverity.error);
  editor.setDecorations(decorations.warning, bySeverity.warning);
  editor.setDecorations(decorations.information, bySeverity.information);
  editor.setDecorations(decorations.hint, bySeverity.hint);
}

type LineInlineGroup = {
  entries: vscode.Diagnostic[];
  seen: Set<string>;
};

type RuleReference = {
  ruleId: string;
  category: string;
  severity: string;
  defaultState: string;
  optional: boolean;
  summary: string;
};

function clearInlineDecorations(
  editor: vscode.TextEditor,
  decorations: {
    error: vscode.TextEditorDecorationType;
    warning: vscode.TextEditorDecorationType;
    information: vscode.TextEditorDecorationType;
    hint: vscode.TextEditorDecorationType;
  }
): void {
  editor.setDecorations(decorations.error, []);
  editor.setDecorations(decorations.warning, []);
  editor.setDecorations(decorations.information, []);
  editor.setDecorations(decorations.hint, []);
}

type InlineSeverity = 'error' | 'warning' | 'information' | 'hint';

function getInlineDecorations(): {
  error: vscode.TextEditorDecorationType;
  warning: vscode.TextEditorDecorationType;
  information: vscode.TextEditorDecorationType;
  hint: vscode.TextEditorDecorationType;
} | null {
  if (
    inlineErrorDecoration === undefined ||
    inlineWarningDecoration === undefined ||
    inlineInfoDecoration === undefined ||
    inlineHintDecoration === undefined
  ) {
    return null;
  }
  return {
    error: inlineErrorDecoration,
    warning: inlineWarningDecoration,
    information: inlineInfoDecoration,
    hint: inlineHintDecoration
  };
}

function diagnosticSeverityToInlineSeverity(severity: vscode.DiagnosticSeverity): InlineSeverity {
  switch (severity) {
    case vscode.DiagnosticSeverity.Error:
      return 'error';
    case vscode.DiagnosticSeverity.Warning:
      return 'warning';
    case vscode.DiagnosticSeverity.Information:
      return 'information';
    case vscode.DiagnosticSeverity.Hint:
    default:
      return 'hint';
  }
}

function parseInlineSeverityConfig(value: unknown): Set<InlineSeverity> {
  const defaults: InlineSeverity[] = ['error', 'warning'];
  if (!Array.isArray(value)) {
    return new Set(defaults);
  }
  const parsed: InlineSeverity[] = [];
  for (const item of value) {
    if (item === 'error' || item === 'warning' || item === 'information' || item === 'hint') {
      parsed.push(item);
    }
  }
  if (parsed.length === 0) {
    return new Set(defaults);
  }
  return new Set(parsed);
}

function inlineSeverityPrefix(severity: InlineSeverity): string {
  switch (severity) {
    case 'error':
      return 'E';
    case 'warning':
      return 'W';
    case 'information':
      return 'I';
    case 'hint':
    default:
      return 'H';
  }
}

function inlineSeverityPriority(severity: InlineSeverity): number {
  switch (severity) {
    case 'error':
      return 0;
    case 'warning':
      return 1;
    case 'information':
      return 2;
    case 'hint':
    default:
      return 3;
  }
}

function diagnosticDisplayPriority(diagnostic: vscode.Diagnostic): number {
  const severity = diagnosticSeverityToInlineSeverity(diagnostic.severity);
  const severityRank = inlineSeverityPriority(severity);
  return severityRank * 1_000_000 + diagnostic.range.start.line * 10_000 + diagnostic.range.start.character;
}

function countInlineSeverityFromDiagnostics(diagnostics: readonly vscode.Diagnostic[]): Record<InlineSeverity, number> {
  const counts: Record<InlineSeverity, number> = {
    error: 0,
    warning: 0,
    information: 0,
    hint: 0
  };
  for (const diagnostic of diagnostics) {
    const severity = diagnosticSeverityToInlineSeverity(diagnostic.severity);
    counts[severity]++;
  }
  return counts;
}

function formatInlineCountSummary(counts: Record<InlineSeverity, number>): string {
  const parts: string[] = [];
  if (counts.error > 0) {
    parts.push(`E${counts.error}`);
  }
  if (counts.warning > 0) {
    parts.push(`W${counts.warning}`);
  }
  if (counts.information > 0) {
    parts.push(`I${counts.information}`);
  }
  if (counts.hint > 0) {
    parts.push(`H${counts.hint}`);
  }
  if (parts.length === 0) {
    return '[ ]';
  }
  return `[${parts.join('/')}]`;
}

function summarizeInlineMessage(raw: string, maxLength = 140): string {
  const firstLine = raw.split(/\r?\n/, 2)[0].trim();
  if (firstLine === '') {
    return '';
  }
  if (firstLine.length <= maxLength) {
    return firstLine;
  }
  if (maxLength <= 3) {
    return '...';
  }
  return firstLine.slice(0, maxLength - 3) + '...';
}

function clampPositiveInt(value: number | undefined, fallback: number): number {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return fallback;
  }
  const floored = Math.floor(value);
  if (floored < 1) {
    return fallback;
  }
  return floored;
}

function countVhdlDiagnostics(): {
  errors: number;
  warnings: number;
  infos: number;
  hints: number;
  total: number;
} {
  let errors = 0;
  let warnings = 0;
  let infos = 0;
  let hints = 0;

  for (const [uri, diagnostics] of vscode.languages.getDiagnostics()) {
    if (!isVhdlUri(uri)) {
      continue;
    }

    for (const diagnostic of diagnostics) {
      switch (diagnostic.severity) {
        case vscode.DiagnosticSeverity.Error:
          errors++;
          break;
        case vscode.DiagnosticSeverity.Warning:
          warnings++;
          break;
        case vscode.DiagnosticSeverity.Information:
          infos++;
          break;
        case vscode.DiagnosticSeverity.Hint:
          hints++;
          break;
      }
    }
  }

  return {
    errors,
    warnings,
    infos,
    hints,
    total: errors + warnings + infos + hints
  };
}

function isVhdlUri(uri: vscode.Uri): boolean {
  const lowerPath = uri.fsPath.toLowerCase();
  if (lowerPath.endsWith('.vhd') || lowerPath.endsWith('.vhdl')) {
    return true;
  }

  const doc = vscode.workspace.textDocuments.find(d => d.uri.toString() === uri.toString());
  return doc?.languageId === 'vhdl';
}

function isVhdlDocument(doc: vscode.TextDocument): boolean {
  if (doc.languageId === 'vhdl') {
    return true;
  }
  const lowerPath = doc.uri.fsPath.toLowerCase();
  return lowerPath.endsWith('.vhd') || lowerPath.endsWith('.vhdl');
}

function sanitizeArgs(value: unknown): string[] {
  if (!Array.isArray(value)) {
    return [];
  }
  const out: string[] = [];
  for (const item of value) {
    if (typeof item === 'string' && item.trim() !== '') {
      out.push(item);
    }
  }
  return out;
}

function coerceEnv(value: unknown): Record<string, string> {
  if (value === null || typeof value !== 'object') {
    return {};
  }
  const out: Record<string, string> = {};
  for (const [key, raw] of Object.entries(value as Record<string, unknown>)) {
    if (typeof raw === 'string') {
      out[key] = raw;
      continue;
    }
    if (typeof raw === 'number' || typeof raw === 'boolean') {
      out[key] = String(raw);
    }
  }
  return out;
}
