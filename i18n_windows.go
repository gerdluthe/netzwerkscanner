package main

import (
	"strings"

	"netzscan/third_party/walk"
)

func platformLanguageHook() {
	if lang == "de" {
		walk.ShortcutTranslator = func(s string) string {
			return strings.NewReplacer("Ctrl", "Strg", "Shift", "Umschalt", "Delete", "Entf", "Return", "Eingabe").Replace(s)
		}
	} else {
		walk.ShortcutTranslator = nil
	}
}
