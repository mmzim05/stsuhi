package theme

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// systemThemesDir is Omarchy's installed theme catalog. Respects
// $OMARCHY_PATH (what Omarchy's own scripts use) so this still works if
// Omarchy is ever installed somewhere other than /usr/share/omarchy.
func systemThemesDir() string {
	if p := os.Getenv("OMARCHY_PATH"); p != "" {
		return filepath.Join(p, "themes")
	}
	return "/usr/share/omarchy/themes"
}

// themeDirs lists where themes live, user overrides first, same
// convention as Omarchy's own omarchy-theme-set/omarchy-theme-list.
// Without a home directory only the system catalog is searched.
func themeDirs() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return []string{systemThemesDir()}
	}
	return []string{
		filepath.Join(home, ".config", "omarchy", "themes"),
		systemThemesDir(),
	}
}

// Available reports whether Omarchy's theme catalog exists on this
// machine at all. stsuhi runs fine without it - callers should fall back
// to Default().
func Available() bool {
	_, err := os.Stat(systemThemesDir())
	return err == nil
}

// Info describes one selectable Omarchy theme.
type Info struct {
	Slug        string // directory name, e.g. "tokyo-night"
	DisplayName string // e.g. "Tokyo Night"
}

// DisplayName turns an Omarchy theme directory slug into a human label,
// matching the capitalization Omarchy's own omarchy-theme-list produces.
func DisplayName(slug string) string {
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

// List returns every Omarchy theme available on this machine: the system
// catalog plus any user overrides under ~/.config/omarchy/themes, deduped
// by slug (user override wins), sorted by display name.
func List() []Info {
	seen := map[string]bool{}
	var slugs []string
	for _, dir := range themeDirs() {
		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() || seen[e.Name()] {
				continue
			}
			seen[e.Name()] = true
			slugs = append(slugs, e.Name())
		}
	}

	infos := make([]Info, len(slugs))
	for i, s := range slugs {
		infos[i] = Info{Slug: s, DisplayName: DisplayName(s)}
	}
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].DisplayName < infos[j].DisplayName
	})
	return infos
}

// LoadCurrent loads whatever theme Omarchy currently has active on the
// desktop, straight from its live staged state so it always matches.
func LoadCurrent() (Theme, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Theme{}, fmt.Errorf("find current omarchy theme: %w", err)
	}
	// Omarchy stages the active theme's files here on every
	// omarchy-theme-set.
	current := filepath.Join(home, ".local", "state", "omarchy", "current")
	name := "Omarchy"
	data, err := os.ReadFile(filepath.Join(current, "theme.name"))
	if err == nil {
		name = DisplayName(strings.TrimSpace(string(data)))
	}
	return loadColorsFile(filepath.Join(current, "theme", "colors.toml"), name)
}

// LoadBySlug loads one specific Omarchy theme by its directory slug,
// checking the user override directory before the system catalog.
func LoadBySlug(slug string) (Theme, error) {
	for _, dir := range themeDirs() {
		path := filepath.Join(dir, slug, "colors.toml")
		if _, err := os.Stat(path); err == nil {
			return loadColorsFile(path, DisplayName(slug))
		}
	}
	return Theme{}, fmt.Errorf("omarchy theme %q not found", slug)
}

// loadColorsFile parses an Omarchy theme's colors.toml. The format is a
// flat set of `key = "value"` lines (see any file under
// /usr/share/omarchy/themes/*/colors.toml) - no nested tables, so a small
// hand-rolled parser covers it without adding a TOML dependency.
func loadColorsFile(path, name string) (Theme, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Theme{}, fmt.Errorf("read omarchy theme: %w", err)
	}

	raw := map[string]string{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		raw[strings.TrimSpace(key)] = strings.Trim(strings.TrimSpace(val), `"`)
	}

	color := func(k string) lipgloss.Color { return lipgloss.Color(raw[k]) }

	t := Theme{
		Name:              name,
		Mode:              raw["mode"],
		Background:        color("background"),
		DarkBackground:    color("dark_background"),
		LighterBackground: color("lighter_background"),
		Foreground:        color("foreground"),
		MutedForeground:   color("dark_foreground"),
		BrightForeground:  color("bright_foreground"),
		Accent:            color("accent"),
		Selection:         color("selection"),
		Muted:             color("muted"),
		Red:               color("red"),
		Orange:            color("orange"),
		Yellow:            color("yellow"),
		Green:             color("green"),
		Cyan:              color("cyan"),
		Blue:              color("blue"),
		Magenta:           color("magenta"),
	}
	if t.Mode == "" {
		t.Mode = "dark"
	}
	return t, nil
}
