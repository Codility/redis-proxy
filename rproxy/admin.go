package rproxy

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
)

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
	log.Printf("Admin URL: http://%s/\n", config.AdminOn)
	log.Fatal(http.ListenAndServe(config.AdminOn, mux))
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
		"stateStr":       string(st.State),
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
