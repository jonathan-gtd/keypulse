package input

import (
	"testing"

	"github.com/jonathan-gtd/keypulse/internal/stats"
)

func TestKeyID(t *testing.T) {
	tests := []struct {
		vk, scan uint16
		e0       bool
		want     string
	}{
		{0x41, 0x1E, false, "A"},
		{0x31, 0x02, false, "1"},
		{vkShift, 0x2A, false, "LSHIFT"},
		{vkShift, scanRightShift, false, "RSHIFT"},
		{vkControl, 0x1D, false, "LCTRL"},
		{vkControl, 0x1D, true, "RCTRL"},
		{vkMenu, 0x38, true, "RALT"},
		{vkReturn, 0x1C, false, "ENTER"},
		{vkReturn, 0x1C, true, "NUMPAD_ENTER"},
		{0x63, 0x51, false, "NUMPAD3"},
		{0x7B, 0x58, false, "F12"},
		{0x20, 0x39, false, "SPACE"},
		{0xE7, 0x00, false, "VK_E7"},
	}
	for _, tt := range tests {
		if got := keyNames[keyID(tt.vk, tt.scan, tt.e0)]; got != tt.want {
			t.Errorf("keyNames[keyID(%#x, %#x, %v)] = %q, want %q", tt.vk, tt.scan, tt.e0, got, tt.want)
		}
	}
}

func TestCategory(t *testing.T) {
	tests := map[int]stats.Category{
		'A': stats.CatLetter, 'Z': stats.CatLetter,
		'0': stats.CatDigit, '9': stats.CatDigit,
		0x20: stats.CatPunctuation, 0xBA: stats.CatPunctuation, 0xE2: stats.CatPunctuation,
		0x25: stats.CatNavigation, 0x08: stats.CatNavigation, 0x0D: stats.CatNavigation,
		0x70: stats.CatFunction, 0x87: stats.CatFunction,
		0xA0: stats.CatModifier, 0x5B: stats.CatModifier, 0x14: stats.CatModifier,
		0x60: stats.CatNumpad, 0x90: stats.CatNumpad, numpadEnter: stats.CatNumpad,
		0xAF: stats.CatMedia,
		0x07: stats.CatOther,
	}
	for id, want := range tests {
		if got := category(id); got != want {
			t.Errorf("category(%#x) = %q, want %q",
				id, stats.CategoryNames[got], stats.CategoryNames[want])
		}
	}
}

func TestHandFor(t *testing.T) {
	tests := []struct {
		name string
		scan uint16
		e0   bool
		want stats.Hand
	}{
		{"the key left of 1", 0x29, false, stats.HandNone},
		{"digit 4", 0x05, false, stats.HandLeft},
		{"digit 7", 0x08, false, stats.HandRight},
		{"top row, left of centre", 0x11, false, stats.HandLeft},
		{"top row, right of centre", 0x16, false, stats.HandRight},
		{"home row, left", 0x1E, false, stats.HandLeft},
		{"home row, right", 0x24, false, stats.HandRight},
		{"left shift", 0x2A, false, stats.HandLeft},
		{"right shift", 0x36, false, stats.HandRight},
		{"left ctrl", 0x1D, false, stats.HandLeft},
		{"right ctrl", 0x1D, true, stats.HandRight},
		{"left alt", 0x38, false, stats.HandLeft},
		{"right alt", 0x38, true, stats.HandRight},
		{"enter", 0x1C, false, stats.HandRight},
		{"numeric pad enter", 0x1C, true, stats.HandNone},
		{"space belongs to neither", 0x39, false, stats.HandNone},
		{"numeric pad 7", 0x47, false, stats.HandNone},
		{"arrow key", 0x48, true, stats.HandNone},
		{"F3", 0x3D, false, stats.HandLeft},
		{"F9", 0x43, false, stats.HandRight},
	}
	for _, tt := range tests {
		if got := handFor(tt.scan, tt.e0); got != tt.want {
			t.Errorf("%s: handFor(%#x, %v) = %d, want %d", tt.name, tt.scan, tt.e0, got, tt.want)
		}
	}
}

func TestShortcutName(t *testing.T) {
	down := func(ids ...int) func(int) bool {
		set := map[int]bool{}
		for _, id := range ids {
			set[id] = true
		}
		return func(k int) bool { return set[k] }
	}
	const (
		lctrl, rctrl = 0xA2, 0xA3
		lalt         = 0xA4
		lwin         = 0x5B
		lshift       = 0xA0
	)
	tests := []struct {
		name string
		id   int
		held func(int) bool
		want string
	}{
		{"copy", 'C', down(lctrl), "CTRL+C"},
		{"right ctrl is the same combo", 'V', down(rctrl), "CTRL+V"},
		{"task manager", 0x1B, down(lctrl, lshift), "CTRL+SHIFT+ESC"},
		{"alt tab", 0x09, down(lalt), "ALT+TAB"},
		{"win d", 'D', down(lwin), "WIN+D"},
		{"order is fixed", 'S', down(lshift, lalt, lctrl), "CTRL+ALT+SHIFT+S"},
		{"shift alone is typing", 'A', down(lshift), ""},
		{"no modifier", 'A', down(), ""},
		{"a modifier is not a shortcut", lctrl, down(lctrl), ""},
	}
	for _, tt := range tests {
		if got := shortcutName(tt.id, tt.held); got != tt.want {
			t.Errorf("%s: shortcutName = %q, want %q", tt.name, got, tt.want)
		}
	}
}
