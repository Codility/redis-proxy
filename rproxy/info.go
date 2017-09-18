package rproxy

type ProxyInfo struct {
	ActiveRequests  int
	WaitingRequests int
	State           ProxyState
	Config          *Config
	RawConnections  int
}
