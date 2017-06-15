package rproxy

import (
	"io"
	"log"

	"gitlab.codility.net/marcink/redis-proxy/resp"
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

		if !ch.preprocessRequest(req) {
			continue
		}

		res, err := ch.proxy.controller.CallUplink(func() (*resp.Msg, error) {
			config := ch.proxy.config
			currUplinkConf := &config.Uplink
			if (ch.uplinkConf == nil) || !ch.uplinkConf.Equal(currUplinkConf) {
				ch.uplinkConf = currUplinkConf
				if ch.uplinkConn != nil {
					ch.uplinkConn.Close()
				}
				var err error
				ch.uplinkConn, err = resp.Dial("tcp", ch.uplinkConf.Addr,
					config.ReadTimeLimitMs,
					config.LogMessages,
				)
				if err != nil {
					return nil, err
				}

				if ch.uplinkConf.Pass != "" {
					if err := ch.uplinkConn.Authenticate(ch.uplinkConf.Pass); err != nil {
						return nil, err
					}
				}

				if ch.db != 0 {
					if err := ch.uplinkConn.Select(ch.db); err != nil {
						return nil, err
					}
				}
			}

			_, err := ch.uplinkConn.WriteMsg(req)
			if err != nil {
				return nil, err
			}
			return ch.uplinkConn.ReadMsg()
		})
		if err != nil {
			log.Printf("Error: %v\n", err)
			return
		}
		ch.postprocessRequest(req, res)
		ch.writeToClient(res.Data())
	}
}

func (ch *CliHandler) readMsgFromClient() *resp.Msg {
	req, err := ch.cliConn.ReadMsg()
	if err != nil {
		if err != io.EOF {
			log.Printf("Could not read from %s: %v\n",
				ch.cliConn.RemoteAddr().String(),
				err)
			ch.done = true
		}
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
