package output

import (
	"fmt"
	"os"
	"path/filepath"
)

// DirectorySize returns the total byte size of path: the file's own size if
// path is a regular file or symlink, or the recursive sum of every regular
// file under path if it is a directory. It is best-effort and on-demand --
// nothing in toolsniff persists this value, so a moved or deleted file
// simply returns an error the caller can display as "size unavailable"
// rather than a stale number.
func DirectorySize(path string) (int64, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return 0, err
	}
	if !info.IsDir() {
		return info.Size(), nil
	}

	var total int64
	err = filepath.Walk(path, func(_ string, walkInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			// A single unreadable file (permissions, race with deletion)
			// should not fail the whole measurement -- skip it and keep
			// summing the rest, the same tolerance scanner/evidence.go
			// already applies to individual filesystem reads.
			return nil
		}
		if !walkInfo.IsDir() {
			total += walkInfo.Size()
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

// FormatBytes renders a byte count the way a user reads disk usage: whole
// numbers below 1KB, one decimal place above it, capped at GB (nothing this
// tool measures is expected to reach TB).
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	units := []string{"KB", "MB", "GB"}
	// Loop scales up, but stops before trying to use a 4th unit (TiB),
	// so anything >= 1TiB renders as a large GB number instead of panicking.
	for n := bytes / unit; n >= unit && exp < len(units)-1; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}
