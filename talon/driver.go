// Copyright (c) 2016 Brandon Buck

package talon

import bolt "github.com/johnnadratowski/golang-neo4j-bolt-driver"

// Driver is an interface defining the requirement of a method that returns
// a bolt connection or error.
type Driver interface {
	Conn() (bolt.Conn, error)
}

type driverPool struct {
	pool bolt.DriverPool
}

func (dp *driverPool) Conn() (bolt.Conn, error) {
	return dp.pool.OpenPool()
}

type driver struct {
	bolt.Driver
	connectOptions ConnectOptions
}

func (d *driver) Conn() (bolt.Conn, error) {
	return d.OpenNeo(d.connectOptions.URL())
}
