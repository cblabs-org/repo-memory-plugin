package antigravity

import (
	_ "embed"
	"strings"
)

//go:embed hooks.json
var HooksJSONTemplate string

//go:embed mcp_config.json
var MCPConfigJSONTemplate string

//go:embed sidecar.json
var SidecarJSONTemplate string

//go:embed daemon_plugin.json
var DaemonPluginJSONTemplate string

//go:embed workspace_plugin.json
var WorkspacePluginJSONTemplate string

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
