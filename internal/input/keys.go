package input

import (
	"fmt"
	"strings"

	"github.com/jonathan-gtd/keypulse/internal/stats"
)

// numpadEnter is a pseudo key id: Windows reports both Enter keys as VK_RETURN.
const numpadEnter = 256

const (
	vkShift   = 0x10
	vkControl = 0x11
	vkMenu    = 0x12
	vkReturn  = 0x0D

	scanRightShift = 0x36
)

var vkNames = map[uint16]string{
	0x08: "BACKSPACE", 0x09: "TAB", 0x0C: "CLEAR", 0x0D: "ENTER", 0x13: "PAUSE",
	0x14: "CAPSLOCK", 0x1B: "ESC", 0x20: "SPACE", 0x21: "PAGEUP", 0x22: "PAGEDOWN",
	0x23: "END", 0x24: "HOME", 0x25: "LEFT", 0x26: "UP", 0x27: "RIGHT", 0x28: "DOWN",
	0x2C: "PRINTSCREEN", 0x2D: "INSERT", 0x2E: "DELETE",
	0x5B: "LWIN", 0x5C: "RWIN", 0x5D: "MENU",
	0x6A: "NUMPAD_MULTIPLY", 0x6B: "NUMPAD_ADD", 0x6C: "NUMPAD_SEPARATOR",
	0x6D: "NUMPAD_SUBTRACT", 0x6E: "NUMPAD_DECIMAL", 0x6F: "NUMPAD_DIVIDE",
	0x90: "NUMLOCK", 0x91: "SCROLLLOCK",
	0xA0: "LSHIFT", 0xA1: "RSHIFT", 0xA2: "LCTRL", 0xA3: "RCTRL", 0xA4: "LALT", 0xA5: "RALT",
	0xAD: "VOLUME_MUTE", 0xAE: "VOLUME_DOWN", 0xAF: "VOLUME_UP",
	0xB0: "MEDIA_NEXT", 0xB1: "MEDIA_PREV", 0xB2: "MEDIA_STOP", 0xB3: "MEDIA_PLAY_PAUSE",
	0xBA: "OEM_1", 0xBB: "OEM_PLUS", 0xBC: "OEM_COMMA", 0xBD: "OEM_MINUS", 0xBE: "OEM_PERIOD",
	0xBF: "OEM_2", 0xC0: "OEM_3", 0xDB: "OEM_4", 0xDC: "OEM_5", 0xDD: "OEM_6", 0xDE: "OEM_7",
	0xDF: "OEM_8", 0xE2: "OEM_102",
}

// keyID turns a raw keyboard event into a stable key id: left and right
// modifiers are told apart, and so are the two Enter keys.
func keyID(vk, makeCode uint16, e0 bool) int {
	switch vk {
	case vkShift:
		if makeCode == scanRightShift {
			return 0xA1
		}
		return 0xA0
	case vkControl:
		if e0 {
			return 0xA3
		}
		return 0xA2
	case vkMenu:
		if e0 {
			return 0xA5
		}
		return 0xA4
	case vkReturn:
		if e0 {
			return numpadEnter
		}
	}
	return int(vk)
}

// keyName returns a readable name for a key id. Letters follow the active
// keyboard layout, as Windows virtual-key codes do.
func keyName(id int) string {
	if id == numpadEnter {
		return "NUMPAD_ENTER"
	}
	vk := uint16(id)
	switch {
	case vk >= 'A' && vk <= 'Z', vk >= '0' && vk <= '9':
		return string(rune(vk))
	case vk >= 0x60 && vk <= 0x69:
		return fmt.Sprintf("NUMPAD%d", vk-0x60)
	case vk >= 0x70 && vk <= 0x87:
		return fmt.Sprintf("F%d", vk-0x6F)
	}
	if name, ok := vkNames[vk]; ok {
		return name
	}
	return fmt.Sprintf("VK_%02X", vk)
}

// modifiers, in the order they appear in a combo name. CTRL, ALT and WIN
// make a shortcut; SHIFT only decorates one, since shift alone is typing.
var modifierKeys = []struct {
	name    string
	ids     [2]int
	inCombo bool
}{
	{"CTRL", [2]int{0xA2, 0xA3}, true},
	{"ALT", [2]int{0xA4, 0xA5}, true},
	{"WIN", [2]int{0x5B, 0x5C}, true},
	{"SHIFT", [2]int{0xA0, 0xA1}, false},
}

