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
