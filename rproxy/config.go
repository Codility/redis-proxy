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

func LoadConfig(fname string) (*ProxyConfig, error) {
	configJson, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	var config ProxyConfig
	return &config, json.Unmarshal(configJson, &config)
}
