//go:build !windows
// +build !windows

package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
)

const launchLabel = "com.replado.connector"

var lockFile *os.File

func exeName() string { return "replado-connector" }

func homeDir() string { return os.Getenv("HOME") }

func appDir() string {
	if dir := os.Getenv("REPLADO_CONNECTOR_DIR"); dir != "" {
		return dir
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(homeDir(), "Library", "Application Support", "Replado")
	}
	if base := os.Getenv("XDG_CONFIG_HOME"); base != "" {
		return filepath.Join(base, "replado-connector")
	}
	return filepath.Join(homeDir(), ".config", "replado-connector")
}

func systemDirs() []string {
	if runtime.GOOS == "darwin" {
		return []string{"/System", "/usr", "/bin", "/sbin", "/etc", "/dev", "/private/etc", "/private/var/db"}
	}
	return []string{"/usr", "/bin", "/sbin", "/etc", "/proc", "/sys", "/dev", "/boot", "/run", "/lib", "/lib64"}
}

func browseRoots() []string {
	roots := []string{}
	if home := homeDir(); home != "" {
		roots = append(roots, home)
	}
	if runtime.GOOS == "darwin" {
		roots = append(roots, "/Volumes")
	} else {
		roots = append(roots, "/mnt", "/media", "/srv", "/opt")
	}
	return append(roots, "/")
}

func initConsole() {}

func singleInstance() bool {
	os.MkdirAll(appDir(), 0700)
	f, err := os.OpenFile(filepath.Join(appDir(), "connector.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return true
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return false
	}
	lockFile = f
	ioutil.WriteFile(filepath.Join(appDir(), "connector.pid"), []byte(strconv.Itoa(os.Getpid())), 0600)
	return true
}

func showMessage(title, text string) { fmt.Println(title + ": " + text) }

func osVersion() string {
	if runtime.GOOS == "darwin" {
		if out, err := exec.Command("sw_vers", "-productVersion").Output(); err == nil {
			return "macOS " + strings.TrimSpace(string(out))
		}
		return "macOS"
	}
	if b, err := ioutil.ReadFile("/etc/os-release"); err == nil {
		for _, line := range strings.Split(string(b), "\n") {
			if strings.HasPrefix(line, "PRETTY_NAME=") {
				return strings.Trim(strings.TrimPrefix(line, "PRETTY_NAME="), `"`)
			}
		}
	}
	return "Linux"
}

func hideWindow(c *exec.Cmd) {}

func openBrowser(url string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", url).Start()
	}
	return exec.Command("xdg-open", url).Start()
}

func launchAgentPath() string {
	return filepath.Join(homeDir(), "Library", "LaunchAgents", launchLabel+".plist")
}

func systemdUnitPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(homeDir(), ".config")
	}
	return filepath.Join(base, "systemd", "user", "replado-connector.service")
}

func desktopAutostartPath() string {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		base = filepath.Join(homeDir(), ".config")
	}
	return filepath.Join(base, "autostart", "replado-connector.desktop")
}

func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;")
	return r.Replace(s)
}

func launchAgent(exe string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key><string>` + launchLabel + `</string>
  <key>ProgramArguments</key><array><string>` + xmlEscape(exe) + `</string><string>serve</string></array>
  <key>RunAtLoad</key><true/>
  <key>KeepAlive</key><true/>
  <key>ProcessType</key><string>Background</string>
</dict>
</plist>
`
}

func systemdUnit(exe string) string {
	return "[Unit]\nDescription=Replado Connector\nAfter=network-online.target\n\n[Service]\nExecStart=\"" + exe + "\" serve\nRestart=on-failure\nRestartSec=30\nNoNewPrivileges=true\n\n[Install]\nWantedBy=default.target\n"
}

func hasSystemdUser() bool {
	return exec.Command("systemctl", "--user", "show-environment").Run() == nil
}

func setAutostart(exe string) error {
	if runtime.GOOS == "darwin" {
		p := launchAgentPath()
		os.MkdirAll(filepath.Dir(p), 0755)
		if err := ioutil.WriteFile(p, []byte(launchAgent(exe)), 0644); err != nil {
			return err
		}
		exec.Command("launchctl", "unload", p).Run()
		return exec.Command("launchctl", "load", "-w", p).Run()
	}
	if hasSystemdUser() {
		p := systemdUnitPath()
		os.MkdirAll(filepath.Dir(p), 0755)
		if err := ioutil.WriteFile(p, []byte(systemdUnit(exe)), 0644); err != nil {
			return err
		}
		exec.Command("systemctl", "--user", "daemon-reload").Run()
		return exec.Command("systemctl", "--user", "enable", "--now", "replado-connector.service").Run()
	}
	p := desktopAutostartPath()
	os.MkdirAll(filepath.Dir(p), 0755)
	return ioutil.WriteFile(p, []byte("[Desktop Entry]\nType=Application\nName=Replado Connector\nExec=\""+exe+"\" serve\nNoDisplay=true\nX-GNOME-Autostart-enabled=true\n"), 0644)
}

func removeAutostart() {
	if runtime.GOOS == "darwin" {
		exec.Command("launchctl", "unload", launchAgentPath()).Run()
		os.Remove(launchAgentPath())
		return
	}
	if _, err := os.Stat(systemdUnitPath()); err == nil {
		exec.Command("systemctl", "--user", "disable", "--now", "replado-connector.service").Run()
		os.Remove(systemdUnitPath())
		exec.Command("systemctl", "--user", "daemon-reload").Run()
	}
	os.Remove(desktopAutostartPath())
}

func stopRunning() {
	b, err := ioutil.ReadFile(filepath.Join(appDir(), "connector.pid"))
	if err != nil {
		return
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || pid <= 1 || pid == os.Getpid() {
		return
	}
	if p, err := os.FindProcess(pid); err == nil {
		p.Signal(syscall.SIGTERM)
	}
}

func startsItself() bool {
	if runtime.GOOS == "darwin" {
		return true
	}
	_, err := os.Stat(systemdUnitPath())
	return err == nil
}

func scheduleRemoveDir(dir string) { os.RemoveAll(dir) }

func restartAfterUpdate(exe string) {
	if runtime.GOOS == "darwin" || os.Getenv("INVOCATION_ID") != "" {
		os.Exit(3)
	}
	c := exec.Command(exe, "serve", "--nach-update")
	c.Dir = filepath.Dir(exe)
	if err := c.Start(); err != nil {
		return
	}
	os.Exit(0)
}
