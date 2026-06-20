package cli

import (
	"sort"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/mark3labs/mcp-go/mcp"

	"github.com/chadit/LinodeMCP/go/internal/profiles"
	"github.com/chadit/LinodeMCP/go/internal/server"
)

// catalogUncategorized labels tools whose name matches no category prefix
// so they still appear in the catalog rather than vanishing.
const catalogUncategorized = "other"

// toolMeta holds the per-tool data the catalog and form need, shared by
// pointer so a catalogItem stays small enough to pass by value (which the
// bubbles list.Item interface requires). The schema is the heavy field;
// keeping it behind a pointer keeps each list item cheap to copy.
type toolMeta struct {
	name       string
	capability profiles.Capability
	schema     mcp.ToolInputSchema
}

// catalogItem is one tool row in the catalog list. It pairs a tool's
// shared metadata with the single category this row represents, and
// satisfies the bubbles list.Item interface so the list widget can render
// and filter it. Only two strings and a pointer wide, so the value
// receivers the interface forces stay cheap.
type catalogItem struct {
	meta     *toolMeta
	category string
}

// Title is the primary line the list delegate renders.
func (i catalogItem) Title() string { return i.meta.name }

// Description is the secondary line: the category and capability tag.
func (i catalogItem) Description() string {
	return i.category + "  " + i.meta.capability.String()
}

// FilterValue is the string the list's fuzzy filter matches against. It
// folds in the category and capability so a search for "destroy" or
// "networking" narrows the list, not only a tool-name search.
func (i catalogItem) FilterValue() string {
	return i.meta.name + " " + i.category + " " + i.meta.capability.String()
}

// CatalogEntry is the framework-agnostic description of one catalog row: a
// tool under one of its categories, with its capability. It carries no
// Bubble Tea types, so the grouping logic that produces it is testable
// without standing up the list widget.
type CatalogEntry struct {
	Name       string
	Category   string
	Capability profiles.Capability
}

// CatalogEntries turns a slice of server ToolInfos into sorted catalog
// entries, one per (tool, category) pair. A tool in several categories
// appears once per category so it shows up under each, matching how the
// profile-category mapping treats multi-category tools. Tools whose name
// matches no category land in the "other" bucket. Sorted by category then
// name so the list is stable and grouped.
//
// Pure and framework-free: it takes the ToolInfo slice the caller pulled
// from the server and returns plain structs, so the grouping is
// unit-testable without a running program or the bubbles list.
func CatalogEntries(infos []server.ToolInfo) []CatalogEntry {
	entries := make([]CatalogEntry, 0, len(infos))

	for idx := range infos {
		for _, category := range categoriesFor(infos[idx].Name) {
			entries = append(entries, CatalogEntry{
				Name:       infos[idx].Name,
				Category:   category,
				Capability: infos[idx].Capability,
			})
		}
	}

	sort.Slice(entries, func(left, right int) bool {
		if entries[left].Category != entries[right].Category {
			return entries[left].Category < entries[right].Category
		}

		return entries[left].Name < entries[right].Name
	})

	return entries
}

// buildCatalogItems builds the bubbles list items from the sorted catalog
// entries, sharing one toolMeta per tool so each item stays small. The
// order matches CatalogEntries, so the list is grouped by category.
func buildCatalogItems(infos []server.ToolInfo) []list.Item {
	metas := make(map[string]*toolMeta, len(infos))
	for idx := range infos {
		metas[infos[idx].Name] = &toolMeta{
			name:       infos[idx].Name,
			capability: infos[idx].Capability,
			schema:     infos[idx].InputSchema,
		}
	}

	entries := CatalogEntries(infos)
	items := make([]list.Item, 0, len(entries))

	for idx := range entries {
		items = append(items, catalogItem{
			meta:     metas[entries[idx].Name],
			category: entries[idx].Category,
		})
	}

	return items
}

// categoriesFor returns the categories a tool belongs to, substituting a
// single "other" bucket when the profile mapping returns none so every
// tool is reachable in the catalog.
func categoriesFor(toolName string) []string {
	cats := profiles.Categories(toolName)
	if len(cats) == 0 {
		return []string{catalogUncategorized}
	}

	return cats
}

// catalogModel is the catalog screen: a filterable list of tools grouped
// by category, scoped to the active profile by default with a key to
// reveal the full surface. It owns a bubbles list widget and the two
// ToolInfo slices (profile-scoped and full) so the scope toggle is a
// local swap with no re-fetch.
type catalogModel struct {
	list        list.Model
	profileable []server.ToolInfo
	all         []server.ToolInfo
	showingAll  bool
	width       int
	height      int
}

// newCatalogModel builds the catalog from the server's two views. The
// list starts scoped to the active profile (profileable); toggleScope
// swaps in the full catalog. The delegate is the default two-line item
// renderer; filtering is the list widget's built-in fuzzy match.
func newCatalogModel(srv *server.Server) catalogModel {
	profileable := srv.ToolInfos()
	all := srv.AllToolInfos()

	delegate := list.NewDefaultDelegate()
	toolList := list.New(buildCatalogItems(profileable), delegate, 0, 0)
	toolList.Title = "Tools (active profile)"
	toolList.SetShowHelp(true)

	return catalogModel{
		list:        toolList,
		profileable: profileable,
		all:         all,
	}
}

// setSize lays the list out to the available terminal size, reserving
// rows for the surrounding chrome the parent model draws.
func (m *catalogModel) setSize(width, height int) {
	m.width = width
	m.height = height
	m.list.SetSize(width, height)
}

// toggleScope flips between the active-profile surface and the full
// catalog, rebuilding the list items and the title. The filter, if any,
// is preserved by the list widget across SetItems.
func (m *catalogModel) toggleScope() {
	m.showingAll = !m.showingAll

	if m.showingAll {
		m.list.Title = "Tools (full catalog)"
		m.list.SetItems(buildCatalogItems(m.all))

		return
	}

	m.list.Title = "Tools (active profile)"
	m.list.SetItems(buildCatalogItems(m.profileable))
}

// selected returns the highlighted tool and true, or a zero item and
// false when the list is empty (every tool filtered out). The parent uses
// this to decide whether the select key opens the form.
func (m *catalogModel) selected() (catalogItem, bool) {
	item, ok := m.list.SelectedItem().(catalogItem)

	return item, ok
}

// filtering reports whether the list's text filter is active, so the
// parent can let the list consume keystrokes (including the select key)
// rather than treating them as navigation.
func (m *catalogModel) filtering() bool {
	return m.list.FilterState() == list.Filtering
}

// update forwards a message to the embedded list and returns any command
// the list produced (filter cursor blink, etc.).
func (m *catalogModel) update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd

	m.list, cmd = m.list.Update(msg)

	return cmd
}

// view renders the catalog list.
func (m *catalogModel) view() string {
	return m.list.View()
}
