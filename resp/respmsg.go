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

	op          MessageOp
	firstArg    string
	firstArgInt int
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

func (m *Msg) FirstArg() string {
	return m.firstArg
}

func (m *Msg) FirstArgInt() int {
	return m.firstArgInt
}

func (m *Msg) IsOk() bool {
	return bytes.Equal(m.data, MSG_DATA_OK)
}

var MSG_PREFIX_MAP = []struct {
	prefix []byte
	op     MessageOp
}{
	{[]byte("*2\r\n$4\r\nAUTH\r\n$"), MSG_OP_AUTH},
	{[]byte("*2\r\n$6\r\nSELECT\r\n$"), MSG_OP_SELECT},
}

var MSG_DATA_OK = []byte("+OK\r\n")

func (m *Msg) analyse() {
	if m.op != MSG_OP_UNCHECKED {
		return
	}

	m.op = MSG_OP_OTHER
	for _, def := range MSG_PREFIX_MAP {
		if bytes.EqualFold(def.prefix, m.data[:len(def.prefix)]) {
			m.op = def.op

			suff := m.data[len(def.prefix):]
			end := bytes.IndexByte(suff, '\r')
			n, err := strconv.Atoi(string(suff[:end]))
			if err != nil {
				m.op = MSG_OP_BROKEN
				return
			}
			m.firstArg = string(suff[end+2 : end+2+n])
		}
	}

	if m.op == MSG_OP_SELECT {
		var err error
		m.firstArgInt, err = strconv.Atoi(m.firstArg)
		if err != nil {
			m.op = MSG_OP_BROKEN
		}
	}
}
