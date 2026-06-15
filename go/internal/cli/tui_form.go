package cli

import (
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/mark3labs/mcp-go/mcp"
)

// Two-stage mode cycle values shown on the form's mode control. "none"
// leaves the mode field off the request (the tool's default path); plan
// and apply map to the MCP mode field exactly as Phase 1's --mode does.
const (
	modeNone  = "none"
	modePlan  = "plan"
	modeApply = "apply"
)

// envNone is the environment-picker value meaning "leave environment
// unset", so the server selects its default environment.
const envNone = "(default)"

// formField is one editable argument on the tool form. It wraps a bubbles
// textinput and the schema facts the renderer and the request builder
// need: the property name, its JSON-schema type, and whether the schema
// marks it required. The embedded spec carries the schema facts; input is
// the live text widget.
type formField struct {
	spec  FormFieldSpec
	input textinput.Model
}

// FormFieldSpec is the framework-agnostic description of one tool-form
// field: its argument name, JSON-schema type, and whether the schema marks
// it required. The TUI builds a text input per spec; tests assert on the
// specs directly without standing up the Bubble Tea widgets.
//
// Exported because it is the pure, testable contract of the form: the spec
// list is what BuildFormFieldSpecs produces from a schema, and a test can
// verify field skipping, ordering, and required marking against it.
type FormFieldSpec struct {
	Name     string
	TypeName string
	Required bool
}

// formModel is the tool-form screen: a field per schema property plus the
// safety controls (dry-run, two-stage mode, confirm, environment). The
// focus index walks the fields then the controls; Enter on the last row
// (or the dedicated submit key) builds the request. It owns no server
// reference: building the request is pure, so the parent model performs
// the dispatch.
type formModel struct {
	tool       string
	schema     mcp.ToolInputSchema
	fields     []formField
	envChoices []string

	focus   int
	dryRun  bool
	confirm bool
	mode    string
	envIdx  int

	width  int
	height int
}

// newFormModel builds a form for one tool from its input schema and the
// list of configured environment names. Fields are sorted with required
// ones first, then alphabetically, so the most important inputs lead. The
// environment choices always start with envNone so "leave unset" is the
// default selection.
func newFormModel(tool string, schema mcp.ToolInputSchema, environments []string) formModel {
	fields := buildFormFields(schema)
	if len(fields) > 0 {
		fields[0].input.Focus()
	}

	choices := make([]string, 0, len(environments)+1)
	choices = append(choices, envNone)
	choices = append(choices, environments...)

	return formModel{
		tool:       tool,
		schema:     schema,
		fields:     fields,
		envChoices: choices,
		mode:       modeNone,
	}
}

// BuildFormFieldSpecs turns a tool's input schema into ordered field specs,
// skipping the safety-control properties (dry_run, confirm, mode, plan_id,
// environment, yolo, confirmed_dry_run) because the form renders dedicated
// controls for those rather than raw text boxes. Required fields sort
// first, then alphabetical, so a user fills the mandatory inputs first.
//
// Pure and framework-free: it reads only the schema and returns plain
// structs, so the field-derivation logic (skipping, ordering, required
// marking) is unit-testable without the Bubble Tea widgets. The TUI builds
// a text input per spec; tests assert on the specs.
func BuildFormFieldSpecs(schema mcp.ToolInputSchema) []FormFieldSpec {
	required := requiredLookup(schema.Required)

	names := make([]string, 0, len(schema.Properties))

	for name := range schema.Properties {
		if isSafetyControlField(name) {
			continue
		}

		names = append(names, name)
	}

	sort.Slice(names, func(left, right int) bool {
		_, leftReq := required[names[left]]
		_, rightReq := required[names[right]]

		if leftReq != rightReq {
			return leftReq
		}

		return names[left] < names[right]
	})

	specs := make([]FormFieldSpec, 0, len(names))

	for _, name := range names {
		_, isReq := required[name]
		specs = append(specs, FormFieldSpec{
			Name:     name,
			TypeName: fieldTypeName(schema, name),
			Required: isReq,
		})
	}

	return specs
}

