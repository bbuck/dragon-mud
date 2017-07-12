// Copyright (c) 2016-2017 Brandon Buck

package server

import (
	"net"
	"strings"

	"time"

	"github.com/bbuck/dragon-mud/logger"
	"github.com/bbuck/dragon-mud/plugins"
	"github.com/bbuck/dragon-mud/scripting"
	"github.com/spf13/viper"
)

var (
	serverRunning = false
	log           logger.Log
)

// Run prepars the telnet server and begins running it.
func Run() {
	if serverRunning {
		return
	}

	log = logger.NewWithSource("server(telnet)")
	if err := plugins.LoadViews(); err != nil {
		log.WithError(err).Error("Failed to load views")
	}
	serverRunning = true
	host := viper.GetString("telnet.interface")
	port := viper.GetString("telnet.port")

	scripting.Initialize()
	done := scripting.ServerEmitter.EmitOnce("server:init", nil)
	<-done

	listener, err := net.Listen("tcp", host+":"+port)
	if err != nil {
		log.WithError(err).Fatal("Failed to start TCP server.")
	}

	log.WithFields(logger.Fields{
		"host": host,
		"port": port,
	}).Info("TCP server started")

	runServer(listener)
}

func runServer(listener net.Listener) {
	defer listener.Close()
	go runServerTicks()
	for serverRunning {
		conn, err := listener.Accept()
		if err != nil {
			log.WithError(err).Error("Failed to accept connection")

			continue
		}

		addrInfo := strings.Split(conn.RemoteAddr().String(), ":")
		log.WithFields(logger.Fields{
			"ip":   addrInfo[0],
			"port": addrInfo[1],
		}).Debug("Accepted incoming connection.")
		go handleConnection(conn)
	}
}

func runServerTicks() {
	go runTicker(time.Tick(1*time.Second), "tick:1s")
	go runTicker(time.Tick(5*time.Second), "tick:5s")
	go runTicker(time.Tick(30*time.Second), "tick:30s")
	go runTicker(time.Tick(1*time.Minute), "tick:1m")
}

func runTicker(tick <-chan time.Time, evt string) {
	for range tick {
		if !serverRunning {
			return
		}

		scripting.GlobalEmit(evt, nil)
	}
}

func handleConnection(conn net.Conn) {
	conn.Write([]byte("You were connected successfully, closing connection.\r\n"))
	conn.Close()
}
