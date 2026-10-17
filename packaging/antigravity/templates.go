/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: packaging/antigravity/templates.go
 * Description: Embedded plugin manifests, hook/MCP/sidecar JSON templates,
 *              distiller agent definition, global management skill templates,
 *              and portable archive bundle generators for the Antigravity
 *              two-plugin packaging split.
 *
 * Exported Types / Functions:
 * - DefaultPluginVersion:                      Fallback semantic version stamped into plugin manifests when Version is "dev".
 * - HooksJSONTemplate:                         Raw embedded hooks.json template.
 * - MCPConfigJSONTemplate:                     Raw embedded mcp_config.json template.
 * - SidecarJSONTemplate:                       Raw embedded sidecar.json template.
 * - DaemonPluginJSONTemplate:                  Raw embedded daemon_plugin.json manifest.
 * - WorkspacePluginJSONTemplate:               Raw embedded workspace_plugin.json manifest.
 * - DistillerAgentMDTemplate:                  Raw embedded repo-memory-distiller agent markdown.
 * - AdminSkillMDTemplate:                      Raw embedded repo-memory-admin skill markdown.
 * - NormalizePluginVersion():                  Normalizes a build version string into clean SemVer for plugin.json.
 * - RenderTemplate():                          Substitutes <bin> with a JSON-escaped binary path.
 * - GenerateHooksJSON():                       Renders hooks.json with the binary path substituted.
 * - GenerateMCPConfigJSON():                   Renders mcp_config.json with the binary path substituted.
 * - GeneratePortableMCPConfigJSON():           Renders mcp_config.json using a shell-expanded portable runner path.
 * - GenerateSidecarJSON():                     Renders sidecar.json with the binary path substituted.
 * - GeneratePortableSidecarJSON():             Renders sidecar.json using a shell-expanded portable runner path.
 * - GeneratePortableRunnerScript():            Returns the POSIX fail-open shell wrapper that strips macOS quarantine and execs the binary.
 * - GenerateDaemonPluginJSON():                Returns the static daemon_plugin.json manifest content.
 * - GenerateDaemonPluginJSONWithVersion():     Renders daemon_plugin.json with the specified version string stamped.
 * - GenerateWorkspacePluginJSON():             Returns the static workspace_plugin.json manifest content.
 * - GenerateWorkspacePluginJSONWithVersion():  Renders workspace_plugin.json with the specified version string stamped.
 * - GenerateConfigJSON():                      Returns the static config.json content enabling the daemon plugin.
 * - GenerateDistillerAgentMD():                Returns the static repo-memory-distiller agent definition markdown.
 * - GenerateAdminSkillMD():                    Renders the repo-memory-admin SKILL.md with the binary path substituted.
 */

// Package antigravity provides embedded configuration templates for the Antigravity harness.
package antigravity

import (
	_ "embed"
	"encoding/json"
	"strings"
)

// DefaultPluginVersion is the fallback semantic version written to plugin.json when the build version is unset or "dev".
const DefaultPluginVersion = "0.1.0"

// HooksJSONTemplate holds the raw embedded hooks.json template.
//
//go:embed hooks.json
var HooksJSONTemplate string

// MCPConfigJSONTemplate holds the raw embedded mcp_config.json template.
//
//go:embed mcp_config.json
var MCPConfigJSONTemplate string

// SidecarJSONTemplate holds the raw embedded sidecar.json template.
//
//go:embed sidecar.json
var SidecarJSONTemplate string

// DaemonPluginJSONTemplate holds the raw embedded daemon_plugin.json manifest.
//
//go:embed daemon_plugin.json
var DaemonPluginJSONTemplate string

// WorkspacePluginJSONTemplate holds the raw embedded workspace_plugin.json manifest.
//
//go:embed workspace_plugin.json
var WorkspacePluginJSONTemplate string

// DistillerAgentMDTemplate holds the raw embedded repo-memory-distiller agent definition.
//
//go:embed agents/repo-memory-distiller/agent.md
var DistillerAgentMDTemplate string

// AdminSkillMDTemplate holds the raw embedded repo-memory-admin management skill definition.
//
//go:embed skills/repo-memory-admin/SKILL.md
var AdminSkillMDTemplate string

// NormalizePluginVersion converts a build or git tag version string (such as "v0.2.0" or "dev")
// into a clean semantic version string suitable for plugin.json manifests.
func NormalizePluginVersion(version string) string {
	trimmed := strings.TrimSpace(version)
	trimmed = strings.TrimPrefix(trimmed, "v")
	if trimmed == "" || trimmed == "dev" {
		return DefaultPluginVersion
	}
	return trimmed
}

