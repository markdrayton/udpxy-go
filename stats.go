package main

import (
	"fmt"
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
				<th>Client</th><th>Source</th><th>Elapsed</th><th>Bytes read</th>
			</tr>
			{{range .}}
			<tr>
				<td>{{.Client}}</td>
				<td>{{.Source}}</td>
				<td>{{.Elapsed}}</td>
				<td>{{.Bytes}}</td>
			</tr>
			{{end}}
		</table>
	</body>
</html>`))

func humanizeBytes(bytes uint64) string {
	switch {
	case bytes > (1024 * 1024):
		return fmt.Sprintf("%.f MiB", float64(bytes)/1024/1024)
	case bytes > 1024:
		return fmt.Sprintf("%.f KiB", float64(bytes)/1024)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func (p *proxy) statsHandler(w http.ResponseWriter, r *http.Request) {
	type client struct {
		Client  string
		Source  string
		Elapsed string
		Bytes   string
	}
	stats := []client{}
	p.mu.Lock()
	for _, c := range p.clients {
		stats = append(stats, client{
			Client:  c.client,
			Source:  c.source,
			Elapsed: time.Since(c.start).Round(time.Second).String(),
			Bytes:   humanizeBytes(c.bytes),
		})
	}
	p.mu.Unlock()
	err := tmpl.Execute(w, stats)
	if err != nil {
		log.Printf("failed to execute template: %v", err)
		return
	}
}
