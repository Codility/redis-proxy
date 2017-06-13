package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	PROXY_A_PORT  = 7101
	PROXY_A_ADMIN = 7102

	PROXY_B_PORT  = 7201
	PROXY_B_ADMIN = 7202

	REDIS_A_PORT = 6001
	REDIS_B_PORT = 6002
)

func main() {
	log.SetFlags(log.Ltime)

	proxy_a := NewProxy("conf-a.json", PROXY_A_PORT, PROXY_A_ADMIN, REDIS_A_PORT)
	proxy_b := NewProxy("conf-b.json", PROXY_B_PORT, PROXY_B_ADMIN, PROXY_A_PORT)

	proxy_a.Start()
	defer proxy_a.Stop()

	proxy_b.Start()
	defer proxy_b.Stop()

	redis_a := NewRedis(REDIS_A_PORT)
	redis_a.Start()
	defer redis_a.Stop()

	redis_b := NewRedis(REDIS_B_PORT)
	defer redis_b.Stop()

	go statusLoop()

	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)

	for {
		<-sighup
		log.Print("Switching to Redis B")

		redis_b.Start()
		time.Sleep(time.Second)
		redis_b.SlaveOf(redis_a)

		proxy_a.PauseAndWait()
		proxy_b.PauseAndWait()
		logStatus()
		// TODO: wait for replication to catch up
		time.Sleep(time.Second)

		redis_b.SlaveOf(nil)
		redis_a.Stop()

		proxy_b.LinkTo(REDIS_B_PORT)
		time.Sleep(time.Second)
		proxy_a.LinkTo(PROXY_B_PORT)

		log.Print("Done switching to Redis B")

		<-sighup
		log.Print("Switching to Redis A")

		redis_a.Start()
		time.Sleep(time.Second)
		redis_a.SlaveOf(redis_b)

		proxy_a.PauseAndWait()
		proxy_b.PauseAndWait()
		// TODO: wait for replication to catch up
		time.Sleep(time.Second)

		redis_a.SlaveOf(nil)
		redis_b.Stop()

		proxy_a.LinkTo(REDIS_A_PORT)
		time.Sleep(time.Second)
		proxy_b.LinkTo(PROXY_A_PORT)

		log.Print("Done switching to Redis A")
	}
}

func statusLoop() {
	for {
		logStatus()
		time.Sleep(time.Second)
	}
}

func logStatus() {
	log.Printf("A: %s;  B: %s\n",
		getStatus(PROXY_A_ADMIN),
		getStatus(PROXY_B_ADMIN))
}

func getStatus(adminPort int) string {
	url := fmt.Sprintf("http://localhost:%d/status.json", adminPort)
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Sprintf("ERROR, %v", err)
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Sprintf("ERROR, %v", err)
	}

	data := map[string]interface{}{}
	if err := json.Unmarshal(bytes, &data); err != nil {
		return fmt.Sprintf("ERROR, %v", err)
	}

	portFromAddrSpec := func(addrSpec interface{}) string {
		addr := addrSpec.(map[string]interface{})["addr"].(string)
		_, p, err := net.SplitHostPort(addr)
		if err != nil {
			return fmt.Sprintf("%v", err)
		}
		return p
	}

	config := data["config"].(map[string]interface{})
	return fmt.Sprintf("%v -> %v, %v, active: %v",
		portFromAddrSpec(config["listen"]),
		portFromAddrSpec(config["uplink"]),
		data["stateStr"],
		data["activeRequests"],
	)
}
