package resp

import (
	"bytes"
	"strconv"

	"redisgreen.net/respio"
)

type MessageOp int

const (
	MSG_OP_UNCHECKED = MessageOp(iota)
	MSG_OP_AUTH
	MSG_OP_SELECT
	MSG_OP_BROKEN
	MSG_OP_OTHER
)

func (m MessageOp) String() string {
	switch m {
	case MSG_OP_AUTH:
		return "AUTH"
	case MSG_OP_SELECT:
		return "SELECT"
	case MSG_OP_OTHER:
		return "OTHER"
	default:
		return "?"
	}
}

type Msg struct {
	data []byte

	op       MessageOp
	firstArg string
}

func MsgFromStrings(args ...string) *Msg {
	buf := new(bytes.Buffer)
	respio.NewRESPWriter(buf).WriteCommand(args...)
	return &Msg{data: buf.Bytes()}
}

func (m *Msg) String() string {
	return string(m.data)
}

func (m *Msg) Data() []byte {
	return m.data
}

func (m *Msg) Equal(other *Msg) bool {
	return bytes.Equal(m.data, other.data)
}

////////////////////
// Message analysis.
//
// Majority of messages have no meaning to the proxy and it does not
// make any sense to parse them.

func (m *Msg) Op() MessageOp {
	m.analyse()
	return m.op
}

func (m *Msg) Password() string {
	if m.op == MSG_OP_AUTH {
		return m.firstArg
	}
	return ""
}

func (m *Msg) FirstArg() string {
	return m.firstArg
}

func (m *Msg) IsOk() bool {
	return bytes.Equal(m.data, MSG_DATA_OK)
}

var PREFIX_AUTH []byte
var PREFIX_SELECT []byte
var MSG_DATA_OK []byte

func init() {
	PREFIX_AUTH = []byte("*2\r\n$4\r\nAUTH\r\n$")
	PREFIX_SELECT = []byte("*2\r\n$6\r\nSELECT\r\n$")
	MSG_DATA_OK = []byte("+OK\r\n")
}

func (m *Msg) analyse() {
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

	if bytes.EqualFold(m.data[:len(PREFIX_SELECT)], PREFIX_SELECT) {
		m.op = MSG_OP_SELECT
		suff := m.data[len(PREFIX_SELECT):]

		end := bytes.IndexByte(suff, '\r')
		n, err := strconv.Atoi(string(suff[:end]))
		if err != nil {
			m.op = MSG_OP_BROKEN
			return
		}
		m.firstArg = string(suff[end+2 : end+2+n])
	}
}
