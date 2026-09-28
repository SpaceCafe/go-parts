//go:build !linux

package procrun

// sandboxSupported reports whether prlimit and landlock-restrict can restrict a process here. Both
// are Linux tools, so elsewhere processes run unrestricted and Validate rejects Restrictions.Strict.
const sandboxSupported = false

// sandboxArgs returns no arguments: prlimit and landlock-restrict are Linux tools.
func sandboxArgs(*Config, ...string) []string {
	return nil
}

// checkCapabilities warns that no restrictions apply on this platform.
func checkCapabilities(r *Runner) {
	r.Log.Warn("procrun: landlock LSM and rlimits are only available on Linux. " +
		"Process's filesystem, network, and resource restrictions will not be applied!")
}
