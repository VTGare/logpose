package kit

import (
	"embed"
	"fmt"

	"github.com/egoist/mygo/ui"
)

// fontFamily is Inter, bundled under the SIL Open Font License (fonts/OFL.txt), so the
// app looks the same on every Windows install.
const fontFamily = "Inter"

//go:embed fonts/*.ttf
var fontFiles embed.FS

// RegisterFonts adds Inter's weights. Without them the app falls back to the system font.
func RegisterFonts() error {
	for _, name := range []string{"Regular", "Medium", "SemiBold", "Bold"} {
		data, err := fontFiles.ReadFile("fonts/Inter-" + name + ".ttf")
		if err != nil {
			return err
		}
		if err := ui.RegisterFont(data, fontFamily); err != nil {
			return fmt.Errorf("register Inter %s: %w", name, err)
		}
	}
	return nil
}
