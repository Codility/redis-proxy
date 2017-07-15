package rproxy

type ProxyState int

const (
	ProxyStopped = ProxyState(iota)
	ProxyStarting
	ProxyRunning
	ProxyPausing
	ProxyPaused
	ProxyReloading
	ProxyStopping
)

// TODO: verify consistency between Proxy* constants and proxyStateTxt
var proxyStateTxt = [...]string{
	"stopped",
	"starting",
	"running",
	"pausing",
	"paused",
	"reloading",
	"stopping",
}

func (s ProxyState) String() string {
	return proxyStateTxt[s]
}

func (s ProxyState) IsAlive() bool {
	return (s != ProxyStopped && s != ProxyStarting && s != ProxyStopping)
}
