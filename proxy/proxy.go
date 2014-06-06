package proxy

import (
	"fmt"
	"net"
	"net/rpc"
	"sync"
)

type Proxy struct {
	lock           *sync.RWMutex
	newConsumerSem *sync.Cond
	conn           *net.TCPConn
	buffer         chan []byte
	listener       *net.TCPListener
	server         *rpc.Server
	consumers      map[string]*rpc.Client
	subscribers    map[string]*rpc.Client
}

func New() (result *Proxy) {
	result = &Proxy{
		lock:      &sync.RWMutex{},
		buffer:    make(chan []byte, 2<<16),
		consumers: map[string]*rpc.Client{},
	}
	result.newConsumerSem = sync.NewCond(result.lock)
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

func (self *Proxy) consume() {
	for b := range self.buffer {
		self.lock.Lock()
		for len(self.consumers) == 0 {
			self.newConsumerSem.Wait()
		}
		for addr, client := range self.consumers {
			if err := client.Call("rpc.Consume", b, &struct{}{}); err != nil {
				delete(self.consumers, addr)
			}
		}
		for addr, client := range self.subscribers {
			if err := client.Call("rpc.Receive", b, &struct{}{}); err != nil {
				delete(self.subscribers, addr)
			}
		}
		self.lock.Unlock()
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

func (self *Proxy) Subscribe(addr string, unused *struct{}) (err error) {
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		return
	}

	self.lock.Lock()
	defer self.lock.Unlock()

	self.subscribers[addr] = client

	return
}

func (self *Proxy) Consume(addr string, unused *struct{}) (err error) {
	client, err := rpc.Dial("tcp", addr)
	if err != nil {
		return
	}

	self.lock.Lock()
	defer self.lock.Unlock()

	self.consumers[addr] = client

	self.newConsumerSem.Broadcast()

	return
}

func (self *Proxy) Listen(addr string, unused *struct{}) (err error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return
	}

	if self.listener, err = net.ListenTCP("tcp", tcpAddr); err != nil {
		return
	}

	self.server = rpc.NewServer()
	self.server.RegisterName("rpc", self)
	self.server.Accept(self.listener)

	return
}
