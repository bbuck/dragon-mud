// Copyright (c) 2016-2017 Brandon Buck

package server

import (
	"net"
	"strings"

	"github.com/bbuck/dragon-mud/logger"
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
	serverRunning = true
	host := viper.GetString("telnet.interface")
	port := viper.GetString("telnet.port")

	initialize()
	done := Emit("server:init", nil)
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

func handleConnection(conn net.Conn) {
	conn.Write([]byte("You were connected successfully, closing connection.\r\n"))
	conn.Close()
}
