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
	"strconv"
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

func pass(port int) string {
	return "pass-" + strconv.Itoa(port)
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

func handleProcessOutput(cmd *exec.Cmd, stdoutPrefix, stderrPrefix string) {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		panic(err)
	}
	go handleOutput(stdoutPrefix, stdout)

	stderr, err := cmd.StderrPipe()
	if err != nil {
		panic(err)
	}
	go handleOutput(stderrPrefix, stderr)
}

func (p *Proxy) Start() {
	if p.cmd != nil {
		panic("Proxy already running")
	}
	p.WriteConfig()
	p.cmd = exec.Command("./redis-proxy", "-f", p.config_file)
	handleProcessOutput(p.cmd, fmt.Sprintf("[%d:stdout]", p.listenPort), fmt.Sprintf("[%d:stderr]", p.listenPort))

	log.Printf("Starting redis-proxy at %d, password: '%s'", p.listenPort, pass(p.listenPort))

	if err := p.cmd.Start(); err != nil {
		panic(err)
	}
}

func (p *Proxy) Stop() {
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
  "uplink": {
    "addr": "localhost:%d",
    "pass": "%s"
  },
  "listen": {
    "addr": "127.0.0.1:%d",
    "pass": "%s"
  },
  "admin": {
    "addr": "127.0.0.1:%d"
  },
  "log_messages": false,
  "read_time_limit_ms": 5000
}`, p.uplinkPort, pass(p.uplinkPort), p.listenPort, pass(p.listenPort), p.adminPort)
	if err := ioutil.WriteFile(p.config_file, []byte(content), 0666); err != nil {
		panic(err)
	}
}

func (p *Proxy) LinkTo(newUplink int) {
	p.uplinkPort = newUplink
	p.WriteConfig()
	p.Reload()
}

func (p *Proxy) Reload() {
	if p.cmd == nil {
		panic("Proxy not running")
	}
	if err := p.cmd.Process.Signal(syscall.SIGHUP); err != nil {
		panic(err)
	}
}

func (p *Proxy) Pause() {
	u := fmt.Sprintf("http://localhost:%d/cmd/", p.adminPort)
	resp, err := http.PostForm(u, url.Values{"cmd": {"pause"}})
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		panic(resp)
	}
}
