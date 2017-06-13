package rproxy

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	"redisgreen.net/resp"
)

type RespConn struct {
	raw    net.Conn
	reader *resp.RESPReader
	writer *bufio.Writer
	log    bool

	readTimeLimitMs int64
}

func NewRespConn(rawConn net.Conn, readTimeLimitMs int64, log bool) *RespConn {
	return &RespConn{
		raw:    rawConn,
		log:    log,
		reader: resp.NewReader(bufio.NewReader(rawConn)),
		writer: bufio.NewWriter(rawConn),
	}
}

func RespDial(proto, addr string, readTimeLimitMs int64, log bool) (*RespConn, error) {
	conn, err := net.Dial(proto, addr)
	if err == nil {
		return NewRespConn(conn, readTimeLimitMs, log), nil
	} else {
		return nil, err
	}
}

func MustRespDial(proto, addr string, readTimeLimitMs int64, log bool) *RespConn {
	conn, err := RespDial(proto, addr, readTimeLimitMs, log)
	if err != nil {
		panic(err)
	}
	return conn
}

func (rc *RespConn) Write(data []byte) (int, error) {
	if rc.log {
		rc.logMessage(false, data)
	}
	res, err := rc.raw.Write(data)
	if err == nil {
		rc.writer.Flush()
	}
	return res, nil
}

func (rc *RespConn) WriteMsg(msg *RespMsg) (int, error) {
	return rc.Write(msg.data)
}

func (rc *RespConn) MustWriteMsg(msg *RespMsg) int {
	res, err := rc.WriteMsg(msg)
	if err != nil {
		panic(err)
	}
	return res
}

func (rc *RespConn) ReadMsg() (*RespMsg, error) {
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
	return &RespMsg{data: res}, err
}

func (rc *RespConn) MustReadMsg() *RespMsg {
	res, err := rc.ReadMsg()
	if err != nil {
		panic(err)
	}
	return res
}

func (rc *RespConn) Call(req *RespMsg) (*RespMsg, error) {
	_, err := rc.WriteMsg(req)
	if err != nil {
		return nil, err
	}

	return rc.ReadMsg()
}

func (rc *RespConn) MustCall(req *RespMsg) *RespMsg {
	resp, err := rc.Call(req)
	if err != nil {
		panic(err)
	}
	return resp
}

func (rc *RespConn) Close() error {
	rc.writer.Flush()
	return rc.raw.Close()
}

func (rc *RespConn) RemoteAddr() net.Addr {
	return rc.raw.RemoteAddr()
}

func (rc *RespConn) logMessage(inbound bool, data []byte) {
	dirStr := "<"
	if inbound {
		dirStr = ">"
	}

	msgStr := string(data)
	msgStr = strings.Replace(msgStr, "\n", "\\n", -1)
	msgStr = strings.Replace(msgStr, "\r", "\\r", -1)

	log.Printf("%s %s %s", rc.raw.RemoteAddr(), dirStr, msgStr)
}
