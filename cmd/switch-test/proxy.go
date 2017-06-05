package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"syscall"
)

type Proxy struct {
	config_file                       string
	listenPort, adminPort, uplinkPort int
	cmd                               *exec.Cmd
}

func NewProxy(config_file string, listenPort, adminPort, uplinkPort int) *Proxy {
	return &Proxy{
		config_file: config_file,
		listenPort:  listenPort,
		adminPort:   adminPort,
		uplinkPort:  uplinkPort,
	}
}

func handleOutput(prefix string, output io.ReadCloser) {
	reader := bufio.NewReader(output)
	for {
		out, err := reader.ReadString('\n')
		if out != "" {
			log.Printf("%s %s", prefix, out)
		}
		if err != nil {
			return
		}
	}
}

func (p *Proxy) Start() {
	log.Printf("Proxy[%d -> %d].Start", p.listenPort, p.uplinkPort)
	if p.cmd != nil {
		panic("Proxy already running")
	}
	p.WriteConfig()
	p.cmd = exec.Command("./redis-proxy", "-f", p.config_file)

	stdout, err := p.cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	go handleOutput(fmt.Sprintf("[%d:stdout]", p.listenPort), stdout)

	stderr, err := p.cmd.StderrPipe()
	if err != nil {
		panic(err)
	}
	go handleOutput(fmt.Sprintf("[%d:stderr]", p.listenPort), stderr)

	if err := p.cmd.Start(); err != nil {
		panic(err)
	}
}

func (p *Proxy) Stop() {
	log.Printf("Proxy[%d -> %d].Stop", p.listenPort, p.uplinkPort)
	if p.cmd == nil {
		return
	}
	if err := p.cmd.Process.Kill(); err != nil {
		panic(err)
	}
	p.cmd = nil
}

func (p *Proxy) WriteConfig() {
	content := fmt.Sprintf(`{
  "uplink_addr": "localhost:%d",
  "listen_on": "127.0.0.1:%d",
  "admin_on": "127.0.0.1:%d",
  "log_messages": false,
  "read_time_limit_ms": 5000
}`, p.uplinkPort, p.listenPort, p.adminPort)
	log.Printf("Proxy[%d -> %d].WriteConfig", p.listenPort, p.uplinkPort)
	if err := ioutil.WriteFile(p.config_file, []byte(content), 0666); err != nil {
		panic(err)
	}
}

func (p *Proxy) LinkTo(newUplink int) {
	log.Printf("Proxy[%d -> %d].LinkTo(%d)", p.listenPort, p.uplinkPort, newUplink)
	p.uplinkPort = newUplink
	p.WriteConfig()
	p.Reload()
}

func (p *Proxy) Reload() {
	log.Printf("Proxy[%d -> %d].Reload", p.listenPort, p.uplinkPort)
	if p.cmd == nil {
		panic("Proxy not running")
	}
	if err := p.cmd.Process.Signal(syscall.SIGHUP); err != nil {
		panic(err)
	}
}

func (p *Proxy) PauseAndWait() {
	log.Printf("Proxy[%d -> %d].PauseAndWait", p.listenPort, p.uplinkPort)
	u := fmt.Sprintf("http://localhost:%d/cmd/", p.adminPort)
	resp, err := http.PostForm(u, url.Values{"cmd": {"pause-and-wait"}})
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		panic(resp)
	}
	log.Printf("Proxy[%d -> %d].PauseAndWait done", p.listenPort, p.uplinkPort)
}
