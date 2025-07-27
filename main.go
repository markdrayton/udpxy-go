package main

import (
	"bufio"
	"log"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/ipv4"
)

const bufsize = 2048

type client struct {
	client string
	source string
	start  time.Time
	bytes  uint64
}

type proxy struct {
	intf    *net.Interface
	clients []*client
	mu      sync.Mutex
}

func (p *proxy) addClient(client *client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.clients = append(p.clients, client)
}

func (p *proxy) removeClient(client *client) {
	p.mu.Lock()
	defer p.mu.Unlock()
	idx := slices.Index(p.clients, client)
	p.clients = append(p.clients[:idx], p.clients[idx+1:]...)
}

func (p *proxy) proxyHandler(w http.ResponseWriter, r *http.Request) {
	hostPort := strings.TrimPrefix(r.URL.Path, "/udp/")
	addr, err := net.ResolveUDPAddr("udp", hostPort)
	if err != nil {
		log.Fatalf("failed to resolve %s: %v", hostPort, err)
	}

	conn, err := open(addr)
	if err != nil {
		log.Fatal(err)
	}

	group := addr.IP
	if err := conn.JoinGroup(p.intf, &net.UDPAddr{IP: group}); err != nil {
		log.Fatalf("couldn't join group %v: %v", group, err)
	}
	if err := conn.SetControlMessage(ipv4.FlagDst, true); err != nil {
		log.Fatal(err)
	}

	client := &client{
		client: r.RemoteAddr,
		source: addr.String(),
		start:  time.Now(),
	}

	p.addClient(client)
	defer p.removeClient(client)

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Transfer-Encoding", "identity")
	w.WriteHeader(http.StatusOK)

	writer := bufio.NewWriter(w)
	for {
		b := make([]byte, bufsize)
		rn, cm, _, err := conn.ReadFrom(b)
		if err != nil {
			log.Printf("failed to read from UDP conn: %v", err)
			return
		}
		if cm.Dst.IsMulticast() && cm.Dst.Equal(group) {
			wn, err := writer.Write(b[:rn])
			if err != nil {
				log.Printf("failed to write to HTTP conn: %v", err)
				return
			}

			if err := writer.Flush(); err != nil {
				log.Printf("failed to flush: %v", err)
				return
			}
			client.bytes += uint64(wn)
		}
	}
}

func open(addr *net.UDPAddr) (*ipv4.PacketConn, error) {
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
	if err != nil {
		log.Fatal(err)
	}
	if err := syscall.SetsockoptInt(fd, syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1); err != nil {
		log.Fatal(err)
	}

	sa := syscall.SockaddrInet4{Port: addr.Port}
	copy(sa.Addr[:], addr.IP.To4())

	if err := syscall.Bind(fd, &sa); err != nil {
		syscall.Close(fd)
		log.Fatal(err)
	}

	file := os.NewFile(uintptr(fd), "")
	conn, err := net.FilePacketConn(file)
	file.Close()
	if err != nil {
		log.Fatal(err)
	}

	return ipv4.NewPacketConn(conn), nil
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s interface", os.Args[0])
	}

	intf, err := net.InterfaceByName(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}

	p := &proxy{intf: intf}
	http.HandleFunc("/udp/", p.proxyHandler)
	http.HandleFunc("/stats", p.statsHandler)
	log.Fatal(http.ListenAndServe(":4022", nil))
}
