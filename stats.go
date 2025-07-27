package main

import (
	"html/template"
	"log"
	"net/http"
	"time"
)

var tmpl = template.Must(template.New("stats").Parse(`<!DOCTYPE html>
<html>
	<head>
		<title>udpxy-go</title>
		<style>
			th, td {
				padding: 5px;
			}
		</style>
	</head>
	<body>
		<table>
			<tr>
				<th>Source</th><th>Dest</th><th>Elapsed</th><th>Bytes read</th>
			</tr>
			{{range .}}
			<tr>
				<td>{{.Source}}</td>
				<td>{{.Dest}}</td>
				<td>{{.Elapsed}}</td>
				<td>{{.Bytes}}</td>
			</tr>
			{{end}}
		</table>
	</body>
</html>`))

func (p *proxy) statsHandler(w http.ResponseWriter, r *http.Request) {
	type client struct {
		Source  string
		Dest    string
		Elapsed string
		Bytes   int
	}
	stats := []client{}
	p.mu.Lock()
	for _, c := range p.clients {
		stats = append(stats, client{
			Source:  c.source,
			Dest:    c.dest,
			Elapsed: time.Since(c.start).String(),
			Bytes:   c.bytes,
		})
	}
	p.mu.Unlock()
	err := tmpl.Execute(w, stats)
	if err != nil {
		log.Printf("failed to execute template: %v", err)
		return
	}
}
