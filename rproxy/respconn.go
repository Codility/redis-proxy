package rproxy

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"strings"
	"time"

	resp "redisgreen.net/resp"
)

type RespConn struct {
	raw    net.Conn
	reader *resp.RESPReader
	writer *bufio.Writer
	log    bool

	readTimeLimitMs int64
}

type RespMsg struct {
	data []byte
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

func (rc *RespConn) WriteMsg(msg *RespMsg) (int, error) {
	if rc.log {
		rc.logMessage(false, msg.data)
	}
	res, err := rc.writer.Write(msg.data)
	if err == nil {
		rc.writer.Flush()
	}
	return res, err
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
	return &RespMsg{res}, err
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
