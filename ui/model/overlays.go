package model

import (
	"cliamp/theme"
)

// openThemePicker re-loads themes from disk (picking up new user files)
// and opens the theme selector overlay.
func (m *Model) openThemePicker() {
	m.themes = theme.LoadAll()
	m.themePicker.visible = true
	m.themePicker.savedIdx = m.themeIdx
	// Position cursor on the currently active theme.
	// Picker list: 0 = Default, 1..N = themes[0..N-1]
	m.themePicker.cursor = m.themeIdx + 1
	m.themePicker.scroll = 0
	m.themePickerMaybeAdjustScroll(m.themePickerVisible())
}

// themePickerApply applies the theme under the cursor for live preview.
func (m *Model) themePickerApply() {
	if m.themePicker.cursor == 0 {
		m.themeIdx = -1
		applyThemeAll(theme.Default())
	} else {
		m.themeIdx = m.themePicker.cursor - 1
		applyThemeAll(m.themes[m.themeIdx])
	}
}

// themePickerSelect confirms the current selection and closes the picker.
func (m *Model) themePickerSelect() {
	m.themePickerApply()
	m.themePicker.visible = false
}

// themePickerCancel restores the theme from before the picker was opened.
func (m *Model) themePickerCancel() {
	m.themeIdx = m.themePicker.savedIdx
	if m.themeIdx < 0 {
		applyThemeAll(theme.Default())
	} else {
		applyThemeAll(m.themes[m.themeIdx])
	}
	m.themePicker.visible = false
}

func (m *Model) themePickerHelpLine() string {
	return helpKey("↓↑", "Scroll ") + helpKey("Enter", "Select ") + helpKey("Esc", "Close")
}

func (m *Model) themePickerVisible() int {
	return m.measureOverlayVisible([]string{
		titleStyle.Render("T H E M E S"),
		"",
		"x",
		"",
		dimStyle.Render("  0/0 themes"),
		"",
		m.themePickerHelpLine(),
	}, maxPlVisible)
}

func (m *Model) themePickerMaybeAdjustScroll(visible int) {
	clampScroll(&m.themePicker.cursor, &m.themePicker.scroll, len(m.themes)+1, visible)
}
