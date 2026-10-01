package theme

import "strings"

// Resolve turns config.Theme into a concrete Theme, always falling back to
// Default() when Omarchy isn't available or the setting doesn't resolve to
// anything usable (unset, unknown slug, parse failure) - callers never
// need to know why, just get a usable theme back.
//
// Recognized cfg values:
//
//	""                -> Default() (adaptive light/dark built-in)
//	"omarchy"         -> whatever Omarchy has active on the desktop right now
//	"omarchy:<slug>"  -> that specific Omarchy theme, regardless of the
//	                     desktop's current one
func Resolve(cfg string) Theme {
	switch {
	case cfg == "":
		return Default()
	case cfg == "omarchy":
		if t, err := LoadCurrent(); err == nil {
			return t
		}
		return Default()
	case strings.HasPrefix(cfg, "omarchy:"):
		if t, err := LoadBySlug(strings.TrimPrefix(cfg, "omarchy:")); err == nil {
			return t
		}
		return Default()
	default:
		return Default()
	}
}
