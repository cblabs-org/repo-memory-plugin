/*
 * repo-memory-plugin (c) 2026 cblabs-org
 *
 * https://github.com/cblabs-org/repo-memory-plugin
 *
 * SPDX-License-Identifier: Apache-2.0
 *
 * Module: internal/core/delivery/types.go
 * Description: Defines delivery channels, trigger moments, and default token
 *              budgets for the repository memory push-delivery engine.
 *
 * Exported Types / Functions:
 * - Channel:           Identifies the harness transport channel used for delivery.
 * - ChannelSystem:     System prompt / context channel.
 * - ChannelUser:       User message channel.
 * - ChannelEphemeral:  Ephemeral mid-turn injection channel.
 * - ChannelDeny:       Tool-execution denial channel (used for edit gates).
 * - Moment:            Identifies the agent lifecycle moment triggering delivery.
 * - MomentTaskStart:   Start-of-task / pre-invocation hook moment.
 * - MomentFirstEntry:  First entry into a module dossier scope.
 * - MomentFirstTouch:  First read or touch of an anchored file or symbol.
 * - MomentAboutToEdit: Pre-edit notification moment.
 * - MomentError:       Post-command failure diagnostic moment.
 * - MomentGate:        Pre-edit gate check moment.
 * - DefaultBudgets():  Returns the default token budget table keyed by Moment.
 */

package delivery

// Channel identifies the harness output channel used to deliver repository memory.
type Channel string

const (
	// ChannelSystem delivers memory through the harness system context channel.
	ChannelSystem Channel = "system"
	// ChannelUser delivers memory through the user message channel.
	ChannelUser Channel = "userMessage"
	// ChannelEphemeral delivers memory as an ephemeral mid-turn context block.
	ChannelEphemeral Channel = "ephemeral"
	// ChannelDeny delivers memory as a pre-tool-use denial reason.
	ChannelDeny Channel = "deny"
)

// Moment identifies the lifecycle point in an agent session that triggers push delivery.
type Moment string

const (
	// MomentTaskStart triggers at the start of an invocation from prompt terms and git diff paths.
	MomentTaskStart Moment = "task_start"
	// MomentFirstEntry triggers the first time an agent enters a module dossier scope.
	MomentFirstEntry Moment = "first_entry"
	// MomentFirstTouch triggers the first time an agent reads or touches an anchored file or symbol.
	MomentFirstTouch Moment = "first_touch"
	// MomentAboutToEdit triggers immediately before an agent edits a file.
	MomentAboutToEdit Moment = "about_to_edit"
	// MomentError triggers when a shell command or tool execution fails.
	MomentError Moment = "error"
	// MomentGate triggers during pre-edit gate evaluation for `gate: edit` entries.
	MomentGate Moment = "gate"
)

// DefaultBudgets returns a fresh map of default token budgets for each delivery Moment.
func DefaultBudgets() map[Moment]int {
	return map[Moment]int{
		MomentTaskStart:   500,
		MomentFirstEntry:  150,
		MomentFirstTouch:  150,
		MomentAboutToEdit: 50,
		MomentError:       200,
		MomentGate:        200,
	}
}
