package resp

import (
	"bytes"
	"strconv"

	"redisgreen.net/respio"
)

type MessageOp int

const (
	MsgOpUnchecked = MessageOp(iota)
	MsgOpAuth
	MsgOpSelect
	MsgOpSync
	MsgOpPsync
	MsgOpBroken
	MsgOpOther
)

var msgOps = [...]string{
	"unchecked",
	"auth",
	"select",
	"sync",
	"psync",
	"-broken-",
	"-other-",
}

func (m MessageOp) String() string {
	return msgOps[m]
}

var msgPrefixMap = []struct {
	prefix []byte
	op     MessageOp
}{
	{[]byte("*2\r\n$4\r\nAUTH\r\n$"), MsgOpAuth},
	{[]byte("*2\r\n$6\r\nSELECT\r\n$"), MsgOpSelect},
	{[]byte("*1\r\n$4\r\nSYNC\r\n"), MsgOpSync},
	{[]byte("*3\r\n$5\r\nPSYNC\r\n$"), MsgOpPsync},
}

var (
	MsgOk            = []byte("+OK\r\n")
	MsgNoAuth        = []byte("-NOAUTH Authentication required.\r\n")
	MsgInvalidPass   = []byte("-ERR invalid password\r\n")
	MsgNoPasswordSet = []byte("-ERR Client sent AUTH, but no password is set\r\n")
	MsgParseError    = []byte("-ERR Command parse error (redis-proxy)\r\n")
)

type Msg struct {
	data []byte

	op          MessageOp
	firstArg    string
	firstArgInt int
}

func NewMsg(data []byte) *Msg {
	return &Msg{data: data}
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
	return bytes.Equal(m.data, MsgOk)
}

func (m *Msg) analyse() {
	if m.op != MsgOpUnchecked {
		return
	}

	m.op = MsgOpOther
	for _, def := range msgPrefixMap {
		if len(def.prefix) > len(m.data) {
			continue
		}
		if bytes.EqualFold(def.prefix, m.data[:len(def.prefix)]) {
			m.op = def.op

			suff := m.data[len(def.prefix):]
			end := bytes.IndexByte(suff, '\r')
			if end == -1 { // no args
				// should this command have args?
				if !bytes.HasPrefix(m.data, []byte("*1\r")) {
					// yes, it should
					m.op = MsgOpBroken
				}
				return
			}
			n, err := strconv.Atoi(string(suff[:end]))
			if err != nil { // length of first arg is not an int
				m.op = MsgOpBroken
				return
			}
			m.firstArg = string(suff[end+2 : end+2+n])
		}
	}

	if m.op == MsgOpSelect {
		var err error
		m.firstArgInt, err = strconv.Atoi(m.firstArg)
		if err != nil {
			m.op = MsgOpBroken
			m.firstArg = ""
		}
	}
}
