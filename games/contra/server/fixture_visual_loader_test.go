package contra

import "io/fs"

// Existing isolated fixtures delegate to the production setup reader.
func readFixtureVisualResources(files fs.FS, raw []byte, build string) (*BotVisualResources, error) {
	return LoadVisualResources(files, raw, build)
}
