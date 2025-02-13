// Copyright 2025 Upbound Inc.
// All rights reserved

package ctx

import (
	"testing"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"gotest.tools/v3/assert"
)

func TestNewList_InitialCursorPlacement(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		items         []list.Item
		expectedIndex int
	}{
		"empty list": {
			items:         []list.Item{},
			expectedIndex: 0,
		},
		"all items selectable": {
			items: []list.Item{
				item{text: "item 1"},
				item{text: "item 2"},
				item{text: "item 3"},
			},
			expectedIndex: 0,
		},
		"top item selectable": {
			items: []list.Item{
				item{text: "item 1"},
				item{text: "item 2", notSelectable: true},
				item{text: "item 3", notSelectable: true},
			},
			expectedIndex: 0,
		},
		"top item unselectable": {
			items: []list.Item{
				item{text: "item 1", notSelectable: true},
				item{text: "item 2"},
				item{text: "item 3"},
			},
			expectedIndex: 1,
		},
		"top item back": {
			items: []list.Item{
				item{text: "item 1", back: true},
				item{text: "item 2"},
				item{text: "item 3"},
			},
			expectedIndex: 1,
		},
		"top two items undesirable": {
			items: []list.Item{
				item{text: "item 1", back: true},
				item{text: "item 2", notSelectable: true},
				item{text: "item 3"},
			},
			expectedIndex: 2,
		},
		"all items undesirable": {
			items: []list.Item{
				item{text: "item 1", back: true},
				item{text: "item 2", notSelectable: true},
				item{text: "item 3", notSelectable: true},
			},
			expectedIndex: 0,
		},
		"only item back": {
			items: []list.Item{
				item{text: "item 1", back: true},
			},
			expectedIndex: 0,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := model{
				list: newList(test.items),
			}

			assert.Equal(t, m.list.Index(), test.expectedIndex)
		})
	}
}

func TestMoveToSelectableItem(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		items         []list.Item
		startingIndex int
		key           rune
		expectedIndex int
	}{
		"empty list, down": {
			items:         []list.Item{},
			startingIndex: 0,
			key:           'j',
			expectedIndex: 0,
		},
		"empty list, up": {
			items:         []list.Item{},
			startingIndex: 0,
			key:           'k',
			expectedIndex: 0,
		},
		"selectable item, down": {
			items: []list.Item{
				item{text: "item 1"},
				item{text: "item 2"},
			},
			startingIndex: 0,
			key:           'j',
			expectedIndex: 0,
		},
		"selectable item, up": {
			items: []list.Item{
				item{text: "item 1"},
				item{text: "item 2"},
			},
			startingIndex: 1,
			key:           'k',
			expectedIndex: 1,
		},
		"unselectable item, down": {
			items: []list.Item{
				item{text: "item 1"},
				item{text: "item 2", notSelectable: true},
				item{text: "item 3"},
			},
			startingIndex: 1,
			key:           'j',
			expectedIndex: 2,
		},
		"unselectable item, up": {
			items: []list.Item{
				item{text: "item 1"},
				item{text: "item 2", notSelectable: true},
				item{text: "item 3"},
			},
			startingIndex: 1,
			key:           'k',
			expectedIndex: 0,
		},
		"multiple unselectable items, down": {
			items: []list.Item{
				item{text: "item 1"},
				item{text: "item 2", notSelectable: true},
				item{text: "item 2a", notSelectable: true},
				item{text: "item 3"},
			},
			startingIndex: 1,
			key:           'j',
			expectedIndex: 3,
		},
		"multiple unselectable items, up": {
			items: []list.Item{
				item{text: "item 1"},
				item{text: "item 2", notSelectable: true},
				item{text: "item 2a", notSelectable: true},
				item{text: "item 3"},
			},
			startingIndex: 2,
			key:           'k',
			expectedIndex: 0,
		},
		"unselectable item at bottom of list, down": {
			items: []list.Item{
				item{text: "item 1"},
				item{text: "item 2"},
				item{text: "item 3", notSelectable: true},
			},
			startingIndex: 2,
			key:           'j',
			expectedIndex: 1,
		},
		"unselectable item at top of list, up": {
			items: []list.Item{
				item{text: "item 1", notSelectable: true},
				item{text: "item 2"},
				item{text: "item 3"},
			},
			startingIndex: 0,
			key:           'k',
			expectedIndex: 1,
		},
		"all unselectable items, down": {
			items: []list.Item{
				item{text: "item 1", notSelectable: true},
				item{text: "item 2", notSelectable: true},
				item{text: "item 3", notSelectable: true},
			},
			startingIndex: 1,
			key:           'j',
			expectedIndex: 0,
		},
		"all unselectable items, up": {
			items: []list.Item{
				item{text: "item 1", notSelectable: true},
				item{text: "item 2", notSelectable: true},
				item{text: "item 3", notSelectable: true},
			},
			startingIndex: 1,
			key:           'k',
			expectedIndex: 2,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			m := model{
				list: newList(test.items),
			}
			m.list.Select(test.startingIndex)
			// Make sure Select() did what we want so our test is valid.
			assert.Equal(t, m.list.Index(), test.startingIndex)

			result := m.moveToSelectableItem(tea.KeyMsg{
				Type:  tea.KeyRunes,
				Runes: []rune{test.key},
			})

			assert.Equal(t, result.Index(), test.expectedIndex)
		})
	}
}
