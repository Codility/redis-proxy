package main

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

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

	rdbFile := fmt.Sprintf("save-%d-%d.rdb", r.port, time.Now().Unix())
	os.Remove(rdbFile) // ignore errors
	r.cmd = exec.Command(
		"redis-server",
		"./redis.conf",
		"--dbfilename", rdbFile,
		"--port", strconv.Itoa(r.port),
	)

	handleProcessOutput(r.cmd, fmt.Sprintf("[Redis-%d:stdout]", r.port), fmt.Sprintf("[Redis-%d:stderr]", r.port))

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
	if master == nil {
		log.Printf("Redis[%d].SlaveOf(none)", r.port)
	} else {
		log.Printf("Redis[%d].SlaveOf(%d)", r.port, master.port)
	}
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
