//go:build !windows

package doctor

import "syscall"

// openFileLimit returns the soft RLIMIT_NOFILE value.
func openFileLimit() (uint64, bool) {
	var lim syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &lim); err != nil {
		return 0, false
	}
	return lim.Cur, true
}

// freeBytes returns the free space of the filesystem holding path.
func freeBytes(path string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	return stat.Bavail * uint64(stat.Bsize)
}
