//go:build windows
// +build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
	"unsafe"
)

var (
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	user32               = syscall.NewLazyDLL("user32.dll")
	ntdll                = syscall.NewLazyDLL("ntdll.dll")
	procAttachConsole    = kernel32.NewProc("AttachConsole")
	procGetLogicalDrives = kernel32.NewProc("GetLogicalDrives")
	procGetDriveType     = kernel32.NewProc("GetDriveTypeW")
	procCreateMutex      = kernel32.NewProc("CreateMutexW")
	procMessageBox       = user32.NewProc("MessageBoxW")
	procRtlGetVersion    = ntdll.NewProc("RtlGetVersion")
)

const runKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

func exeName() string { return "replado-connector.exe" }

func homeDir() string { return os.Getenv("USERPROFILE") }

func systemDirs() []string {
	dir := os.Getenv("WINDIR")
	if dir == "" {
		dir = `C:\Windows`
	}
	return []string{dir}
}

func browseRoots() []string { return defaultRoots() }

func openBrowser(url string) error {
	c := exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	return c.Start()
}

func startsItself() bool { return false }

func appDir() string {
	if dir := os.Getenv("REPLADO_CONNECTOR_DIR"); dir != "" {
		return dir
	}
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		base = os.TempDir()
	}
	return filepath.Join(base, "Replado")
}

func initConsole() {
	if h, err := syscall.GetStdHandle(syscall.STD_OUTPUT_HANDLE); err == nil && h != syscall.InvalidHandle && h != 0 {
		return
	}
	if r, _, _ := procAttachConsole.Call(uintptr(0xFFFFFFFF)); r == 0 {
		return
	}
	if f, err := os.OpenFile("CONOUT$", os.O_WRONLY, 0); err == nil {
		os.Stdout, os.Stderr = f, f
	}
	if f, err := os.Open("CONIN$"); err == nil {
		os.Stdin = f
	}
}

func defaultRoots() []string {
	mask, _, _ := procGetLogicalDrives.Call()
	var roots []string
	for i := uint(0); i < 26; i++ {
		if mask&(1<<i) == 0 {
			continue
		}
		root := string(rune('A'+i)) + `:\`
		p, _ := syscall.UTF16PtrFromString(root)
		t, _, _ := procGetDriveType.Call(uintptr(unsafe.Pointer(p)))
		if t == 3 || t == 4 {
			roots = append(roots, root)
		}
	}
	return roots
}

func singleInstance() bool {
	name, _ := syscall.UTF16PtrFromString(`Local\ReplaDoConnector`)
	h, _, err := procCreateMutex.Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return true
	}
	if e, ok := err.(syscall.Errno); ok && e == 183 {
		syscall.CloseHandle(syscall.Handle(h))
		return false
	}
	return true
}

func showMessage(title, text string) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	procMessageBox.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x40)
}

type osVersionInfo struct {
	size, major, minor, build, platform uint32
	csd                                 [128]uint16
}

func osVersion() string {
	var v osVersionInfo
	v.size = uint32(unsafe.Sizeof(v))
	procRtlGetVersion.Call(uintptr(unsafe.Pointer(&v)))
	name := "Windows " + strconv.Itoa(int(v.major)) + "." + strconv.Itoa(int(v.minor))
	switch {
	case v.major == 6 && v.minor == 0:
		name = "Windows Vista"
	case v.major == 6 && v.minor == 1:
		name = "Windows 7"
	case v.major == 6 && v.minor == 2:
		name = "Windows 8"
	case v.major == 6 && v.minor == 3:
		name = "Windows 8.1"
	case v.major == 10 && v.build >= 22000:
		name = "Windows 11"
	case v.major == 10:
		name = "Windows 10"
	}
	return name + " Build " + strconv.Itoa(int(v.build))
}

func hideWindow(c *exec.Cmd) { c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }

func run(name string, args ...string) error {
	c := exec.Command(name, args...)
	hideWindow(c)
	return c.Run()
}

func setAutostart(exe string) error {
	return run("reg", "add", runKey, "/v", "ReplaDoConnector", "/t", "REG_SZ", "/d", `"`+exe+`" serve`, "/f")
}

func removeAutostart() { run("reg", "delete", runKey, "/v", "ReplaDoConnector", "/f") }

func stopRunning() {
	run("taskkill", "/F", "/IM", exeName(), "/FI", "PID ne "+strconv.Itoa(os.Getpid()))
}

func scheduleRemoveDir(dir string) {
	c := exec.Command("cmd", "/C", "ping 127.0.0.1 -n 3 > nul & rmdir /S /Q \""+dir+"\"")
	hideWindow(c)
	c.Start()
}

func restartAfterUpdate(exe string) {
	c := exec.Command(exe, "serve", "--nach-update")
	c.Dir = filepath.Dir(exe)
	hideWindow(c)
	if err := c.Start(); err != nil {
		return
	}
	trayRemove()
	os.Exit(0)
}
