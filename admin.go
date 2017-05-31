package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
)

func (proxy *RedisProxy) publishAdminInterface() {
	config := proxy.config
	log.Printf("Admin URL: http://%s/\n", config.AdminOn)
	log.Fatal(http.ListenAndServe(config.AdminOn, proxy))
}

var statusTemplate *template.Template

const statusHtml = `
<!DOCTYPE html>
<html>
	<head>
		<title>Redis Proxy status</title>
	</head>
	<body>
		<div>Active requests: {{.activeRequests}}</div>
		<div>State: {{.stateStr}}</div>
		<pre>Config:
{{.configStr}}
		</pre>
		<form action="." method="POST">
			<button type="submit" name="cmd" value="pause">pause</button>
			<button type="submit" name="cmd" value="unpause">unpause</button>
			<button type="submit" name="cmd" value="reload">reload [=pause+reload config+unpause]</button>
		</form>
	</body>
</html>
`

func (proxy *RedisProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		r.ParseForm()
		cmd := r.Form["cmd"][0]
		switch cmd {
		case "pause":
			proxy.controller.Pause()
		case "unpause":
			proxy.controller.Unpause()
		case "reload":
			proxy.controller.Reload()
		default:
			http.Error(w, fmt.Sprintf("Unknown cmd: '%s'", cmd), http.StatusBadRequest)
			return
		}
		http.Redirect(w, r, r.URL.Path, http.StatusSeeOther)
	}

	st := proxy.controller.GetInfo()

	configBytes, _ := json.MarshalIndent(st.Config, "", "    ")
	ctx := map[string]interface{}{
		"activeRequests": st.ActiveRequests,
		"stateStr":       st.StateStr(),
		"configStr":      string(configBytes),
	}
	err := statusTemplate.Execute(w, ctx)
	if err != nil {
		panic(err)
	}
}

func init() {

	var err error
	statusTemplate, err = template.New("status").Parse(statusHtml)
	if err != nil {
		panic(err)
	}
}
