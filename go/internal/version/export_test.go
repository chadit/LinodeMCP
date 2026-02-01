package version

// SetBuildDate sets the BuildDate variable for testing.
func SetBuildDate(d string) string {
	old := BuildDate
	BuildDate = d

	return old
}