// RenderTemplate replaces <bin> with the provided binary path.
// Backslashes in the binary path (e.g. on Windows) are JSON-escaped.
func RenderTemplate(tmpl, binPath string) string {
	escapedBin := strings.ReplaceAll(binPath, `\`, `\\`)
	return strings.ReplaceAll(tmpl, "<bin>", escapedBin)
}

// GenerateHooksJSON generates the hooks.json file content with the binary path substituted.
func GenerateHooksJSON(binPath string) string {
	return RenderTemplate(HooksJSONTemplate, binPath)
}

// GenerateMCPConfigJSON generates the mcp_config.json file content with the binary path substituted.
func GenerateMCPConfigJSON(binPath string) string {
	return RenderTemplate(MCPConfigJSONTemplate, binPath)
}

// GeneratePortableMCPConfigJSON generates a portable mcp_config.json that invokes the bundled
// runner via /bin/sh -c (or cmd.exe /c on Windows) so $HOME or %USERPROFILE% is expanded at runtime.
func GeneratePortableMCPConfigJSON(targetOS string) string {
	// --- Step 1: Select OS-specific shell wrapper command and environment variable ---
	cmd := "/bin/sh"
	args := []string{
		"-c",
		`exec "$HOME/.gemini/config/plugins/repo-memory-daemon/bin/run-repo-memory.sh" mcp`,
	}
	if strings.EqualFold(targetOS, "windows") {
		cmd = "cmd.exe"
		args = []string{
			"/c",
			`"%USERPROFILE%\.gemini\config\plugins\repo-memory-daemon\bin\repo-memory.exe" mcp`,
		}
	}

	// --- Step 2: Marshal portable MCP configuration payload ---
	payload := map[string]any{
		"mcpServers": map[string]any{
			"repo-memory": map[string]any{
				"command": cmd,
				"args":    args,
			},
		},
	}
	out, _ := json.MarshalIndent(payload, "", "  ")
	return string(out) + "\n"
}

// GenerateSidecarJSON generates the sidecar.json file content with the binary path substituted.
func GenerateSidecarJSON(binPath string) string {
	return RenderTemplate(SidecarJSONTemplate, binPath)
}

// GeneratePortableSidecarJSON generates a portable sidecar.json that invokes the bundled
// runner via /bin/sh -c (or cmd.exe /c on Windows) so $HOME or %USERPROFILE% is expanded at runtime.
func GeneratePortableSidecarJSON(targetOS string) string {
	// --- Step 1: Build schedule arguments with shell-expanded runner path ---
	scheduleArgs := []string{
		"*/1 * * * *",
		"/bin/sh",
		"-c",
		`exec "$HOME/.gemini/config/plugins/repo-memory-daemon/bin/run-repo-memory.sh" tick`,
	}
	if strings.EqualFold(targetOS, "windows") {
		scheduleArgs = []string{
			"*/1 * * * *",
			"cmd.exe",
			"/c",
			`"%USERPROFILE%\.gemini\config\plugins\repo-memory-daemon\bin\repo-memory.exe" tick`,
		}
	}

	// --- Step 2: Marshal portable sidecar configuration payload ---
	payload := map[string]any{
		"builtin":        "schedule",
		"args":           scheduleArgs,
		"restart_policy": "always",
		"display_name":   "Repo Memory Tick",
		"description":    "Advances asynchronous background memory pipeline jobs and scheduled maintenance.",
	}
	out, _ := json.MarshalIndent(payload, "", "  ")
	return string(out) + "\n"
}

// portableRunnerScriptTemplate is the POSIX shell wrapper bundled inside portable .zip
// archives at bin/run-repo-memory.sh.
const portableRunnerScriptTemplate = `#!/bin/sh
# Fail-open portable wrapper for repo-memory plugin bundles.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
BIN="${SCRIPT_DIR}/repo-memory"

# Clear macOS Gatekeeper quarantine attribute if extracted via Finder/Browser
if [ "$(uname -s)" = "Darwin" ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$BIN" 2>/dev/null || true
fi

if [ ! -x "$BIN" ]; then
  for arg in "$@"; do
    case "$arg" in
      PreToolUse|pre_tool_use)
        echo '{"decision":"allow","reason":"fail-open: binary not found"}'
        exit 0
        ;;
    esac
  done
  echo '{}'
  exit 0
fi

exec "$BIN" "$@"
`

// GeneratePortableRunnerScript returns the POSIX shell wrapper bundled inside portable .zip
// archives at bin/run-repo-memory.sh. It resolves its own directory, clears macOS browser
// quarantine (com.apple.quarantine) on first execution, fails open on PreToolUse if the
// binary is missing, and execs the adjacent repo-memory binary.
func GeneratePortableRunnerScript() string {
	return portableRunnerScriptTemplate
}

// GenerateDaemonPluginJSON returns the static daemon_plugin.json manifest content.
func GenerateDaemonPluginJSON() string {
	return DaemonPluginJSONTemplate
}

// GenerateDaemonPluginJSONWithVersion returns daemon_plugin.json stamped with the normalized version string.
func GenerateDaemonPluginJSONWithVersion(version string) string {
	norm := NormalizePluginVersion(version)
	return strings.Replace(DaemonPluginJSONTemplate, `"version": "0.1.0"`, `"version": "`+norm+`"`, 1)
}

// GenerateWorkspacePluginJSON returns the static workspace_plugin.json manifest content.
func GenerateWorkspacePluginJSON() string {
	return WorkspacePluginJSONTemplate
}

// GenerateWorkspacePluginJSONWithVersion returns workspace_plugin.json stamped with the normalized version string.
func GenerateWorkspacePluginJSONWithVersion(version string) string {
	norm := NormalizePluginVersion(version)
	return strings.Replace(WorkspacePluginJSONTemplate, `"version": "0.1.0"`, `"version": "`+norm+`"`, 1)
}

// GenerateConfigJSON returns the static config.json content enabling the daemon plugin.
func GenerateConfigJSON() string {
	return "{\n  \"enabled\": true\n}\n"
}

// GenerateDistillerAgentMD returns the static repo-memory-distiller agent definition markdown.
func GenerateDistillerAgentMD() string {
	return DistillerAgentMDTemplate
}

// GenerateAdminSkillMD generates the repo-memory-admin SKILL.md content with the binary path substituted.
func GenerateAdminSkillMD(binPath string) string {
	return strings.ReplaceAll(AdminSkillMDTemplate, "<bin>", binPath)
}
