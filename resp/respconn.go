package resp

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"time"

	"redisgreen.net/respio"
)

type Conn struct {
	raw    net.Conn
	reader *respio.RESPReader
	writer *bufio.Writer
	log    bool

	readTimeLimitMs int64
}

func NewConn(rawConn net.Conn, readTimeLimitMs int64, log bool) *Conn {
	return &Conn{
		raw:    rawConn,
		log:    log,
		reader: respio.NewReader(bufio.NewReader(rawConn)),
		writer: bufio.NewWriter(rawConn),
	}
}

func Dial(proto, addr string, readTimeLimitMs int64, log bool) (*Conn, error) {
	conn, err := net.Dial(proto, addr)
	if err == nil {
		return NewConn(conn, readTimeLimitMs, log), nil
	} else {
		return nil, err
	}
}

func MustDial(proto, addr string, readTimeLimitMs int64, log bool) *Conn {
	conn, err := Dial(proto, addr, readTimeLimitMs, log)
	if err != nil {
		panic(err)
	}
	return conn
}

func (rc *Conn) Write(data []byte) (int, error) {
	if rc.log {
		rc.logMessage(false, data)
	}
	res, err := rc.raw.Write(data)
	if err != nil {
		return 0, err
	}
	err = rc.writer.Flush()
	return res, err
}

func (rc *Conn) MustWrite(data []byte) int {
	res, err := rc.Write(data)
	if err != nil {
		panic(err)
	}
	return res
}

func (rc *Conn) WriteMsg(msg *Msg) (int, error) {
	return rc.Write(msg.data)
}

func (rc *Conn) MustWriteMsg(msg *Msg) int {
	res, err := rc.WriteMsg(msg)
	if err != nil {
		panic(err)
	}
	return res
}

func (rc *Conn) Read(p []byte) (n int, err error) {
	return rc.raw.Read(p)
}

func (rc *Conn) ReadMsg() (*Msg, error) {
	if rc.readTimeLimitMs > 0 {
		rc.raw.SetReadDeadline(time.Now().Add(time.Duration(rc.readTimeLimitMs) * time.Millisecond))
	}
	res, err := rc.reader.ReadObject()
	if rc.log {
		if err != nil {
			rc.logMessage(true, []byte(fmt.Sprintf("err: %s", err)))
		} else {
			rc.logMessage(true, res)
		}
	}
	return &Msg{data: res}, err
}

func (rc *Conn) MustReadMsg() *Msg {
	res, err := rc.ReadMsg()
	if err != nil {
		panic(err)
	}
	return res
}

func (rc *Conn) Call(req *Msg) (*Msg, error) {
	_, err := rc.WriteMsg(req)
	if err != nil {
		return nil, err
	}

	return rc.ReadMsg()
}

func (rc *Conn) MustCall(req *Msg) *Msg {
	resp, err := rc.Call(req)
	if err != nil {
		panic(err)
	}
	return resp
}

func (rc *Conn) MustCallAndGetOk(req *Msg) {
	resp := rc.MustCall(req)
	if !resp.IsOk() {
		panic("Expected +OK from Redis, got: " + resp.String())
	}
}

func (rc *Conn) Close() error {
	rc.writer.Flush()
	return rc.raw.Close()
}

func (rc *Conn) RemoteAddr() net.Addr {
	return rc.raw.RemoteAddr()
}

func (rc *Conn) Authenticate(pass string) error {
	resp, err := rc.Call(MsgFromStrings("AUTH", pass))
	if err != nil {
		return err
	}
	if !resp.IsOk() {
		return fmt.Errorf(
			"Authentication error: Redis responded with '%s'",
			resp.String())
	}
	return nil
}

func (rc *Conn) Select(db int) error {
	resp, err := rc.Call(MsgFromStrings("SELECT", strconv.Itoa(db)))
	if err != nil {
		return err
	}
	if !resp.IsOk() {
		return fmt.Errorf(
			"SELECT error: Redis responded with '%s'",
			resp.String())
	}
	return nil
}

func (rc *Conn) logMessage(inbound bool, data []byte) {
	dirStr := "<"
	if inbound {
		dirStr = ">"
	}

	msgStr := string(data)
	msgStr = strings.Replace(msgStr, "\n", "\\n", -1)
	msgStr = strings.Replace(msgStr, "\r", "\\r", -1)

	log.Printf("%s %s %s", rc.raw.RemoteAddr(), dirStr, msgStr)
}
