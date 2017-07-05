package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/codility/redis-proxy/resp"
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

func (r *Redis) Pass() string {
	return "pass-" + strconv.Itoa(r.port)
}

func (r *Redis) Start() {
	if r.cmd != nil {
		panic("Redis already running")
	}

	rdbFile := fmt.Sprintf("save-%d-%d.rdb", r.port, time.Now().Unix())
	args := []string{
		"-",
		"--dbfilename", rdbFile,
		"--requirepass", r.Pass(),
		"--port", strconv.Itoa(r.port),
	}
	r.cmd = exec.Command("redis-server", args...)
	handleProcessOutput(r.cmd, fmt.Sprintf("[Redis-%d:stdout]", r.port), fmt.Sprintf("[Redis-%d:stderr]", r.port))
	r.cmd.Dir = fmt.Sprintf("tmp/redis-%d/", r.port)

	if err := os.MkdirAll(r.cmd.Dir, 0777); err != nil {
		panic(err)
	}

	stdin, err := r.cmd.StdinPipe()
	if err != nil {
		panic(err)
	}
	defer stdin.Close()

	log.Printf("Starting redis-server %v", args)

	if err := r.cmd.Start(); err != nil {
		panic(err)
	}
	io.WriteString(stdin, "loglevel debug\n")
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
	conn := resp.MustDial("tcp", r.Url(), 0, false)
	conn.MustCall(resp.MsgFromStrings("AUTH", r.Pass()))

	if master == nil {
		conn.MustWrite([]byte("SLAVEOF NO ONE\r\n"))
	} else {
		if master.Pass() != "" {
			m := resp.MsgFromStrings("CONFIG", "SET", "MASTERAUTH", master.Pass())
			conn.MustCallAndGetOk(m)
		}
		conn.MustWrite([]byte(fmt.Sprintf("SLAVEOF localhost %d\r\n", master.port)))
	}
	resp := conn.MustReadMsg()
	if !resp.IsOk() {
		panic("REDIS error: " + resp.String())
	}
}
