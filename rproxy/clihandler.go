package rproxy

import (
	"io"
	"log"

	"gitlab.codility.net/marcink/redis-proxy/resp"
)

type CliHandler struct {
	cliConn *resp.Conn
	proxy   *Proxy
}

func NewCliHandler(cliConn *resp.Conn, proxy *Proxy) *CliHandler {
	return &CliHandler{cliConn: cliConn, proxy: proxy}
}

func (ch *CliHandler) Handle() {
	log.Printf("Handling new client: connection from %s", ch.cliConn.RemoteAddr())

	uplinkConf := &AddrSpec{}
	var uplinkConn *resp.Conn
	cliAuthenticated := false
	db := 0

	defer func() {
		ch.cliConn.Close()
		if uplinkConn != nil {
			uplinkConn.Close()
		}
	}()

	for {
		req, err := ch.cliConn.ReadMsg()
		if err != nil {
			if err != io.EOF {
				log.Printf("Read error: %v\n", err)
			}
			return
		}

		if req.Op() == resp.MsgOpBroken {
			ch.cliConn.Write(resp.MsgParseError)
			return
		}

		if req.Op() == resp.MsgOpAuth {
			if ch.proxy.RequiresClientAuth() {
				cliAuthenticated = (req.FirstArg() == ch.proxy.config.Listen.Pass)
				if cliAuthenticated {
					ch.cliConn.Write([]byte(resp.MsgOk))
				} else {
					ch.cliConn.Write([]byte(resp.MsgInvalidPass))
				}
			} else {
				ch.cliConn.Write([]byte(resp.MsgNoPasswordSet))
			}
			continue
		}

		if ch.proxy.RequiresClientAuth() && !cliAuthenticated {
			ch.cliConn.Write([]byte(resp.MsgNoAuth))
			continue
		}

		res, err := ch.proxy.controller.CallUplink(func() (*resp.Msg, error) {
			config := ch.proxy.config
			currUplinkConf := &config.Uplink
			if !uplinkConf.Equal(currUplinkConf) {
				uplinkConf = currUplinkConf
				if uplinkConn != nil {
					uplinkConn.Close()
				}
				uplinkConn, err = resp.Dial("tcp", uplinkConf.Addr,
					config.ReadTimeLimitMs,
					config.LogMessages,
				)
				if err != nil {
					return nil, err
				}

				if uplinkConf.Pass != "" {
					if err := uplinkConn.Authenticate(uplinkConf.Pass); err != nil {
						return nil, err
					}
				}

				if db != 0 {
					if err := uplinkConn.Select(db); err != nil {
						return nil, err
					}
				}
			}

			_, err := uplinkConn.WriteMsg(req)
			if err != nil {
				return nil, err
			}
			return uplinkConn.ReadMsg()
		})
		if err != nil {
			log.Printf("Error: %v\n", err)
			return
		}

		if (req.Op() == resp.MsgOpSelect) && res.IsOk() {
			db = req.FirstArgInt()
		}

		ch.cliConn.WriteMsg(res)
	}
}
