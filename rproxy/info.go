package rproxy

type ProxyInfo struct {
	ActiveRequests  int        `json:"active_requests"`
	WaitingRequests int        `json:"waiting_requests"`
	State           ProxyState `json:"state"`
	StateStr        string     `json:"state_str"`
	Config          *Config    `json:"config"`
	RawConnections  int        `json:"raw_connections"`
}
