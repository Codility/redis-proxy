package rproxy

import (
	"encoding/json"
	"io/ioutil"
)

////////////////////////////////////////
// AddrSpec

type AddrSpec struct {
	Addr string `json:"addr"`
	Pass string `json:"pass"`
}

func (as *AddrSpec) Equal(other *AddrSpec) bool {
	return (as.Addr == other.Addr) &&
		(as.Pass == other.Pass)
}

type ProxyConfig struct {
	Uplink          AddrSpec `json:"uplink"`
	Listen          AddrSpec `json:"listen"`
	Admin           AddrSpec `json:"admin"`
	ReadTimeLimitMs int64    `json:"read_time_limit_ms"`
	LogMessages     bool     `json:"log_messages"`
}

type ConfigLoader interface {
	Load() (*ProxyConfig, error)
	String() string
}

////////////////////////////////////////
// FileConfig

type FileConfig struct {
	fileName string
}

func NewFileConfig(name string) *FileConfig {
	return &FileConfig{name}
}

func (f *FileConfig) Load() (*ProxyConfig, error) {
	configJson, err := ioutil.ReadFile(f.fileName)
	if err != nil {
		return nil, err
	}
	var config ProxyConfig
	return &config, json.Unmarshal(configJson, &config)
}

func (f *FileConfig) String() string {
	return f.fileName
}

////////////////////////////////////////
// TestConfig

type TestConfig struct {
	conf *ProxyConfig
	err  error
}

func (c *TestConfig) Load() (*ProxyConfig, error) {
	return c.conf, c.err
}

func (c *TestConfig) String() string {
	return "<const config>"
}

func (c *TestConfig) Replace(conf *ProxyConfig) {
	c.conf = conf
}
