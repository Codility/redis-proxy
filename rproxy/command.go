package rproxy

type command int

const (
	CmdPause = command(iota)
	CmdUnpause
	CmdReload
	CmdStop
	CmdTerminateRawConnections
)

type commandCall struct {
	cmd         command
	respChannel chan commandResponse
}

type commandResponse struct {
	err error
}

func (c *commandCall) Return(err error) {
	c.respChannel <- commandResponse{err}
}
