package launchprofile

import (
	_ "embed"
	"encoding/json"
)

//go:embed policies/mame-content-equivalence.json
var bundledMAMEContentEvidence []byte

var defaultMAMEContentRegistry = func() MAMEContentRegistry {
	var registry MAMEContentRegistry
	if err := json.Unmarshal(bundledMAMEContentEvidence, &registry); err != nil {
		panic(err)
	}
	if err := registry.Validate(); err != nil {
		panic(err)
	}
	return registry
}()

// DefaultMAMEContentRegistry returns bundled, reviewed evidence. Treat it as read-only.
// An administrator registry replaces this default, never changes it on disk.
func DefaultMAMEContentRegistry() MAMEContentRegistry {
	return defaultMAMEContentRegistry
}
