package cli

import (
	"fmt"
	"io"
	"strconv"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/chadit/LinodeMCP/go/internal/config"
	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// ProfileEntry is the framework-agnostic projection of one selectable
// profile for the switcher: its name, whether it is the active one, and a
// one-line summary. Built from the config, so the list construction and
// active marking (the part worth testing) need no Bubble Tea widgets.
type ProfileEntry struct {
	Name     string
	Active   bool
	Disabled bool
	Summary  string
}

// ProfileListEntries builds the sorted list of selectable profiles from a
// config: every built-in plus user-defined profile, with the active one
// marked. Reuses AllProfiles and ResolveActiveName (the same catalog the
// CLI `profile list` reads) so the TUI and the CLI agree on the set and
// the active marking. A nil config yields no entries.
//
// Exported and pure: it reads only the config, so the construction and
// active-marking logic is unit-testable without a server.
func ProfileListEntries(cfg *config.Config) []ProfileEntry {
	if cfg == nil {
		return nil
	}

	all := AllProfiles(cfg)
	active := ResolveActiveName(cfg)

	entries := make([]ProfileEntry, 0, len(all))

	for _, name := range sortedNames(all) {
		prof := all[name]
		entries = append(entries, ProfileEntry{
			Name:     name,
			Active:   name == active,
			Disabled: prof.Disabled,
			Summary:  profileSummary(&prof),
		})
	}

	return entries
}

// profileSummary builds the one-line description shown beside a profile:
// its tool count and yolo flag, so a user can tell the profiles apart
// without opening each.
func profileSummary(prof *profiles.Profile) string {
	yolo := "yolo off"
	if prof.AllowYolo {
		yolo = "yolo ON"
	}

	return strconv.Itoa(len(prof.AllowedTools)) + " tools, " + yolo
}

// profileItem is one profile row in the switcher list. It carries the
// entry data and satisfies the bubbles list.Item interface. Small (a
// pointer plus the embedded value would be heavy, so the entry is held by
// value but kept to a few fields) to keep the value-receiver methods the
// interface forces cheap.
type profileItem struct {
	entry ProfileEntry
}

// Title is the primary list line: the profile name, with an active marker.
func (i profileItem) Title() string {
	if i.entry.Active {
		return "* " + i.entry.Name + " (active)"
	}

	return "  " + i.entry.Name
}

// Description is the secondary line: the summary plus a disabled note.
func (i profileItem) Description() string {
	if i.entry.Disabled {
		return i.entry.Summary + "  [disabled]"
	}

	return i.entry.Summary
}

// FilterValue lets the list filter match on the profile name.
func (i profileItem) FilterValue() string {
	return i.entry.Name
}

// profileSwitchedMsg signals a finished profile switch back into the
// update loop: the new active name and whether the write succeeded. On
// success the parent reloads the server profile and rebuilds the catalog.
type profileSwitchedMsg struct {
	name string
	err  error
}

// profileModel is the profile switcher screen: a list of profiles with the
// active one marked, and an action to switch the active profile. Switching
// reuses RunProfileUse (the CLI's config-write path) rather than
// reimplementing the config mutation.
type profileModel struct {
	list       list.Model
	configPath string
	status     string
	width      int
	height     int
}

// newProfileModel builds the switcher from a config. configPath is the
// file the switch writes to; an empty string falls back to the standard
// config path inside RunProfileUse, matching the CLI default.
func newProfileModel(cfg *config.Config, configPath string) profileModel {
	entries := ProfileListEntries(cfg)
	items := make([]list.Item, 0, len(entries))

	for idx := range entries {
		items = append(items, profileItem{entry: entries[idx]})
	}

	delegate := list.NewDefaultDelegate()
	profileList := list.New(items, delegate, 0, 0)
	profileList.Title = "Profiles"
	profileList.SetShowHelp(true)

	return profileModel{list: profileList, configPath: configPath}
}

// setSize lays the list out within the available space.
func (m *profileModel) setSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height)
}

// filtering reports whether the list's text filter is open, so the parent
// defers the select key to the filter.
func (m *profileModel) filtering() bool {
	return m.list.FilterState() == list.Filtering
}

// selectedName returns the highlighted profile name and true, or "" and
// false when the list is empty.
func (m *profileModel) selectedName() (string, bool) {
	item, ok := m.list.SelectedItem().(profileItem)
	if !ok {
		return "", false
	}

	return item.entry.Name, true
}

// switchCmd returns a tea.Cmd that writes the selected profile as active
// via RunProfileUse (the exact CLI config-write path) and reports the
// outcome as a profileSwitchedMsg. Output goes to io.Discard because the
// TUI surfaces the result through the status line, not stdout.
func (m *profileModel) switchCmd(name string) tea.Cmd {
	configPath := m.configPath

	return func() tea.Msg {
		code := RunProfileUse([]string{name}, configPath, io.Discard, io.Discard)
		if code != 0 {
			return profileSwitchedMsg{name: name, err: errProfileSwitchFailed}
		}

		return profileSwitchedMsg{name: name, err: nil}
	}
}

// rebuild refreshes the list items and active marking from a config, so
// the switcher reflects the new active profile after a successful switch.
func (m *profileModel) rebuild(cfg *config.Config) {
	entries := ProfileListEntries(cfg)
	items := make([]list.Item, 0, len(entries))

	for idx := range entries {
		items = append(items, profileItem{entry: entries[idx]})
	}

	m.list.SetItems(items)
}

// update forwards a message to the embedded list and returns any command.
func (m *profileModel) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	m.list, cmd = m.list.Update(msg)

	return cmd
}

// view renders the profile list plus the status line for the last switch.
func (m *profileModel) view() string {
	if m.status == "" {
		return m.list.View()
	}

	return m.list.View() + "\n" + m.status
}

// reloadServerProfile applies a config's active profile to the running
// server so the catalog's profile-filtered view updates in place. Returns
// the reloaded config on success so the caller can rebuild the catalog and
// the switcher from it; a reload error leaves the server unchanged.
func reloadServerProfile(srv *server.Server, configPath string) (*config.Config, error) {
	path := resolveConfigPath(configPath)

	cfg, err := config.Load(path)
	if err != nil {
		return nil, fmt.Errorf("reload config from %s: %w", path, err)
	}

	if reloadErr := srv.ReloadProfile(cfg); reloadErr != nil {
		return nil, fmt.Errorf("reload profile: %w", reloadErr)
	}

	return cfg, nil
}
