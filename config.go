package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
)

const defaultServer = "https://replado.com"

type Folder struct {
	Path         string `json:"path"`
	Confirmed    bool   `json:"confirmed"`
	Confirmation string `json:"confirmation"`
	ConfirmedAt  string `json:"confirmed_at"`
}

type Config struct {
	Server      string   `json:"server"`
	Account     string   `json:"account,omitempty"`
	ConnectorID string   `json:"connector_id,omitempty"`
	Secret      string   `json:"secret,omitempty"`
	Folders     []Folder `json:"folders,omitempty"`
	Known       []Known  `json:"known,omitempty"`
	RescanAt    int64    `json:"rescan_at,omitempty"`
}

var configMu sync.Mutex

func configPath() string { return filepath.Join(appDir(), "connector.json") }

func loadConfig() *Config {
	configMu.Lock()
	defer configMu.Unlock()
	c := &Config{Server: defaultServer}
	if b, err := ioutil.ReadFile(configPath()); err == nil {
		json.Unmarshal(b, c)
	}
	if c.Server == "" {
		c.Server = defaultServer
	}
	return c
}

func (c *Config) save() error {
	configMu.Lock()
	defer configMu.Unlock()
	if err := os.MkdirAll(appDir(), 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	tmp := configPath() + ".neu"
	if err := ioutil.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	os.Remove(configPath())
	return os.Rename(tmp, configPath())
}

func updateConfig(change func(c *Config) error) (*Config, error) {
	c := loadConfig()
	if err := change(c); err != nil {
		return c, err
	}
	return c, c.save()
}

func (c *Config) confirmedPaths() []string {
	var paths []string
	for _, f := range c.Folders {
		if f.Confirmed && f.Confirmation != "" {
			paths = append(paths, f.Path)
		}
	}
	return paths
}
