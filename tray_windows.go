//go:build windows
// +build windows

package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	procShellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")
	procRegisterClassEx     = user32.NewProc("RegisterClassExW")
	procCreateWindowEx      = user32.NewProc("CreateWindowExW")
	procDefWindowProc       = user32.NewProc("DefWindowProcW")
	procGetMessage          = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessage     = user32.NewProc("DispatchMessageW")
	procRegisterWindowMsg   = user32.NewProc("RegisterWindowMessageW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenu          = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procPostMessage         = user32.NewProc("PostMessageW")
	procSetTimer            = user32.NewProc("SetTimer")
	procGetSystemMetrics    = user32.NewProc("GetSystemMetrics")
	procCreateIconFromResEx = user32.NewProc("CreateIconFromResourceEx")
	procGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
	procMessageBoxConfirm   = user32.NewProc("MessageBoxW")
	trayTaskbarCreated      uintptr
	trayWindow              uintptr
	trayAdded               bool
	trayLast                trayState
	trayIcons               = map[bool]uintptr{}
	trayCallback            = syscall.NewCallback(trayProc)
	trayClassName, _        = syscall.UTF16PtrFromString("RepladoConnectorTray")
)

const (
	nimAdd           = 0
	nimModify        = 1
	nimDelete        = 2
	nifMessage       = 0x1
	nifIcon          = 0x2
	nifTip           = 0x4
	wmNull           = 0x0
	wmTimer          = 0x113
	wmApp            = 0x8000
	wmTray           = wmApp + 1
	wmLButtonUp      = 0x202
	wmLButtonDblClk  = 0x203
	wmRButtonUp      = 0x205
	wmContextMenu    = 0x7B
	mfString         = 0x0
	mfGrayed         = 0x1
	mfSeparator      = 0x800
	tpmRightButton   = 0x2
	tpmNoNotify      = 0x80
	tpmReturnCmd     = 0x100
	smCxSmIcon       = 49
	mbYesNo          = 0x4
	mbIconQuestion   = 0x20
	idYes            = 6
	menuOpen         = 1
	menuQuit         = 2
	trayTimerID      = 1
	trayTimerMS      = 5000
	iconResVersion   = 0x00030000
	trayQuitQuestion = "Solange der Connector beendet ist, kann Replado nicht in der lokalen Software nachsehen. Fragen warten dann, bis er wieder läuft. Er startet beim nächsten Anmelden an Windows von selbst.\n\nConnector jetzt beenden?"
)

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   uintptr
	icon       uintptr
	cursor     uintptr
	background uintptr
	menuName   *uint16
	className  *uint16
	iconSm     uintptr
}

type notifyIconData struct {
	size            uint32
	wnd             uintptr
	id              uint32
	flags           uint32
	callbackMessage uint32
	icon            uintptr
	tip             [128]uint16
	state           uint32
	stateMask       uint32
	info            [256]uint16
	version         uint32
	infoTitle       [64]uint16
	infoFlags       uint32
	guidItem        [16]byte
	balloonIcon     uintptr
}

type trayPoint struct{ x, y int32 }

type trayMsg struct {
	wnd     uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      trayPoint
}

func startTray() {
	go func() {
		runtime.LockOSThread()
		if err := runTray(); err != nil {
			log.Printf("Symbol neben der Uhr: %v", err)
		}
	}()
}

func runTray() error {
	name, _ := syscall.UTF16PtrFromString("TaskbarCreated")
	trayTaskbarCreated, _, _ = procRegisterWindowMsg.Call(uintptr(unsafe.Pointer(name)))
	instance, _, _ := procGetModuleHandle.Call(0)
	wc := wndClassEx{wndProc: trayCallback, instance: instance, className: trayClassName}
	wc.size = uint32(unsafe.Sizeof(wc))
	if r, _, err := procRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return err
	}
	title, _ := syscall.UTF16PtrFromString(trayAppName)
	hwnd, _, err := procCreateWindowEx.Call(0, uintptr(unsafe.Pointer(trayClassName)), uintptr(unsafe.Pointer(title)), 0, 0, 0, 0, 0, 0, 0, instance, 0)
	if hwnd == 0 {
		return err
	}
	trayWindow = hwnd
	trayRefresh(true)
	procSetTimer.Call(hwnd, trayTimerID, trayTimerMS, 0)
	var m trayMsg
	for {
		r, _, _ := procGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return nil
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func trayProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch {
	case msg == wmTray:
		switch lParam & 0xFFFF {
		case wmLButtonUp, wmLButtonDblClk:
			openSetup()
		case wmRButtonUp, wmContextMenu:
			showTrayMenu(hwnd)
		}
		return 0
	case msg == wmTimer:
		trayRefresh(!trayAdded)
		return 0
	case trayTaskbarCreated != 0 && msg == trayTaskbarCreated:
		trayRefresh(true)
		return 0
	}
	r, _, _ := procDefWindowProc.Call(hwnd, msg, wParam, lParam)
	return r
}

