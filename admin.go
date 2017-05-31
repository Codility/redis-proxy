package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
)

func (proxy *RedisProxy) publishAdminInterface() {
	config := proxy.config
	fmt.Printf("Admin URL: http://%s/\n", config.AdminOn)
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
		<form action="." method="POST">
			<button type="submit" name="cmd" value="pause">pause</button>
			<button type="submit" name="cmd" value="unpause">unpause</button>
		</form>
	</body>
</html>
`

func (proxy *RedisProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	st := proxy.getControllerState()
	ctx := map[string]interface{}{
		"activeRequests": st.activeRequests,
		"stateStr":       st.stateStr,
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
