package theme

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
)

type CustomDarkTheme struct {
	fyne.Theme
}

func NewCustomDarkTheme() fyne.Theme {
	return &CustomDarkTheme{theme.DarkTheme()}
}

func (t *CustomDarkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if name == theme.ColorNameBackground {
		return color.Black
	}
	if name == theme.ColorNameButton {
		return color.Black
	}
	return t.Theme.Color(name, variant)
}
