package proxy

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/zond/mdnsrpc"
	"github.com/zond/moxie/common"
)

type Proxy struct {
	conn   *net.TCPConn
	buffer chan []byte
	lock   *sync.RWMutex
}

func New() (result *Proxy) {
	result = &Proxy{
		buffer: make(chan []byte, 2<<16),
		lock:   &sync.RWMutex{},
	}
	return
}

func (self *Proxy) receiveFromRemote(conn *net.TCPConn) {
	buf := make([]byte, 4096)
	read, err := conn.Read(buf)
	for ; err == nil; read, err = conn.Read(buf) {
		cpy := make([]byte, read)
		copy(cpy, buf)
		self.buffer <- cpy
	}
	self.buffer <- []byte(fmt.Sprintf("Reading from %v: %v\n", conn, err))
}

func (self *Proxy) Log(s string, unused *struct{}) (err error) {
	log.Printf(s)
	loggers, err := mdnsrpc.LookupAll(common.Subscriber)
	if err != nil {
		return
	}
	for _, client := range loggers {
		if err := client.Call(common.SubscriberLog, s, nil); err != nil {
			log.Printf(err.Error())
		}
	}
	return
}

func (self *Proxy) consume() {
	for b := range self.buffer {
		var consumers []*mdnsrpc.Client
		var err error
		for len(consumers) == 0 {
			consumers, err = mdnsrpc.LookupAll(common.Consumer)
			if err != nil {
				if _, ok := err.(mdnsrpc.NoSuchService); !ok {
					self.Log(err.Error(), nil)
				}
			}
			time.Sleep(time.Second / 2)
		}
		for _, client := range consumers {
			if err := client.Call(common.ConsumerConsume, b, nil); err != nil {
				self.Log(err.Error(), nil)
			}
		}
		subscribers, err := mdnsrpc.LookupAll(common.Subscriber)
		if err != nil {
			if _, ok := err.(mdnsrpc.NoSuchService); ok {
				err = nil
			} else {
				self.Log(err.Error(), nil)
			}
		}
		for _, client := range subscribers {
			if err := client.Call(common.SubscriberReceive, b, nil); err != nil {
				self.Log(err.Error(), nil)
			}
		}
	}
}

func (self *Proxy) Connect(addr string, unused *struct{}) (err error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return
	}

	self.lock.Lock()
	defer self.lock.Unlock()

	if self.conn != nil {
		self.conn.Close()
	}

	if self.conn, err = net.DialTCP("tcp", nil, tcpAddr); err != nil {
		return
	}

	go self.consume()

	go self.receiveFromRemote(self.conn)

	return
}

func (self *Proxy) ProxyTransmit(s string, unused *struct{}) (err error) {
	toWrite := []byte(s)
	wrote := 0
	for len(toWrite) > 0 {
		if wrote, err = self.conn.Write(toWrite); err != nil {
			return
		}
		toWrite = toWrite[wrote:]
	}
	go func() {
		subscribers, err := mdnsrpc.LookupAll(common.Subscriber)
		if err != nil {
			if _, ok := err.(mdnsrpc.NoSuchService); ok {
				err = nil
			} else {
				self.Log(err.Error(), nil)
			}
		}
		for _, client := range subscribers {
			if err := client.Call(common.SubscriberTransmit, []byte(s), nil); err != nil {
				self.Log(err.Error(), nil)
			}
		}
	}()
	return
}

func (self *Proxy) Publish(unused struct{}, unused2 *struct{}) (err error) {
	done, err := mdnsrpc.Publish(common.Proxy, self)
	if err != nil {
		return
	}
	<-done
	return
}
