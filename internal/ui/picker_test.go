package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

func pickerTestItems() []Item {
	return []Item{
		{ItemTitle: "First", URL: "https://example.com/first", Hostname: "example.com"},
		{
			ItemTitle:    "Second",
			URL:          "https://example.com/second",
			Hostname:     "example.com",
			SecondaryURL: "https://markist.xyz/shared/abc",
		},
	}
}

func runPicker(t *testing.T, m PickerModel) *teatest.TestModel {
	t.Helper()
	return teatest.NewTestModel(t, m, teatest.WithInitialTermSize(80, 24))
}

func finalPickerModel(t *testing.T, tm *teatest.TestModel) PickerModel {
	t.Helper()
	final := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second))
	pm, ok := final.(PickerModel)
	if !ok {
		t.Fatalf("final model is %T, want PickerModel", final)
	}
	return pm
}

func TestPickerEnterOpensSelectedAndQuits(t *testing.T) {
	m := NewPickerModel("Results", pickerTestItems(), false, nil, nil)
	tm := runPicker(t, m)

	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	final := finalPickerModel(t, tm)
	if final.Opened == nil {
		t.Fatal("Opened = nil, want the first item")
	}
	if final.Opened.URL != "https://example.com/first" {
		t.Fatalf("Opened.URL = %q", final.Opened.URL)
	}
	if final.OpenedSecondary {
		t.Fatal("OpenedSecondary = true, want false")
	}
}

func TestPickerDownThenEnterOpensSecondItem(t *testing.T) {
	m := NewPickerModel("Results", pickerTestItems(), false, nil, nil)
	tm := runPicker(t, m)

	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	final := finalPickerModel(t, tm)
	if final.Opened == nil || final.Opened.URL != "https://example.com/second" {
		t.Fatalf("Opened = %+v, want second item", final.Opened)
	}
}

func TestPickerJKNavigateLikeArrows(t *testing.T) {
	m := NewPickerModel("Results", pickerTestItems(), false, nil, nil)
	tm := runPicker(t, m)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	tm.Send(tea.KeyMsg{Type: tea.KeyEnter})

	final := finalPickerModel(t, tm)
	if final.Opened == nil || final.Opened.URL != "https://example.com/second" {
		t.Fatalf("Opened = %+v, want second item via j", final.Opened)
	}
}

func TestPickerQuitWithoutSelecting(t *testing.T) {
	m := NewPickerModel("Results", pickerTestItems(), false, nil, nil)
	tm := runPicker(t, m)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	final := finalPickerModel(t, tm)
	if final.Opened != nil {
		t.Fatalf("Opened = %+v, want nil", final.Opened)
	}
}

func TestPickerStayKeyCallsOpenFuncWithoutQuitting(t *testing.T) {
	var openedURL string
	openFunc := func(u string) error { openedURL = u; return nil }
	m := NewPickerModel("Results", pickerTestItems(), false, openFunc, nil)
	tm := runPicker(t, m)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	final := finalPickerModel(t, tm)
	if final.Opened != nil {
		t.Fatalf("Opened = %+v, want nil ('o' stays running)", final.Opened)
	}
	if openedURL != "https://example.com/first" {
		t.Fatalf("openFunc called with %q, want the first item's URL", openedURL)
	}
}

func TestPickerCopyKeyCallsCopyFunc(t *testing.T) {
	var copied string
	copyFunc := func(text string) error { copied = text; return nil }
	m := NewPickerModel("Results", pickerTestItems(), false, nil, copyFunc)
	tm := runPicker(t, m)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	finalPickerModel(t, tm)
	if copied != "https://example.com/first" {
		t.Fatalf("copyFunc called with %q, want the first item's URL", copied)
	}
}

func TestPickerSharedOpenAltKeyOpensSecondaryURL(t *testing.T) {
	m := NewPickerModel("Shared", pickerTestItems(), true, nil, nil)
	tm := runPicker(t, m)

	tm.Send(tea.KeyMsg{Type: tea.KeyDown})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})

	final := finalPickerModel(t, tm)
	if final.Opened == nil || !final.OpenedSecondary {
		t.Fatalf("Opened = %+v, OpenedSecondary = %v, want second item opened via secondary", final.Opened, final.OpenedSecondary)
	}
	if final.Opened.SecondaryURL != "https://markist.xyz/shared/abc" {
		t.Fatalf("Opened.SecondaryURL = %q", final.Opened.SecondaryURL)
	}
}

func TestPickerOpenAltKeyIgnoredWhenNotShared(t *testing.T) {
	m := NewPickerModel("Results", pickerTestItems(), false, nil, nil)
	tm := runPicker(t, m)

	// "s" has no meaning outside `shared`; it must not open or quit.
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	final := finalPickerModel(t, tm)
	if final.Opened != nil {
		t.Fatalf("Opened = %+v, want nil ('s' is not bound outside shared)", final.Opened)
	}
}

func TestPickerFilterConsumesKeysInsteadOfTriggeringActions(t *testing.T) {
	var openedURL string
	openFunc := func(u string) error { openedURL = u; return nil }
	m := NewPickerModel("Results", pickerTestItems(), false, openFunc, nil)
	tm := runPicker(t, m)

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'/'}}) // start filtering
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}}) // typed into the filter box
	tm.Send(tea.KeyMsg{Type: tea.KeyEscape})                    // cancel filtering
	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	finalPickerModel(t, tm)
	if openedURL != "" {
		t.Fatalf("openFunc called with %q while filtering, want not called", openedURL)
	}
}
