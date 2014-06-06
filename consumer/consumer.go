package consumer

import (
	"fmt"
	"net"
	"net/rpc"
)

type Consumer struct {
	listener *net.TCPListener
	server   *rpc.Server
	client   *rpc.Client
	done     chan struct{}
}

func New() *Consumer {
	return &Consumer{
		done: make(chan struct{}),
	}
}

func (self *Consumer) Listen(hostname string, unused *struct{}) (err error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%v:0", hostname))
	if err != nil {
		return
	}

	if self.listener, err = net.ListenTCP("tcp", tcpAddr); err != nil {
		return
	}

	self.server = rpc.NewServer()
	self.server.Register(self)
	go func() {
		self.server.Accept(self.listener)
		close(self.done)
	}()

	return
}

func (self *Consumer) Consume(b []byte, unused *struct{}) (err error) {
	fmt.Printf("%s", b)
	return
}

func (self *Consumer) Connect(addr string, unused *struct{}) (err error) {
	if self.client, err = rpc.Dial("tcp", addr); err != nil {
		return
	}

	if err = self.client.Call("Proxy.Consume", self.listener.Addr().String(), &struct{}{}); err != nil {
		return
	}

	<-self.done

	return
}
