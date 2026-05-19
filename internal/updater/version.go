package updater

import "sync/atomic"

var version atomic.Value

func init() {
	version.Store("dev")
}

// SetVersion is called once from main() with the version embedded at build
// time via -ldflags "-X main.version=v1.2.3". Defaults to "dev" for local
// builds.
func SetVersion(v string) {
	if v == "" {
		v = "dev"
	}
	version.Store(v)
}

func Version() string {
	return version.Load().(string)
}
