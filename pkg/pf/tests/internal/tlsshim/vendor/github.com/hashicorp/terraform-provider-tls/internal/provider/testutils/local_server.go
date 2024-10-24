package testutils

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/elazarl/goproxy"
	"github.com/elazarl/goproxy/ext/auth"
	r "github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// LocalServerTest is a simple HTTP server used for testing.
type LocalServerTest struct {
	listener      net.Listener
	server        *http.Server
	connActivated int
	connClosed    int
}

// NewHTTPServer creates an HTTP server that listens on a random port.
func NewHTTPServer() (*LocalServerTest, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return nil, err
	}

	// Create HTTP server, listening on a randomly-selected port
	localServer := &LocalServerTest{
		listener: listener,
		server: &http.Server{
			Addr: listener.Addr().String(),
		},
	}

	// Count connections activated and closed
	localServer.server.ConnState = func(conn net.Conn, state http.ConnState) {
		if state == http.StateActive {
			localServer.connActivated++
		}
		if state == http.StateClosed {
			localServer.connClosed++
		}
	}

	return localServer, nil
}

// NewHTTPProxyServer creates an HTTP Proxy server that listens on a random port.
func NewHTTPProxyServer() (*LocalServerTest, error) {
	localServer, err := NewHTTPServer()
	if err != nil {
		return nil, err
	}

	// Turn http server into a proxy
	localServer.server.Handler = goproxy.NewProxyHttpServer()

	return localServer, nil
}

// NewHTTPProxyServerWithBasicAuth creates an HTTP Proxy server that listens on a random port and expects HTTP Basic Auth.
func NewHTTPProxyServerWithBasicAuth(expectedUsername, expectedPassword string) (*LocalServerTest, error) {
	proxy, err := NewHTTPProxyServer()
	if err != nil {
		return nil, err
	}

	// Add "HTTP Connect auth handler" to proxy server
	proxy.server.Handler.(*goproxy.ProxyHttpServer).OnRequest().HandleConnect(auth.BasicConnect("restricted", func(username, password string) bool {
		return username == expectedUsername && (expectedPassword == "" || password == expectedPassword)
	}))

	return proxy, nil
}

// ServeTLS makes the server begin listening for TLS client connections.
func (lst *LocalServerTest) ServeTLS() {
	err := lst.server.ServeTLS(lst.listener, "fixtures/public.pem", "fixtures/private.pem")
	if err != nil {
		log.Println("Failed to start LocalServerTest with TLS", err)
	}
}

// Serve makes the server begin listening for plain client connections.
func (lst *LocalServerTest) Serve() {
	err := lst.server.Serve(lst.listener)
	if err != nil {
		log.Println("Failed to start LocalServerTest", err)
	}
}

func (lst *LocalServerTest) Close() error {
	if err := lst.listener.Close(); err != nil {
		return err
	}
	if err := lst.server.Close(); err != nil {
		return err
	}
	return nil
}

func (lst *LocalServerTest) Address() string {
	return lst.listener.Addr().String()
}

func (lst *LocalServerTest) ConnActivated() int {
	return lst.connActivated
}

func (lst *LocalServerTest) ConnClosed() int {
	return lst.connClosed
}

func TestCheckBothServerAndProxyWereUsed(server, proxy *LocalServerTest) r.TestCheckFunc {
	return func(_ *terraform.State) error {
		if server.ConnActivated() != proxy.ConnActivated() {
			return fmt.Errorf("expected server and proxy actived connection count to match: server was %d, while proxy was %d", server.ConnActivated(), proxy.ConnActivated())
		}
		if server.ConnClosed() != proxy.ConnClosed() {
			return fmt.Errorf("expected server and proxy closed connection count to match: server was %d, while proxy was %d", server.ConnClosed(), proxy.ConnClosed())
		}
		return nil
	}
}
