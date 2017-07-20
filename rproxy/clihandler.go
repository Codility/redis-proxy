package rproxy

import (
	"log"
	"time"

	"github.com/Codility/redis-proxy/resp"
)

type CliHandler struct {
	proxy   *Proxy
	cliConn *resp.Conn

	done             bool
	cliAuthenticated bool
	db               int
	uplinkConf       *AddrSpec
	uplinkConn       *resp.Conn
}

func NewCliHandler(cliConn *resp.Conn, proxy *Proxy) *CliHandler {
	return &CliHandler{cliConn: cliConn, proxy: proxy}
}

func (ch *CliHandler) Run() {
	log.Printf("Handling new client: connection from %s", ch.cliConn.RemoteAddr())

	defer func() {
		ch.cliConn.Close()
		if ch.uplinkConn != nil {
			ch.uplinkConn.Close()
		}
	}()

	for !ch.done {
		req := ch.readMsgFromClient()
		if req == nil {
			continue
		}
		ch.handleRequest(req)
	}
}

func (ch *CliHandler) dialUplink(config *Config) error {
	if ch.uplinkConn != nil {
		ch.uplinkConn.Close()
		ch.uplinkConn = nil
	}

	conn, err := config.Uplink.Dial()
	if err != nil {
		return err
	}
	ch.uplinkConn = resp.NewConn(conn,
		config.ReadTimeLimitMs,
		config.LogMessages,
	)
	return nil
}

func (ch *CliHandler) readMsgFromClient() *resp.Msg {
	req, err := ch.cliConn.ReadMsg()
	if err != nil {
		ch.done = true
		log.Printf("Could not read from %s: %v\n",
			ch.cliConn.RemoteAddr().String(),
			err)
		return nil
	}
	return req
}

func (ch *CliHandler) writeToClient(data []byte) bool {
	_, err := ch.cliConn.Write(data)
	if err != nil {
		log.Printf("Could not write to %s: %v\n",
			ch.cliConn.RemoteAddr().String(),
			err)
		ch.done = true
		return false
	}
	return true
}

func (ch *CliHandler) preprocessRequest(req *resp.Msg) bool {
	if req.Op() == resp.MsgOpBroken {
		ch.writeToClient(resp.MsgParseError)
		ch.done = true
		return false
	}

	if req.Op() == resp.MsgOpAuth {
		if ch.proxy.RequiresClientAuth() {
			ch.cliAuthenticated = (req.FirstArg() == ch.proxy.config.Listen.Pass)
			if ch.cliAuthenticated {
				ch.writeToClient(resp.MsgOk)
			} else {
				ch.writeToClient(resp.MsgInvalidPass)
			}
		} else {
			ch.writeToClient(resp.MsgNoPasswordSet)
		}
		return false
	}

	if ch.proxy.RequiresClientAuth() && !ch.cliAuthenticated {
		ch.writeToClient(resp.MsgNoAuth)
		return false
	}

	return true
}

func (ch *CliHandler) postprocessRequest(req, res *resp.Msg) {
	if (req.Op() == resp.MsgOpSelect) && res.IsOk() {
		ch.db = req.FirstArgInt()
	}
}

func callAndMeasure(callable func() error) (time.Duration, error) {
	startTs := time.Now()
	err := callable()
	return time.Since(startTs), err
}

func (ch *CliHandler) handleRequest(req *resp.Msg) {
	startTs := time.Now()
	redisCallDuration := time.Duration(0)
	defer func() {
		statRecordRequest(time.Since(startTs), redisCallDuration)
	}()

	if !ch.preprocessRequest(req) {
		return
	}

	res, err := ch.proxy.CallUplink(func() (*resp.Msg, error) {
		config := ch.proxy.config
		currUplinkConf := &config.Uplink
		if (ch.uplinkConf == nil) || *ch.uplinkConf != *currUplinkConf {
			ch.uplinkConf = currUplinkConf

			duration, err := callAndMeasure(func() error { return ch.dialUplink(config) })
			redisCallDuration += duration
			if err != nil {
				return nil, err
			}

			if ch.uplinkConf.Pass != "" {
				duration, err := callAndMeasure(func() error { return ch.uplinkConn.Authenticate(ch.uplinkConf.Pass) })
				redisCallDuration += duration
				if err != nil {
					return nil, err
				}
			}

			if ch.db != 0 {
				duration, err := callAndMeasure(func() error { return ch.uplinkConn.Select(ch.db) })
				redisCallDuration += duration
				if err != nil {
					return nil, err
				}
			}
		}

		redisReqTs := time.Now()
		_, err := ch.uplinkConn.WriteMsg(req)
		if err != nil {
			redisCallDuration += time.Since(redisReqTs)
			return nil, err
		}
		resp, err := ch.uplinkConn.ReadMsg()
		redisCallDuration += time.Since(redisReqTs)
		return resp, err
	})
	if err != nil {
		log.Printf("Error: %v\n", err)
		return
	}
	ch.postprocessRequest(req, res)
	ch.writeToClient(res.Data())
}
