package tui

import (
	"reflect"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func key(s string) tea.KeyMsg {
	switch s {
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func drive(m MultiSelectModel, keys ...string) MultiSelectModel {
	for _, k := range keys {
		nm, _ := m.Update(key(k))
		m = nm.(MultiSelectModel)
	}
	return m
}

func TestMultiSelectToggleAndConfirm(t *testing.T) {
	m := NewMultiSelect("Pick", []string{"a", "b", "c"}, nil)
	// select index 0, move to index 2 and select it, confirm.
	m = drive(m, " ", "down", "down", " ", "enter")
	if !m.done || m.Canceled() {
		t.Fatalf("enter should confirm, not cancel")
	}
	if got := m.Selected(); !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("selected = %v, want [0 2]", got)
	}
}

func TestMultiSelectToggleAll(t *testing.T) {
	m := NewMultiSelect("Pick", []string{"a", "b"}, nil)
	m = drive(m, "a") // select all
	if got := m.Selected(); !reflect.DeepEqual(got, []int{0, 1}) {
		t.Fatalf("a should select all, got %v", got)
	}
	m = drive(m, "a") // clear all
	if got := m.Selected(); len(got) != 0 {
		t.Fatalf("a again should clear, got %v", got)
	}
}

func TestMultiSelectCancel(t *testing.T) {
	m := NewMultiSelect("Pick", []string{"a", "b"}, nil)
	m = drive(m, " ", "esc")
	if !m.Canceled() {
		t.Fatalf("esc should cancel")
	}
}

func TestMultiSelectCursorBounds(t *testing.T) {
	m := NewMultiSelect("Pick", []string{"a", "b"}, nil)
	m = drive(m, "up", "up")             // cannot go below 0
	m = drive(m, "down", "down", "down") // cannot go past last
	m = drive(m, " ", "enter")
	if got := m.Selected(); !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("cursor should clamp to last item, got %v", got)
	}
}

// TestMultiSelectToggleAllAfterDeselect covers the stale-entry bug: toggling an
// item off leaves a false value in the map, so counting entries instead of
// selections made "a" clear the list when it should have selected everything.
func TestMultiSelectToggleAllAfterDeselect(t *testing.T) {
	var m tea.Model = NewMultiSelect("pick", []string{"a", "b", "c"}, nil)
	// Turn every item on, then off again: three map entries, none selected.
	for _, k := range []string{" ", "down", " ", "down", " "} {
		m, _ = m.Update(key(k))
	}
	for _, k := range []string{" ", "up", " ", "up", " "} {
		m, _ = m.Update(key(k))
	}
	if got := m.(MultiSelectModel).Selected(); len(got) != 0 {
		t.Fatalf("precondition failed: %v still selected", got)
	}
	m, _ = m.Update(key("a"))
	if got := m.(MultiSelectModel).Selected(); !reflect.DeepEqual(got, []int{0, 1, 2}) {
		t.Errorf("'a' selected %v, want every item", got)
	}
	// Pressing it again with everything on clears the selection.
	m, _ = m.Update(key("a"))
	if got := m.(MultiSelectModel).Selected(); len(got) != 0 {
		t.Errorf("'a' on a full selection left %v", got)
	}
}
