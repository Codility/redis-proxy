package rproxy

type ProxyCommand int

const (
	CmdPause = ProxyCommand(iota)
	CmdUnpause
	CmdReload
	CmdStop
)

type commandPack struct {
	cmd         ProxyCommand
	respChannel chan commandResponse
}

type commandResponse struct {
	err error
}

func (c *commandPack) Return(err error) {
	c.respChannel <- commandResponse{err}
}
