package rproxy

import (
	"encoding/json"
	"io/ioutil"
)

type RedisProxyConfig struct {
	UplinkAddr      string `json:"uplink_addr"`
	ListenOn        string `json:"listen_on"`
	AdminOn         string `json:"admin_on"`
	ReadTimeLimitMs int64  `json:"read_time_limit_ms"`
	LogMessages     bool   `json:"log_messages"`
}

func LoadConfig(fname string) (*RedisProxyConfig, error) {
	configJson, err := ioutil.ReadFile(fname)
	if err != nil {
		return nil, err
	}
	var config RedisProxyConfig
	return &config, json.Unmarshal(configJson, &config)
}