// shortcutName names the combination being pressed, e.g. "CTRL+SHIFT+ESC".
// It returns "" when id is itself a modifier, or when no CTRL, ALT or WIN is
// held. Only the combination is named: no sequence of keys is ever kept.
func shortcutName(id int, isDown func(int) bool) string {
	var parts []string
	real := false
	for _, m := range modifierKeys {
		if id == m.ids[0] || id == m.ids[1] {
			return ""
		}
		if isDown(m.ids[0]) || isDown(m.ids[1]) {
			parts = append(parts, m.name)
			real = real || m.inCombo
		}
	}
	if !real || id < 0 || id >= len(keyNames) {
		return ""
	}
	return strings.Join(append(parts, keyNames[id]), "+")
}

// category groups a key by what it is for, from its virtual-key code.
func category(id int) stats.Category {
	if id == numpadEnter {
		return stats.CatNumpad
	}
	vk := uint16(id)
	switch {
	case vk >= 'A' && vk <= 'Z':
		return stats.CatLetter
	case vk >= '0' && vk <= '9':
		return stats.CatDigit
	case vk >= 0x60 && vk <= 0x6F, vk == 0x90:
		return stats.CatNumpad
	case vk >= 0x70 && vk <= 0x87:
		return stats.CatFunction
	case vk >= 0xA0 && vk <= 0xA5, vk == 0x5B, vk == 0x5C, vk == 0x14:
		return stats.CatModifier
	case vk >= 0xAD && vk <= 0xB3:
		return stats.CatMedia
	case vk >= 0x21 && vk <= 0x28, vk == 0x2D, vk == 0x2E,
		vk == 0x08, vk == 0x09, vk == 0x0D, vk == 0x1B:
		return stats.CatNavigation
	case vk == 0x20, vk >= 0xBA && vk <= 0xDF, vk == 0xE2:
		// Space counts here too: it separates words rather than forming them.
		return stats.CatPunctuation
	}
	return stats.CatOther
}

// Scan codes of the main block, by side of the keyboard. Scan codes are
// physical positions, so the split holds whatever the layout is — on AZERTY
// the key that types A sits where QWERTY has Q, and both are on the left.
var (
	leftScan = map[uint16]bool{
		0x01: true, 0x02: true, 0x03: true, 0x04: true, 0x05: true, 0x06: true, // esc 1-5
		0x0F: true, 0x10: true, 0x11: true, 0x12: true, 0x13: true, 0x14: true, // tab qwert
		0x3A: true, 0x1E: true, 0x1F: true, 0x20: true, 0x21: true, 0x22: true, // caps asdfg
		0x2A: true, 0x56: true, 0x2C: true, 0x2D: true, 0x2E: true, 0x2F: true, 0x30: true, // lshift <zxcvb
		0x3B: true, 0x3C: true, 0x3D: true, 0x3E: true, 0x3F: true, 0x40: true, // F1-F6
	}
	rightScan = map[uint16]bool{
		0x07: true, 0x08: true, 0x09: true, 0x0A: true, 0x0B: true, 0x0C: true, 0x0D: true, 0x0E: true,
		0x15: true, 0x16: true, 0x17: true, 0x18: true, 0x19: true, 0x1A: true, 0x1B: true, 0x2B: true,
		0x23: true, 0x24: true, 0x25: true, 0x26: true, 0x27: true, 0x28: true,
		0x31: true, 0x32: true, 0x33: true, 0x34: true, 0x35: true, 0x36: true,
		0x41: true, 0x42: true, 0x43: true, 0x44: true, 0x57: true, 0x58: true, // F7-F12
	}
)

// handFor says which side of the keyboard a key sits on. Keys in the middle
// or off to the side — space, the numeric pad, the navigation cluster —
// belong to neither.
func handFor(makeCode uint16, e0 bool) stats.Hand {
	switch makeCode {
	case 0x1D, 0x38: // ctrl and alt: the right-hand ones are escaped
		if e0 {
			return stats.HandRight
		}
		return stats.HandLeft
	case 0x1C: // enter, unless it is the one on the numeric pad
		if e0 {
			return stats.HandNone
		}
		return stats.HandRight
	case 0x5B:
		return stats.HandLeft
	case 0x5C, 0x5D:
		return stats.HandRight
	}
	if e0 { // the navigation cluster and the media keys
		return stats.HandNone
	}
	switch {
	case leftScan[makeCode]:
		return stats.HandLeft
	case rightScan[makeCode]:
		return stats.HandRight
	}
	return stats.HandNone
}

// keyNames is filled once so that a key press never allocates.
var keyNames = func() [numpadEnter + 1]string {
	var names [numpadEnter + 1]string
	for i := range names {
		names[i] = keyName(i)
	}
	return names
}()
