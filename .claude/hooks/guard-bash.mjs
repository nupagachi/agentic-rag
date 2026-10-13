#!/usr/bin/env node
// PreToolUse guard for Bash/PowerShell commands.
// Usage:
//   node guard-bash.mjs                -> global rules (every session and agent)
//   node guard-bash.mjs --implementer  -> + no push / no GitHub writes (fe/be/test-agent)
//   node guard-bash.mjs --readonly     -> + no git writes / no file writes (review/security-agent)
// Exit 2 blocks the tool call; stderr is shown to Claude as the reason.

const mode = process.argv[2] ?? "--global";

let raw = "";
for await (const chunk of process.stdin) raw += chunk;

let command = "";
try {
  command = JSON.parse(raw)?.tool_input?.command ?? "";
} catch {
  process.exit(0); // not our payload; let other checks decide
}
if (!command) process.exit(0);

const GLOBAL = [
  [/\bgh\s+pr\s+merge\b/, "Merging is reserved for the human Owner."],
  [/\bgit\s+push\b[^\n;&|]*(--force\b|--force-with-lease\b|\s-f(\s|$))/, "Force push is not allowed."],
  [/\bgit\s+push\b[^\n;&|]*\s(HEAD:)?(refs\/heads\/)?(main|master)(\s|$)/, "Pushing to main is not allowed; open a PR."],
  [/--no-verify\b/, "Skipping git hooks is not allowed."],
  [/\brm\s+-[a-zA-Z]*r[a-zA-Z]*f?\s+(\/|~|\$HOME|\.|\*)(\s|$)/, "Recursive delete of root/home/repo is not allowed."],
  [/\bdocker\s+compose\b[^\n]*\bdown\b[^\n]*\s-v\b/, "Deleting docker volumes is not allowed."],
  [/(cat|less|more|head|tail|type|Get-Content|gc|grep|rg|sed|awk|cp|scp)\b[^\n;&|]*(\.env(\s|$|")|deploy[\\/]secrets|\.pem\b|master_key|internal_token)/,
    "Reading secrets (.env, deploy/secrets, *.pem) is not allowed."],
];

const IMPLEMENTER = [
  [/\bgit\s+push\b/, "Implementer agents must not push; the orchestrator pushes after the quality gate."],
  [/\bgh\s+(pr\s+(?!view\b|diff\b|checks\b)|issue|release|repo|api|workflow|secret|variable)/, "Agents may only read PRs (gh pr view/diff/checks); report back to the orchestrator."],
  [/\bgit\s+(rebase|reset\s+--hard|filter-branch|filter-repo)\b/, "History rewriting is not allowed for implementer agents."],
];

const READONLY = [
  ...IMPLEMENTER,
  [/\bgit\s+(add|commit|restore|checkout\s+--|stash|merge|cherry-pick|switch\s+-c|branch\s+-[dD])\b/, "Read-only agent: git writes are not allowed."],
  [/\b(sed\s+-i|tee|Set-Content|Out-File|Add-Content|New-Item|Remove-Item|mv|rm|cp)\b/, "Read-only agent: file modifications are not allowed."],
  [/(^|[^>2&])>{1,2}\s*(?!\/dev\/null|\$null|&)[^\s]/, "Read-only agent: output redirection to files is not allowed."],
];

const rules =
  mode === "--readonly" ? [...GLOBAL, ...READONLY]
  : mode === "--implementer" ? [...GLOBAL, ...IMPLEMENTER]
  : GLOBAL;

for (const [pattern, reason] of rules) {
  if (pattern.test(command)) {
    process.stderr.write(`Blocked by .claude/hooks/guard-bash.mjs (${mode}): ${reason}\nCommand: ${command}\n`);
    process.exit(2);
  }
}
process.exit(0);
