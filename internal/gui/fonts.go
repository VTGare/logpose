package gui

import (
	"embed"
	"fmt"

	"github.com/egoist/mygo/ui"
)

// fontFamily is Inter, bundled under the SIL Open Font License (assets/fonts/OFL.txt), so the
// app looks the same on every Windows install.
const fontFamily = "Inter"

//go:embed assets/fonts/*.ttf
var fontFiles embed.FS

// registerFonts adds Inter's weights. Without them the app falls back to the system font.
func registerFonts() error {
	for _, name := range []string{"Regular", "Medium", "SemiBold", "Bold"} {
		data, err := fontFiles.ReadFile("assets/fonts/Inter-" + name + ".ttf")
		if err != nil {
			return err
		}
		if err := ui.RegisterFont(data, fontFamily); err != nil {
			return fmt.Errorf("register Inter %s: %w", name, err)
		}
	}
	return nil
}
