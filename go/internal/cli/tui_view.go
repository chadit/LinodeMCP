package cli

import (
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// Lipgloss colors for the TUI chrome. Kept as ANSI 256 codes (strings) so
// the styles render on any terminal the program attaches to.
const (
	colorHeaderFg = "230"
	colorHeaderBg = "62"
	colorFooterFg = "245"
	colorAccentFg = "212"
)

// tuiHeaderStyle styles the top title bar. Built by a function rather than
// a package var so no global mutable lipgloss.Style lives at package
// scope.
func tuiHeaderStyleValue() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(colorHeaderFg)).
		Background(lipgloss.Color(colorHeaderBg)).
		Padding(0, 1)
}

// tuiFooterStyleValue styles the bottom status/help bar.
func tuiFooterStyleValue() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(colorFooterFg)).
		Padding(0, 1)
}

// tuiLabelStyleValue styles a form field's label, brightening the focused
// one and dimming the rest.
func tuiLabelStyleValue(focused bool) lipgloss.Style {
	style := lipgloss.NewStyle()
	if focused {
		return style.Bold(true).Foreground(lipgloss.Color(colorAccentFg))
	}

	return style
}

// The chrome styles are resolved through these package-scoped wrappers so
// the View code reads tuiHeaderStyle/tuiFooterStyle as values. They are
// vars holding the constructor result, evaluated once at init, which keeps
// the call sites terse without a global mutable Style (the value is never
// reassigned).
//
//nolint:gochecknoglobals // immutable lipgloss styles, built once, never mutated
var (
	tuiHeaderStyle = tuiHeaderStyleValue()
	tuiFooterStyle = tuiFooterStyleValue()
)

// screenTitle returns the header text for a screen.
func screenTitle(screen tuiScreen) string {
	switch screen {
	case screenForm:
		return "LinodeMCP TUI - Tool Form"
	case screenRun:
		return "LinodeMCP TUI - Run & Result"
	case screenAudit:
		return "LinodeMCP TUI - Audit"
	case screenProfile:
		return "LinodeMCP TUI - Profiles"
	case screenHealth:
		return "LinodeMCP TUI - Health & Version"
	case screenCatalog:
		return "LinodeMCP TUI - Catalog"
	default:
		return "LinodeMCP TUI - Catalog"
	}
}

// screenKeyHint returns the terse footer key hint for a screen, listing
// the bindings most useful on it.
func screenKeyHint(screen tuiScreen) string {
	switch screen {
	case screenForm:
		return "tab/shift+tab move | space toggle | ctrl+r run | esc back | q quit"
	case screenRun:
		return "y confirm | ctrl+t json/table | up/down scroll | esc back | q quit"
	case screenAudit:
		return "r refresh | up/down scroll | esc back | q quit"
	case screenProfile:
		return "enter switch | / filter | up/down move | esc back | q quit"
	case screenHealth:
		return "r refresh | up/down scroll | esc back | q quit"
	case screenCatalog:
		return "/ filter | enter select | ctrl+e audit | ctrl+p profiles | ctrl+h health | q quit"
	default:
		return "/ filter | enter select | ctrl+a scope | q quit"
	}
}

// renderForm renders the tool form: the tool name, one labeled line per
// field (required marked, type shown), and the safety controls with their
// current values. The focused row is highlighted so the user sees where
// input or a toggle will land.
func renderForm(form *formModel) string {
	lines := make([]string, 0, len(form.fields)+1)

	for idx := range form.fields {
		lines = append(lines, renderFormField(&form.fields[idx], idx == form.focus))
	}

	body := strings.Join(lines, "\n")

	return "Tool: " + form.tool + "\n\n" + body + "\n\n" + renderControls(form)
}

// renderFormField renders one field's label and input box, marking
// required fields and showing the schema type.
func renderFormField(field *formField, focused bool) string {
	marker := " "
	if field.spec.Required {
		marker = "*"
	}

	label := tuiLabelStyleValue(focused).Render(marker + " " + field.spec.Name + " (" + field.spec.TypeName + ")")

	return label + ": " + field.input.View()
}

// renderControls renders the four safety controls with their current
// values, highlighting the focused one. The focus index past the fields
// selects which control is active.
func renderControls(m *formModel) string {
	rows := []string{
		controlRow("dry-run", boolLabel(m.dryRun), m.controlIndex() == controlDryRun),
		controlRow("mode", m.mode, m.controlIndex() == controlMode),
		controlRow("confirm", boolLabel(m.confirm), m.controlIndex() == controlConfirm),
		controlRow("environment", environmentLabel(m), m.controlIndex() == controlEnvironment),
	}

	return strings.Join(rows, "\n")
}

// controlRow renders one safety control line: its name, current value, and
// a focus highlight.
func controlRow(name, value string, focused bool) string {
	return tuiLabelStyleValue(focused).Render("["+name+"] ") + value
}

// boolLabel renders a bool control's value as on/off.
func boolLabel(value bool) string {
	if value {
		return "on"
	}

	return "off"
}

// environmentLabel renders the environment control's current selection.
func environmentLabel(m *formModel) string {
	if m.envIdx >= 0 && m.envIdx < len(m.envChoices) {
		return m.envChoices[m.envIdx]
	}

	return envNone
}

// renderRun renders the run screen: the pending request, a confirm prompt
// while a destructive call waits on the gate, and the result viewport once
// the dispatch completes.
func renderRun(run *runModel) string {
	head := "Tool: " + run.tool + "\nCapability: " + run.cap.String() + "\n\n"

	if run.awaitingConfirm {
		return head + "This tool is destructive. Press y to run, esc to cancel.\n"
	}

	if !run.done {
		return head + "Running...\n"
	}

	return head + run.viewport.View()
}

// sortedStrings returns a sorted copy of names so callers get stable
// ordering without mutating the input slice.
func sortedStrings(names []string) []string {
	out := make([]string, len(names))
	copy(out, names)
	sort.Strings(out)

	return out
}
