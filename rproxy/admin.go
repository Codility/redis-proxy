package rproxy

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type AdminUI struct {
	Addr net.Addr

	proxy  *Proxy
	server *http.Server
}

func NewAdminUI(proxy *Proxy) *AdminUI {
	return &AdminUI{proxy: proxy}
}

func (a *AdminUI) Start() error {
	config := a.proxy.GetConfig()

	ln, err := config.Admin.Listen()
	if err != nil {
		return err
	}
	a.Addr = ln.Addr()

	proto := "http"
	if config.Admin.TLS {
		proto = "https"
	}
	log.Printf("Admin URL: %s://%s/\n", proto, config.Admin.Addr)

	a.server = &http.Server{
		Addr:      config.Admin.Addr,
		TLSConfig: config.Admin.GetTLSConfig(),
		Handler:   a.buildMux(),
	}

	go func() {
		err := a.server.Serve(ln)
		if err != http.ErrServerClosed {
			log.Fatal("server.Serve returned error: ", err)
		}
	}()

	return nil
}

func (a *AdminUI) Stop() {
	a.server.Close()
}

func (a *AdminUI) buildMux() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/cmd/", a.handleHTTPCmd)
	mux.HandleFunc("/status.json", a.handleHTTPStatusJSON)
	mux.HandleFunc("/info.json", a.handleHTTPInfo)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		a.handleHTTPStatusHTML(w, r)
	})
	mux.Handle("/metrics/", promhttp.Handler())
	return mux
}

var statusTemplate *template.Template

const statusHtml = `<!DOCTYPE html>
<html>
	<head>
		<title>Proxy status</title>
	</head>
	<body>
		<pre>
State: {{.stateStr}}
{{.info}}
		</pre>
		<div>As JSON: <a href="info.json">here</a></div>
		<div>Metrics: <a href="/metrics/">prometheus endpoint</a></div>
		<form action="/cmd/" method="POST">
			<button type="submit" name="cmd" value="pause">pause</button>
			<button type="submit" name="cmd" value="unpause">unpause</button>
			<button type="submit" name="cmd" value="reload">reload [=pause+reload config+unpause]</button>
			<button type="submit" name="cmd" value="terminate-raw-connections">terminate raw connections</button>
		</form>
	</body>
</html>
`

func (a *AdminUI) handleHTTPStatusHTML(w http.ResponseWriter, r *http.Request) {
	info := a.proxy.GetInfo()
	infoBytes, _ := json.MarshalIndent(info, "", "    ")

	err := statusTemplate.Execute(w, map[string]interface{}{
		"info": string(infoBytes),
	})
	if err != nil {
		panic(err)
	}
}

func (a *AdminUI) handleHTTPStatusJSON(w http.ResponseWriter, r *http.Request) {
	info := a.proxy.GetInfo()
	infoBytes, _ := json.MarshalIndent(map[string]interface{}{
		"Warning":         "DEPRECATED, DO NOT USE THIS FILE, use /info.json instead",
		"ActiveRequests":  info.ActiveRequests,
		"WaitingRequests": info.WaitingRequests,
		"State":           info.State,
		"Config":          info.Config,
		"RawConnections":  info.RawConnections,
	}, "", "    ")
	w.Write(infoBytes)
}

func (a *AdminUI) handleHTTPInfo(w http.ResponseWriter, r *http.Request) {
	infoBytes, _ := json.MarshalIndent(a.proxy.GetInfo(), "", "    ")
	w.Write(infoBytes)
}

type JsonHttpResponse struct {
	Ok    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func respond(w http.ResponseWriter, status int, errStr string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(JsonHttpResponse{
		Ok:    status == http.StatusOK,
		Error: errStr,
	})
}

func call(w http.ResponseWriter, block func() error) {
	defer func() {
		err := recover()
		if err != nil {
			log.Printf("Caught an internal error while handling API call: %s", err)
			respond(w, http.StatusInternalServerError, "Internal error; try again later")
		}
	}()

	err := block()
	if err != nil {
		respond(w, http.StatusBadRequest, err.Error())
	} else {
		respond(w, http.StatusOK, "")
	}
}

func (a *AdminUI) handleHTTPCmd(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}

	r.ParseForm()
	cmd := r.Form["cmd"][0]
	switch cmd {
	case "pause":
		call(w, a.proxy.Pause)
	case "unpause":
		call(w, a.proxy.Unpause)
	case "reload":
		call(w, a.proxy.Reload)
	case "terminate-raw-connections":
		call(w, a.proxy.TerminateRawConnections)
	default:
		respond(w, http.StatusBadRequest, fmt.Sprintf("Unknown cmd: '%s'", cmd))
	}
}

func init() {
	var err error
	statusTemplate, err = template.New("status").Parse(statusHtml)
	if err != nil {
		panic(err)
	}
}
