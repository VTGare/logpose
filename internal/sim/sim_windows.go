package sim

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

func runningExe() (string, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return "", err
	}
	defer func() { _ = windows.CloseHandle(snap) }()

	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snap, &entry); err == nil; err = windows.Process32Next(snap, &entry) {
		if strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), "OPTCGSim.exe") {
			return processPath(entry.ProcessID)
		}
	}
	return "", ErrNotFound
}

func processPath(pid uint32) (string, error) {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return "", err
	}
	defer func() { _ = windows.CloseHandle(h) }()
	buf := make([]uint16, windows.MAX_LONG_PATH)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return "", err
	}
	if n == 0 {
		return "", errors.New("sim: empty process path")
	}
	return windows.UTF16ToString(buf[:n]), nil
}

func usualDirs() []string {
	var dirs []string
	for _, d := range []string{os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LOCALAPPDATA")} {
		if d != "" {
			dirs = append(dirs, filepath.Join(d, "OPTCGSim", "Builds_Windows"), filepath.Join(d, "OPTCGSim"))
		}
	}
	return dirs
}
