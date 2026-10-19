package input

import "fmt"

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

// keyNames is filled once so that a key press never allocates.
var keyNames = func() [numpadEnter + 1]string {
	var names [numpadEnter + 1]string
	for i := range names {
		names[i] = keyName(i)
	}
	return names
}()
