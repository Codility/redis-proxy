package rproxy

////////////////////////////////////////
// TestConfigHolder

type TestConfigHolder struct {
	config *ProxyConfig
}

func (ch *TestConfigHolder) GetConfig() *ProxyConfig {
	return ch.config
}

func (ch *TestConfigHolder) ReloadConfig() {}

////////////////////////////////////////
// TestRequest

type TestRequest struct {
	contr *ProxyController
	done  bool
	block func()
}

func NewTestRequest(contr *ProxyController, block func()) *TestRequest {
	return &TestRequest{contr: contr, block: block}
}

func (r *TestRequest) Do() {
	r.contr.CallUplink(func() (*RespMsg, error) {
		return nil, nil
	})
	r.done = true
}