// buildFormFields wraps each field spec in a Bubble Tea text input with a
// type-aware placeholder, producing the live form fields the model edits.
func buildFormFields(schema mcp.ToolInputSchema) []formField {
	specs := BuildFormFieldSpecs(schema)
	fields := make([]formField, 0, len(specs))

	for _, spec := range specs {
		input := textinput.New()
		input.Prompt = ""
		input.Placeholder = placeholderFor(spec.TypeName)

		fields = append(fields, formField{spec: spec, input: input})
	}

	return fields
}

// fieldTypeName returns the schema type for display, defaulting to
// "string" when the schema leaves it unspecified (the same permissive
// default the coercion uses).
func fieldTypeName(schema mcp.ToolInputSchema, name string) string {
	if declared := schemaPropType(schema, name); declared != "" {
		return declared
	}

	return schemaTypeString
}

// schemaTypeString names the JSON-schema string type and the form's
// default type for an untyped property.
const schemaTypeString = "string"

// placeholderFor returns a hint string for a field's text input based on
// its schema type, so an empty field still signals what's expected.
func placeholderFor(typeName string) string {
	switch typeName {
	case schemaTypeNumber, schemaTypeInteger:
		return "number"
	case schemaTypeBoolean:
		return "true or false"
	default:
		return schemaTypeString
	}
}

// isSafetyControlField reports whether a schema property is one the form
// renders as a dedicated safety control rather than a text field. Keeping
// these out of the text fields avoids a user typing a raw mode string
// when the mode control already drives that value.
func isSafetyControlField(name string) bool {
	switch name {
	case fieldDryRun, fieldConfirm, fieldMode, fieldPlanID, fieldEnvironment, fieldYolo, fieldConfirmedDry:
		return true
	default:
		return false
	}
}

// requiredLookup turns the schema's required-name slice into a set for
// O(1) membership checks while sorting and rendering.
func requiredLookup(required []string) map[string]struct{} {
	set := make(map[string]struct{}, len(required))
	for _, name := range required {
		set[name] = struct{}{}
	}

	return set
}

// controlCount is the number of safety controls that follow the text
// fields in the focus order: dry-run, mode, confirm, environment.
const controlCount = 4

// rowCount is the total focusable rows: one per field plus the controls.
func (m *formModel) rowCount() int {
	return len(m.fields) + controlCount
}

// focusOnControls reports whether the focus index currently sits on one
// of the trailing safety controls rather than a text field.
func (m *formModel) focusOnControls() bool {
	return m.focus >= len(m.fields)
}

// controlIndex maps the focus index to a 0-based control position when
// focus is on the controls, or -1 when focus is on a field.
func (m *formModel) controlIndex() int {
	if !m.focusOnControls() {
		return -1
	}

	return m.focus - len(m.fields)
}

// Control positions within the trailing control block, in focus order.
const (
	controlDryRun = iota
	controlMode
	controlConfirm
	controlEnvironment
)

// setSize records the terminal size for the view layout.
func (m *formModel) setSize(width, height int) {
	m.width = width
	m.height = height
}

// focusNext moves focus to the next row, wrapping from the last control
// back to the first field, and updates which text input holds the cursor.
func (m *formModel) focusNext() {
	m.focus = (m.focus + 1) % m.rowCount()
	m.syncFocus()
}

// focusPrev moves focus to the previous row, wrapping from the first field
// to the last control.
func (m *formModel) focusPrev() {
	m.focus = (m.focus - 1 + m.rowCount()) % m.rowCount()
	m.syncFocus()
}

// syncFocus makes exactly the focused text input active (and blinking) and
// blurs the rest, so typing only affects the highlighted field.
func (m *formModel) syncFocus() {
	for idx := range m.fields {
		if idx == m.focus {
			m.fields[idx].input.Focus()

			continue
		}

		m.fields[idx].input.Blur()
	}
}

