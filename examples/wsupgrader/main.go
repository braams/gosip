package main

import (
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gobwas/ws"

	"github.com/ghettovoice/gosip"
	"github.com/ghettovoice/gosip/log"
	"github.com/ghettovoice/gosip/transport"
)

var (
	logger log.Logger
)

func init() {
	logger = log.NewDefaultLogrusLogger().WithPrefix("Server")
}

type Connection struct {
	headers http.Header
	uri     string
	host    string
}

type Authenticator struct {
	Connections map[string]Connection
}

func NewAuthenticator() *Authenticator {
	return &Authenticator{
		Connections: map[string]Connection{},
	}
}

func (a *Authenticator) NewFactory() transport.UpgraderFactory {
	return func(conn net.Conn) ws.Upgrader {
		logger.Infof("Connection: %s", conn.RemoteAddr())
		return a.NewUpgrader(conn)
	}
}

func (a *Authenticator) NewUpgrader(conn net.Conn) ws.Upgrader {
	c := Connection{
		headers: map[string][]string{},
	}
	u := ws.Upgrader{
		Protocol: func(val []byte) bool {
			return string(val) == "sip"
		},
		OnRequest: func(uri []byte) error {
			c.uri = string(uri)
			return nil
		},
		OnHost: func(host []byte) error {
			c.host = string(host)
			return nil
		},
		OnHeader: func(key, value []byte) error {
			c.headers.Add(string(key), string(value))
			return nil
		},
		OnBeforeUpgrade: func() (header ws.HandshakeHeader, err error) {
			// any other check here
			if len(c.headers) > 0 {
				return ws.HandshakeHeaderHTTP(map[string][]string{}), nil
			} else {
				return nil, ws.RejectConnectionError(ws.RejectionStatus(http.StatusForbidden))
			}
		},
	}
	a.Connections[conn.RemoteAddr().String()] = c
	return u
}

func main() {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)

	srvConf := gosip.ServerConfig{}
	srv := gosip.NewServer(srvConf, nil, nil, logger)
	//srv.Listen("wss", "0.0.0.0:5081", &transport.TLSConfig{Cert: "certs/cert.pem", Key: "certs/key.pem"})

	a := NewAuthenticator()

	err := srv.Listen("ws", "0.0.0.0:8090", &transport.WSConfig{UpgraderFactory: a.NewFactory()})
	if err != nil {
		logger.Fatal(err)
	}
	<-stop

	srv.Shutdown()
}
