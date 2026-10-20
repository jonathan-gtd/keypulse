package input

import "testing"

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