// toggleControl flips or advances whichever control currently has focus:
// the bool controls toggle, the mode control cycles none/plan/apply, and
// the environment control cycles through the configured environments.
// A no-op when focus is on a text field.
func (m *formModel) toggleControl() {
	switch m.controlIndex() {
	case controlDryRun:
		m.dryRun = !m.dryRun
	case controlConfirm:
		m.confirm = !m.confirm
	case controlMode:
		m.cycleMode()
	case controlEnvironment:
		m.cycleEnvironment()
	}
}

// cycleMode advances the two-stage mode none -> plan -> apply -> none.
func (m *formModel) cycleMode() {
	switch m.mode {
	case modeNone:
		m.mode = modePlan
	case modePlan:
		m.mode = modeApply
	default:
		m.mode = modeNone
	}
}

// cycleEnvironment advances the environment selection, wrapping past the
// last configured environment back to envNone.
func (m *formModel) cycleEnvironment() {
	if len(m.envChoices) == 0 {
		return
	}

	m.envIdx = (m.envIdx + 1) % len(m.envChoices)
}

// selectedEnvironment returns the currently picked environment name, or
// "" when envNone is selected so the request omits the field.
func (m *formModel) selectedEnvironment() string {
	if m.envIdx <= 0 || m.envIdx >= len(m.envChoices) {
		return ""
	}

	return m.envChoices[m.envIdx]
}

// safetyFlags assembles the SafetyFlags the form's controls represent.
// dry-run and confirm are written only when toggled on (a tri-state
// pointer left nil keeps the field off the request); mode is written only
// when not "none"; environment only when a real one is picked. This
// mirrors Phase 1's flag folding exactly.
func (m *formModel) safetyFlags() SafetyFlags {
	var flags SafetyFlags

	if m.dryRun {
		on := true
		flags.DryRun = &on
	}

	if m.confirm {
		on := true
		flags.Confirm = &on
	}

	if m.mode != modeNone {
		flags.Mode = m.mode
	}

	flags.Environment = m.selectedEnvironment()

	return flags
}

// fieldPairs returns the filled fields as key=value strings, skipping
// empty inputs so an untouched optional field stays off the request. The
// values are raw strings; buildFormRequest runs them through the same
// BuildArguments coercion Phase 1's `call` uses, so the schema types apply
// identically.
func (m *formModel) fieldPairs() []string {
	pairs := make([]string, 0, len(m.fields))

	for idx := range m.fields {
		value := strings.TrimSpace(m.fields[idx].input.Value())
		if value == "" {
			continue
		}

		pairs = append(pairs, m.fields[idx].spec.Name+"="+value)
	}

	return pairs
}

// buildRequest turns the form state into the tool name and the final
// arguments map for a tools/call, reusing the exact Phase 1 helpers:
// BuildArguments coerces the field values by schema type, then
// ApplySafetyFlags folds in the safety controls. The result is identical
// to what `linodemcp call <tool> --arg ... --dry-run ...` would build, so
// the TUI and the CLI cannot drift in how a form becomes a request.
func (m *formModel) buildRequest() (string, map[string]any, error) {
	args, err := BuildArguments(m.schema, "", m.fieldPairs())
	if err != nil {
		return "", nil, err
	}

	flags := m.safetyFlags()
	ApplySafetyFlags(args, &flags)

	return m.tool, args, nil
}

// update routes a key message to the focused text input (when focus is on
// a field) so typing edits the right value, and returns any command the
// input produced. Navigation and control toggles are handled by the
// parent before this is called.
func (m *formModel) update(msg tea.Msg) tea.Cmd {
	if m.focusOnControls() || len(m.fields) == 0 {
		return nil
	}

	var cmd tea.Cmd

	m.fields[m.focus].input, cmd = m.fields[m.focus].input.Update(msg)

	return cmd
}
