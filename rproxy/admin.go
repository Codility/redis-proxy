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
	Addr *net.Addr

	proxy  *Proxy
	server *http.Server
}

func NewAdminUI(proxy *Proxy) *AdminUI {
	return &AdminUI{proxy: proxy}
}

func (a *AdminUI) Start() error {
	config := a.proxy.GetConfig()

	ln, _, addr, err := config.Admin.Listen()
	if err != nil {
		return err
	}
	a.Addr = addr

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
	mux.HandleFunc("/status.json", func(w http.ResponseWriter, r *http.Request) {
		a.handleHTTPStatus(w, r, "json")
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		a.handleHTTPStatus(w, r, "html")
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
{{.}}
		</pre>
		<div>As JSON: <a href="status.json">here</a></div>
		<div>Metrics: <a href="/metrics/">prometheus endpoint</a></div>
		<form action="/cmd/" method="POST">
			<button type="submit" name="cmd" value="pause">pause</button>
			<button type="submit" name="cmd" value="unpause">unpause</button>
			<button type="submit" name="cmd" value="reload">reload [=pause+reload config+unpause]</button>
		</form>
	</body>
</html>
`

func (a *AdminUI) handleHTTPStatus(w http.ResponseWriter, r *http.Request, format string) {
	st := a.proxy.GetInfo()
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
		// TODO: limit what errors we show to users?
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
	log.Println("Received cmd:", cmd)
	switch cmd {
	case "pause":
		call(w, a.proxy.Pause)
	case "unpause":
		call(w, a.proxy.Unpause)
	case "reload":
		call(w, a.proxy.Reload)
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
