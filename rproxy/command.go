package rproxy

type ProxyCommand int

const (
	CmdPause = ProxyCommand(iota)
	CmdUnpause
	CmdReload
	CmdStop
)
