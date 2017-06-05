package main

import (
	"bufio"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"

	"gitlab.codility.net/marcink/redis-proxy/rproxy"
)

type Redis struct {
	port int

	cmd *exec.Cmd
}

func NewRedis(port int) *Redis {
	return &Redis{port: port}
}

func (r *Redis) Url() string {
	return fmt.Sprintf("localhost:%d", r.port)
}

func (r *Redis) Start() {
	if r.cmd != nil {
		panic("Redis already running")
	}
	r.cmd = exec.Command("redis-server", "--port", strconv.Itoa(r.port))
	if err := r.cmd.Start(); err != nil {
		panic(err)
	}
}

func (r *Redis) Stop() {
	if r.cmd == nil {
		return
	}
	if err := r.cmd.Process.Kill(); err != nil {
		panic(err)
	}
	if _, err := r.cmd.Process.Wait(); err != nil {
		panic(err)
	}
	r.cmd = nil
}

func (r *Redis) SlaveOf(master *Redis) {
	conn, err := net.Dial("tcp", r.Url())
	if err != nil {
		panic(err)
	}

	reader := rproxy.NewReader(bufio.NewReader(conn))
	writer := bufio.NewWriter(conn)

	if master == nil {
		writer.Write([]byte("SLAVEOF NO ONE\n"))
	} else {
		writer.Write([]byte(fmt.Sprintf("SLAVEOF localhost %d\n", master.port)))
	}
	writer.Flush()
	resp, err := reader.ReadObject()
	if err != nil {
		panic(err)
	}
	if strings.TrimSpace(string(resp)) != "+OK" {
		panic("REDIS error: " + string(resp))
	}
}
