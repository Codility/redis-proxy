package rproxy

type ProxyInfo struct {
	ActiveRequests  int        `json:"active_requests"`
	WaitingRequests int        `json:"waiting_requests"`
	State           ProxyState `json:"state"`
	StateStr        string     `json:"state_str"`
	Config          *Config    `json:"config"`
	RawConnections  int        `json:"raw_connections"`
}

func (p *ProxyInfo) SanitizedForPublication() *ProxyInfo {
	return &ProxyInfo{
		ActiveRequests:  p.ActiveRequests,
		WaitingRequests: p.WaitingRequests,
		State:           p.State,
		StateStr:        p.StateStr,
		Config:          p.Config.SanitizedForPublication(),
		RawConnections:  p.RawConnections,
	}
}
