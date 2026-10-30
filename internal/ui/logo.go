package ui

import (
	"embed"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

//go:embed logo/logo.txt
var logoFS embed.FS

// logoArt carries its own colour escapes, so it is rendered as it was authored.
var logoArt = func() []string {
	data, err := logoFS.ReadFile("logo/logo.txt")
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, "\r")
	}
	return lines
}()

var logoWidth = func() int {
	w := 0
	for _, l := range logoArt {
		w = max(w, lipgloss.Width(l))
	}
	return w
}()

func logoHeight() int { return len(logoArt) }

// logoBlock renders the logo, padded to its full width so anything beside it
// lines up.
func logoBlock() string {
	if len(logoArt) == 0 {
		return styleTitle.Render("sugarlime")
	}
	lines := make([]string, len(logoArt))
	for i, l := range logoArt {
		lines[i] = l + strings.Repeat(" ", max(0, logoWidth-lipgloss.Width(l)))
	}
	return strings.Join(lines, "\n")
}

// logoFits reports whether the browser has room for the logo block beside the
// file details without crowding out the list.
func logoFits(width, height int) bool {
	return width >= logoWidth+28 && height >= logoHeight()+17
}

// logoFitsPicker is the laxer check for the picker screen, which has no list
// competing for the lines.
func logoFitsPicker(width, height int) bool {
	return width >= logoWidth+2 && height >= logoHeight()+7
}
