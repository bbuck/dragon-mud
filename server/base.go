package server

import (
	"net"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/bbuck/dragon-mud/logger"
	"github.com/spf13/viper"
)

var serverRunning = false

func Run() {
	serverRunning = true
	host := viper.GetString("net.interface")
	port := viper.GetString("net.port")
	listener, err := net.Listen("tcp", host+":"+port)
	if err != nil {
		logger.WithField("error", err.Error()).Fatal("Failed to start TCP server.")
	}

	logger.WithFields(logrus.Fields{
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
			logger.WithField("error", err.Error()).Error("Failed to accept connection")

			continue
		}

		addrInfo := strings.Split(conn.RemoteAddr().String(), ":")
		logger.WithFields(logrus.Fields{
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
