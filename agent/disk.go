package agent

import (
	"os"
	"strings"
	"syscall"
)

// realFS to systemy plików, które są prawdziwymi dyskami.
// /proc/mounts wymienia też dziesiątki pseudo-systemów (proc,
// sysfs, tmpfs…) — te pomijamy, bo nie interesują użytkownika.
var realFS = map[string]bool{
	"ext2": true, "ext3": true, "ext4": true,
	"xfs": true, "btrfs": true, "f2fs": true,
	"vfat": true, "exfat": true, "ntfs": true, "ntfs3": true,
	"zfs": true,
}

// listDisks czyta punkty montowania z /proc/mounts i dla każdego
// prawdziwego dysku pyta jądro o zajętość wywołaniem Statfs
// (to samo, czego używa polecenie `df`).
func listDisks() []DiskStat {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return nil
	}

	var disks []DiskStat
	seen := map[string]bool{} // to samo urządzenie bywa zamontowane kilka razy

	for _, line := range strings.Split(string(data), "\n") {
		// format wiersza: urządzenie punkt_montowania typ opcje …
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}
		device, mount, fstype := fields[0], fields[1], fields[2]
		if !realFS[fstype] || seen[device] {
			continue
		}

		var st syscall.Statfs_t
		if err := syscall.Statfs(mount, &st); err != nil {
			continue
		}
		bsize := uint64(st.Bsize)
		total := st.Blocks * bsize
		if total == 0 {
			continue
		}
		// Bavail = bloki dostępne dla zwykłego użytkownika
		// (Bfree zawiera też rezerwę roota — jak w `df`).
		used := (st.Blocks - st.Bfree) * bsize
		avail := st.Bavail * bsize

		d := DiskStat{
			Mount:      mount,
			Device:     device,
			FSType:     fstype,
			TotalBytes: total,
			UsedBytes:  used,
		}
		if used+avail > 0 {
			d.UsedPercent = 100 * float64(used) / float64(used+avail)
		}
		disks = append(disks, d)
		seen[device] = true
	}
	return disks
}
