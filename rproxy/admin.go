package rproxy

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
)

func getListener(addrSpec AddrSpec) (*net.Listener, *net.Addr, error) {
	ln, err := net.Listen("tcp", addrSpec.Addr)
	if err != nil {
		log.Fatalf("Could not listen: %s", err)
		return nil, nil, err
	}
	addr := ln.(*net.TCPListener).Addr()

	tlsSpec := addrSpec.TLS
	if tlsSpec == nil {
		return &ln, &addr, nil
	}

	cer, err := tls.LoadX509KeyPair(tlsSpec.CertFile, tlsSpec.KeyFile)
	if err != nil {
		log.Fatalf("Could not load key pair (%s, %s): %s",
			tlsSpec.CertFile, tlsSpec.KeyFile, err)
		return nil, nil, err
	}
	tlsLn := tls.NewListener(ln, &tls.Config{
		Certificates: []tls.Certificate{cer},
	})
	return &tlsLn, &addr, nil
}

func (proxy *Proxy) publishAdminInterface() {
	mux := http.NewServeMux()
	mux.HandleFunc("/cmd/", proxy.handleHTTPCmd)
	mux.HandleFunc("/status.json", func(w http.ResponseWriter, r *http.Request) {
		proxy.handleHTTPStatus(w, r, "json")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		proxy.handleHTTPStatus(w, r, "html")
	})

	config := proxy.config

	ln, addr, err := getListener(config.Admin)
	if err != nil {
		log.Fatal(err)
		return
	}
	proxy.adminAddr = addr
	proto := "http"
	if config.Admin.TLS != nil {
		proto = "https"
	}
	log.Printf("Admin URL: %s://%s/\n", proto, *addr)

	go func() {
		log.Fatal(http.Serve(*ln, mux))
	}()
}

var statusTemplate *template.Template

const statusHtml = `<!DOCTYPE html>
<html>
	<head>
		<title>Proxy status</title>
	</head>
	<body>
		<pre>
{{.}}
		</pre>
		<div>As JSON: <a href="status.json">here</a></div>
		<form action="/cmd/" method="POST">
			<button type="submit" name="cmd" value="pause">pause</button>
			<button type="submit" name="cmd" value="pause-and-wait">pause and wait</button>
			<button type="submit" name="cmd" value="unpause">unpause</button>
			<button type="submit" name="cmd" value="reload">reload [=pause+reload config+unpause]</button>
		</form>
	</body>
</html>
`

func (proxy *Proxy) handleHTTPStatus(w http.ResponseWriter, r *http.Request, format string) {
	st := proxy.controller.GetInfo()
	info := map[string]interface{}{
		"activeRequests": st.ActiveRequests,
		"config":         st.Config,
		"stateStr":       st.State.String(),
	}
	infoBytes, _ := json.MarshalIndent(info, "", "    ")

	if format == "json" {
		w.Write(infoBytes)
		return
	}

	err := statusTemplate.Execute(w, string(infoBytes))
	if err != nil {
		panic(err)
	}
}

func (proxy *Proxy) handleHTTPCmd(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()
		cmd := r.Form["cmd"][0]
		log.Println("Received cmd:", cmd)
		switch cmd {
		case "pause":
			proxy.controller.Pause()
		case "pause-and-wait":
			proxy.controller.PauseAndWait()
		case "unpause":
			proxy.controller.Unpause()
		case "reload":
			proxy.controller.Reload()
		default:
			http.Error(w, fmt.Sprintf("Unknown cmd: '%s'", cmd), http.StatusBadRequest)
			return
		}
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func init() {
	var err error
	statusTemplate, err = template.New("status").Parse(statusHtml)
	if err != nil {
		panic(err)
	}
}
