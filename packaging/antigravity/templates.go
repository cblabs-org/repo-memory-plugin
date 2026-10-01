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

//go:embed plugin.json
var PluginJSONTemplate string

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

// GeneratePluginJSON returns the static plugin.json manifest content.
func GeneratePluginJSON() string {
	return PluginJSONTemplate
}
