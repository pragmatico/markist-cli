package ui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// Picker key bindings beyond what bubbles/list already provides
// (↑/↓/j/k navigate, "/" filters, "?" toggles help, q/esc quits -- all
// handled inside list.Model itself). Plan Task 13.
var (
	keyOpen    = key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "open & quit"))
	keyStay    = key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "open & stay"))
	keyCopy    = key.NewBinding(key.WithKeys("c"), key.WithHelp("c", "copy URL"))
	keyOpenAlt = key.NewBinding(key.WithKeys("s"), key.WithHelp("s", "open Markist page"))
)

// PickerModel is the Bubble Tea model for the interactive result picker used
// by search/list/shared (plan Task 13). It wraps bubbles/list for
// navigation, local fuzzy filtering and help, and layers open/copy behavior
// on top.
type PickerModel struct {
	list     list.Model
	shared   bool // enables the "s" (open SecondaryURL) binding, used by `shared`
	openFunc func(string) error
	copyFunc func(string) error

	// Opened is set when Enter or "s" ended the program by choosing a row;
	// nil means the user quit (q/esc/ctrl+c) without choosing one.
	Opened *Item
	// OpenedSecondary is true when "s" (not Enter) chose the row, meaning
	// the caller should open SecondaryURL instead of URL.
	OpenedSecondary bool
}

// NewPickerModel builds a picker over items. openFunc/copyFunc are seams
// (over browser.OpenURL / CopyToClipboard) so tests can stub them; shared
// enables the shared-only "s" binding and its help entry.
func NewPickerModel(title string, items []Item, shared bool, openFunc, copyFunc func(string) error) PickerModel {
	delegate := list.NewDefaultDelegate()
	listItems := make([]list.Item, len(items))
	for i, it := range items {
		listItems[i] = it
	}

	l := list.New(listItems, delegate, 0, 0)
	l.Title = title
	l.SetShowStatusBar(true)

	shortHelp := []key.Binding{keyOpen, keyStay, keyCopy}
	if shared {
		shortHelp = append(shortHelp, keyOpenAlt)
	}
	l.AdditionalShortHelpKeys = func() []key.Binding { return shortHelp }
	l.AdditionalFullHelpKeys = func() []key.Binding { return shortHelp }

	return PickerModel{list: l, shared: shared, openFunc: openFunc, copyFunc: copyFunc}
}

func (m PickerModel) Init() tea.Cmd { return nil }

func (m PickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.list.SetSize(msg.Width, msg.Height)
		return m, nil

	case tea.KeyMsg:
		// Custom bindings only apply while browsing -- while the user is
		// typing into the filter box, every keystroke must reach it (a "c"
		// typed into a filter query must not copy the URL).
		if !m.list.SettingFilter() {
			switch {
			case key.Matches(msg, keyOpen):
				if it, ok := m.list.SelectedItem().(Item); ok {
					m.Opened = &it
					return m, tea.Quit
				}
			case key.Matches(msg, keyStay):
				if it, ok := m.list.SelectedItem().(Item); ok && m.openFunc != nil {
					var cmd tea.Cmd
					if err := m.openFunc(it.URL); err != nil {
						cmd = m.list.NewStatusMessage("Couldn't open: " + err.Error())
					} else {
						cmd = m.list.NewStatusMessage("Opened " + it.URL)
					}
					return m, cmd
				}
			case key.Matches(msg, keyCopy):
				if it, ok := m.list.SelectedItem().(Item); ok && m.copyFunc != nil {
					var cmd tea.Cmd
					if err := m.copyFunc(it.URL); err != nil {
						cmd = m.list.NewStatusMessage("Couldn't copy: " + err.Error())
					} else {
						cmd = m.list.NewStatusMessage("Copied URL")
					}
					return m, cmd
				}
			case m.shared && key.Matches(msg, keyOpenAlt):
				if it, ok := m.list.SelectedItem().(Item); ok {
					m.Opened = &it
					m.OpenedSecondary = true
					return m, tea.Quit
				}
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m PickerModel) View() string {
	return m.list.View()
}