func trayIcon(online bool) uintptr {
	if h, ok := trayIcons[online]; ok {
		return h
	}
	size, _, _ := procGetSystemMetrics.Call(smCxSmIcon)
	if size == 0 {
		size = 16
	}
	img := iconImage(iconFile(online), int(size))
	if len(img) == 0 {
		return 0
	}
	h, _, _ := procCreateIconFromResEx.Call(uintptr(unsafe.Pointer(&img[0])), uintptr(len(img)), 1, iconResVersion, size, size, 0)
	if h != 0 {
		trayIcons[online] = h
	}
	return h
}

func trayData(s trayState) *notifyIconData {
	d := &notifyIconData{wnd: trayWindow, id: 1, flags: nifMessage | nifIcon | nifTip, callbackMessage: wmTray, icon: trayIcon(s.online)}
	d.size = uint32(unsafe.Sizeof(*d))
	tip, _ := syscall.UTF16FromString(trayTip(s))
	copy(d.tip[:len(d.tip)-1], tip)
	return d
}

func trayRefresh(add bool) {
	s := currentTrayState(loadConfig(), readStatus(), time.Now())
	if !add && s == trayLast {
		return
	}
	action := uintptr(nimModify)
	if add {
		action = nimAdd
		procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(trayData(s))))
	}
	r, _, _ := procShellNotifyIcon.Call(action, uintptr(unsafe.Pointer(trayData(s))))
	if r != 0 {
		trayLast = s
		trayAdded = true
	} else if add {
		trayAdded = false
	}
}

func appendMenu(menu uintptr, flags, id uintptr, text string) {
	var p uintptr
	if text != "" {
		t, _ := syscall.UTF16PtrFromString(text)
		p = uintptr(unsafe.Pointer(t))
	}
	procAppendMenu.Call(menu, flags, id, p)
}

func showTrayMenu(hwnd uintptr) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer procDestroyMenu.Call(menu)
	appendMenu(menu, mfString|mfGrayed, 0, currentTrayState(loadConfig(), readStatus(), time.Now()).text)
	appendMenu(menu, mfSeparator, 0, "")
	appendMenu(menu, mfString, menuOpen, "Einrichtung öffnen")
	appendMenu(menu, mfString, menuQuit, "Connector beenden")
	var pt trayPoint
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(hwnd)
	cmd, _, _ := procTrackPopupMenu.Call(menu, tpmRightButton|tpmNoNotify|tpmReturnCmd, uintptr(pt.x), uintptr(pt.y), 0, hwnd, 0)
	procPostMessage.Call(hwnd, wmNull, 0, 0)
	switch cmd {
	case menuOpen:
		openSetup()
	case menuQuit:
		if confirmQuit() {
			procShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(trayData(trayLast))))
			log.Printf("Vom Nutzer beendet")
			os.Exit(0)
		}
	}
}

func confirmQuit() bool {
	t, _ := syscall.UTF16PtrFromString(trayAppName)
	m, _ := syscall.UTF16PtrFromString(trayQuitQuestion)
	r, _, _ := procMessageBoxConfirm.Call(trayWindow, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), mbYesNo|mbIconQuestion)
	return r == idYes
}

func openSetup() {
	self, err := os.Executable()
	if err != nil {
		return
	}
	c := exec.Command(self, "ui")
	c.Dir = filepath.Dir(self)
	hideWindow(c)
	if err := c.Start(); err != nil {
		log.Printf("Einrichtung öffnen: %v", err)
		return
	}
	go c.Wait()
}

func shortcutPath() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		return ""
	}
	return filepath.Join(base, "Microsoft", "Windows", "Start Menu", "Programs", trayAppName+".lnk")
}

func vbsString(s string) string { return `"` + strings.Replace(s, `"`, `""`, -1) + `"` }

func installShortcut(exe string) {
	link := shortcutPath()
	if link == "" {
		return
	}
	dir := filepath.Dir(exe)
	icon := filepath.Join(dir, "replado.ico")
	if err := ioutil.WriteFile(icon, iconFile(true), 0600); err != nil {
		icon = exe
	}
	script := strings.Join([]string{
		`Set s = CreateObject("WScript.Shell")`,
		`Set l = s.CreateShortcut(` + vbsString(link) + `)`,
		`l.TargetPath = ` + vbsString(exe),
		`l.Arguments = "ui"`,
		`l.WorkingDirectory = ` + vbsString(dir),
		`l.IconLocation = ` + vbsString(icon+",0"),
		`l.Description = "Replado Connector einrichten"`,
		`l.Save`,
	}, "\r\n") + "\r\n"
	vbs := filepath.Join(dir, "verknuepfung.vbs")
	if err := ioutil.WriteFile(vbs, []byte(script), 0600); err != nil {
		return
	}
	defer os.Remove(vbs)
	if err := run("cscript", "//nologo", "//B", vbs); err != nil {
		log.Printf("Eintrag im Startmenü: %v", err)
	}
}

func removeShortcut() {
	if link := shortcutPath(); link != "" {
		os.Remove(link)
	}
}
