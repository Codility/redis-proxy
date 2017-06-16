package rproxy

import (
	"encoding/json"
	"io/ioutil"
)

////////////////////////////////////////
// AddrSpec

type TLSSpec struct {
	CertFile, KeyFile, CACertFile string
}

type AddrSpec struct {
	Addr string   `json:"addr"`
	Pass string   `json:"pass"`
	TLS  *TLSSpec `json:"tls"`
}

func (as *AddrSpec) Equal(other *AddrSpec) bool {
	return (as.Addr == other.Addr) &&
		(as.Pass == other.Pass)
}

////////////////////////////////////////
// Config

type Config struct {
	Uplink          AddrSpec `json:"uplink"`
	Listen          AddrSpec `json:"listen"`
	Admin           AddrSpec `json:"admin"`
	ReadTimeLimitMs int64    `json:"read_time_limit_ms"`
	LogMessages     bool     `json:"log_messages"`
}

type ConfigLoader interface {
	Load() (*Config, error)
}

////////////////////////////////////////
// FileConfigLoader

type FileConfigLoader struct {
	fileName string
}

func NewFileConfigLoader(name string) *FileConfigLoader {
	return &FileConfigLoader{name}
}

func (f *FileConfigLoader) Load() (*Config, error) {
	configJson, err := ioutil.ReadFile(f.fileName)
	if err != nil {
		return nil, err
	}
	var config Config
	return &config, json.Unmarshal(configJson, &config)
}

////////////////////////////////////////
// TestConfigLoader

type TestConfigLoader struct {
	conf *Config
	err  error
}

func NewTestConfigLoader(uplinkAddr string) *TestConfigLoader {
	return &TestConfigLoader{
		conf: &Config{
			Uplink: AddrSpec{Addr: uplinkAddr},
			Listen: AddrSpec{Addr: "127.0.0.1:0"},
			Admin:  AddrSpec{Addr: "127.0.0.1:0"},
		},
	}
}

func (c *TestConfigLoader) Load() (*Config, error) {
	return c.conf, c.err
}

func (c *TestConfigLoader) Replace(conf *Config) {
	c.conf = conf
}
