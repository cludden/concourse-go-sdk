package settings

// Settings describes common archive configuration common to all backends
type Settings struct {
	// ForceHistory indicates that an archive should return all available history on Check
	// regardless of whether or not a latest version is provided. This can be useful when
	// pinned resources are orphaned in various situations (e.g. resource credentials are
	// rotated)
	ForceHistory bool `json:"force_history"`
}
