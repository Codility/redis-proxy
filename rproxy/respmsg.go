package rproxy

import (
	"bytes"
	"strconv"

	"redisgreen.net/resp"
)

type MessageOp int

const (
	MSG_OP_UNCHECKED = MessageOp(iota)
	MSG_OP_AUTH
	MSG_OP_BROKEN
	MSG_OP_OTHER
)

func (m MessageOp) String() string {
	switch m {
	case MSG_OP_AUTH:
		return "AUTH"
	case MSG_OP_OTHER:
		return "OTHER"
	default:
		return "?"
	}
}

type RespMsg struct {
	data []byte

	op       MessageOp
	firstArg string
}

func RespMsgFromStrings(args ...string) *RespMsg {
	buf := new(bytes.Buffer)
	resp.NewRESPWriter(buf).WriteCommand(args...)
	return &RespMsg{data: buf.Bytes()}
}

func (m *RespMsg) String() string {
	return string(m.data)
}

////////////////////
// Message analysis.
//
// Majority of messages have no meaning to the proxy and it does not
// make any sense to parse them.

func (m *RespMsg) Op() MessageOp {
	m.analyse()
	return m.op
}

func (m *RespMsg) Password() string {
	if m.op == MSG_OP_AUTH {
		return m.firstArg
	}
	return ""
}

var PREFIX_AUTH []byte

func init() {
	PREFIX_AUTH = []byte("*2\r\n$4\r\nAUTH\r\n$")
}

func (m *RespMsg) analyse() {
	if m.op != MSG_OP_UNCHECKED {
		return
	}

	m.op = MSG_OP_OTHER

	if bytes.EqualFold(m.data[:len(PREFIX_AUTH)], PREFIX_AUTH) {
		m.op = MSG_OP_AUTH
		suff := m.data[len(PREFIX_AUTH):]

		end := bytes.IndexByte(suff, '\r')
		n, err := strconv.Atoi(string(suff[:end]))
		if err != nil {
			m.op = MSG_OP_BROKEN
			return
		}
		m.firstArg = string(suff[end+2 : end+2+n])
	}
}
