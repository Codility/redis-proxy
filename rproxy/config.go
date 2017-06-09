package rproxy

import (
	"encoding/json"
	"io/ioutil"
)

type ProxyConfig struct {
	UplinkAddr      string `json:"uplink_addr"`
	ListenOn        string `json:"listen_on"`
	AdminOn         string `json:"admin_on"`
	ReadTimeLimitMs int64  `json:"read_time_limit_ms"`
	LogMessages     bool   `json:"log_messages"`
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
// ConstConfig

type ConstConfig struct {
	conf *ProxyConfig
	err  error
}

func (c *ConstConfig) Load() (*ProxyConfig, error) {
	return c.conf, c.err
}

func (c *ConstConfig) String() string {
	return "<const config>"
}
