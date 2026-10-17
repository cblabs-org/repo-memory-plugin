/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: packaging/antigravity/templates.go
 * Description: Embedded plugin manifests, hook/MCP/sidecar JSON templates,
 *              distiller agent definition, and global management skill
 *              templates for the Antigravity two-plugin packaging split.
 *
 * Exported Types / Functions:
 * - HooksJSONTemplate:           Raw embedded hooks.json template.
 * - MCPConfigJSONTemplate:       Raw embedded mcp_config.json template.
 * - SidecarJSONTemplate:         Raw embedded sidecar.json template.
 * - DaemonPluginJSONTemplate:    Raw embedded daemon_plugin.json manifest.
 * - WorkspacePluginJSONTemplate: Raw embedded workspace_plugin.json manifest.
 * - DistillerAgentMDTemplate:    Raw embedded repo-memory-distiller agent markdown.
 * - AdminSkillMDTemplate:        Raw embedded repo-memory-admin skill markdown.
 * - RenderTemplate():            Substitutes <bin> with a JSON-escaped binary path.
 * - GenerateHooksJSON():         Renders hooks.json with the binary path substituted.
 * - GenerateMCPConfigJSON():     Renders mcp_config.json with the binary path substituted.
 * - GenerateSidecarJSON():       Renders sidecar.json with the binary path substituted.
 * - GenerateDaemonPluginJSON():  Returns the static daemon_plugin.json manifest content.
 * - GenerateWorkspacePluginJSON(): Returns the static workspace_plugin.json manifest content.
 * - GenerateConfigJSON():        Returns the static config.json content enabling the daemon plugin.
 * - GenerateDistillerAgentMD():  Returns the static repo-memory-distiller agent definition markdown.
 * - GenerateAdminSkillMD():      Renders the repo-memory-admin SKILL.md with the binary path substituted.
 */

// Package antigravity provides embedded configuration templates for the Antigravity harness.
package antigravity

import (
	_ "embed"
	"strings"
)

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

// GenerateSidecarJSON generates the sidecar.json file content with the binary path substituted.
func GenerateSidecarJSON(binPath string) string {
	return RenderTemplate(SidecarJSONTemplate, binPath)
}

// GenerateDaemonPluginJSON returns the static daemon_plugin.json manifest content.
func GenerateDaemonPluginJSON() string {
	return DaemonPluginJSONTemplate
}

// GenerateWorkspacePluginJSON returns the static workspace_plugin.json manifest content.
func GenerateWorkspacePluginJSON() string {
	return WorkspacePluginJSONTemplate
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
