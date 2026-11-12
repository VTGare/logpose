package shortcut

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func folder(place Place) (string, error) {
	id := windows.FOLDERID_Programs
	if place == Desktop {
		id = windows.FOLDERID_Desktop
	}
	// KF_FLAG_CREATE, since a fresh profile may not have the folder yet.
	return windows.KnownFolderPath(id, windows.KF_FLAG_CREATE)
}

var (
	ole32                = windows.NewLazySystemDLL("ole32.dll")
	procCoCreateInstance = ole32.NewProc("CoCreateInstance")

	clsidShellLink  = windows.GUID{Data1: 0x00021401, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIShellLinkW  = windows.GUID{Data1: 0x000214f9, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
	iidIPersistFile = windows.GUID{Data1: 0x0000010b, Data4: [8]byte{0xc0, 0, 0, 0, 0, 0, 0, 0x46}}
)

// Vtable slots of IShellLinkW and IPersistFile, after IUnknown's QueryInterface, AddRef and
// Release.
const (
	slotQueryInterface  = 0
	slotRelease         = 2
	slotSetDescription  = 7
	slotSetWorkingDir   = 9
	slotSetArguments    = 11
	slotSetPath         = 20
	slotPersistFileSave = 6
)

const clsctxInprocServer = 1

// Results of CoInitializeEx that x/sys returns as errors, though COM is usable after them.
// S_FALSE means the thread had COM already.
var (
	sFalse          error = syscall.Errno(1)
	rpcEChangedMode error = syscall.Errno(0x80010106)
)

func save(path string, l Link) error {
	// COM is set up per thread, so the calls stay on this one.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	switch err := windows.CoInitializeEx(0, windows.COINIT_APARTMENTTHREADED); {
	case err == nil, errors.Is(err, sFalse):
		defer windows.CoUninitialize()
	case errors.Is(err, rpcEChangedMode):
		// The thread already has COM in another mode, which ShellLink also works in.
	default:
		return fmt.Errorf("shortcut: starting COM: %w", err)
	}

	var link *comObject
	hr, _, _ := procCoCreateInstance.Call(uintptr(unsafe.Pointer(&clsidShellLink)), 0, clsctxInprocServer,
		uintptr(unsafe.Pointer(&iidIShellLinkW)), uintptr(unsafe.Pointer(&link)))
	if failed(hr) {
		return hresultError("CoCreateInstance(ShellLink)", hr)
	}
	defer call(link, slotRelease)

	for _, set := range []struct {
		slot  int
		name  string
		value string
	}{
		{slotSetPath, "SetPath", l.Target},
		{slotSetArguments, "SetArguments", joinArgs(l.Args)},
		{slotSetWorkingDir, "SetWorkingDirectory", l.Dir},
		{slotSetDescription, "SetDescription", l.Description},
	} {
		p, err := windows.UTF16PtrFromString(set.value)
		if err != nil {
			return fmt.Errorf("shortcut: %s: %w", set.name, err)
		}
		if hr := call(link, set.slot, uintptr(unsafe.Pointer(p))); failed(hr) {
			return hresultError("IShellLinkW::"+set.name, hr)
		}
	}

	var file *comObject
	if hr := call(link, slotQueryInterface, uintptr(unsafe.Pointer(&iidIPersistFile)), uintptr(unsafe.Pointer(&file))); failed(hr) {
		return hresultError("QueryInterface(IPersistFile)", hr)
	}
	defer call(file, slotRelease)
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return fmt.Errorf("shortcut: %w", err)
	}
	if hr := call(file, slotPersistFileSave, uintptr(unsafe.Pointer(p)), 1); failed(hr) {
		return hresultError("IPersistFile::Save", hr)
	}
	return nil
}

// comObject is a COM interface pointer. Its first word points at the method table.
type comObject struct {
	vtbl *[32]uintptr
}

func call(obj *comObject, slot int, args ...uintptr) uintptr {
	hr, _, _ := syscall.SyscallN(obj.vtbl[slot], append([]uintptr{uintptr(unsafe.Pointer(obj))}, args...)...)
	return hr
}

func failed(hr uintptr) bool { return int32(hr) < 0 }

func hresultError(op string, hr uintptr) error {
	return fmt.Errorf("shortcut: %s: %w", op, windows.Errno(hr))
}

func joinArgs(args []string) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = windows.EscapeArg(a)
	}
	return strings.Join(quoted, " ")
}
