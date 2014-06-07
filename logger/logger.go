package logger

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net"
	"net/rpc"
)

type Logger struct {
	listener        *net.TCPListener
	server          *rpc.Server
	client          *rpc.Client
	transmitWriter  io.Writer
	transmitReader  io.Reader
	transmitScanner *bufio.Scanner
	receiveWriter   io.Writer
	receiveReader   io.Reader
	receiveScanner  *bufio.Scanner
	done            chan struct{}
}

func New() (result *Logger) {
	result = &Logger{
		done: make(chan struct{}),
	}
	result.receiveReader, result.receiveWriter = io.Pipe()
	result.receiveScanner = bufio.NewScanner(result.receiveReader)
	result.transmitReader, result.transmitWriter = io.Pipe()
	result.transmitScanner = bufio.NewScanner(result.transmitReader)
	return
}

func (self *Logger) Listen(hostname string, unused *struct{}) (err error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%v:0", hostname))
	if err != nil {
		return
	}

	if self.listener, err = net.ListenTCP("tcp", tcpAddr); err != nil {
		return
	}

	self.server = rpc.NewServer()
	self.server.RegisterName("rpc", self)
	go func() {
		self.server.Accept(self.listener)
		close(self.done)
	}()

	return
}

func (self *Logger) Transmit(b []byte, unused *struct{}) (err error) {
	_, err = self.transmitWriter.Write(b)
	return
}

func (self *Logger) Receive(b []byte, unused *struct{}) (err error) {
	_, err = self.receiveWriter.Write(b)
	return
}

func (self *Logger) Connect(addr string, unused *struct{}) (err error) {
	if self.client, err = rpc.Dial("tcp", addr); err != nil {
		return
	}

	if err = self.client.Call("rpc.Subscribe", self.listener.Addr().String(), &struct{}{}); err != nil {
		return
	}

	go func() {
		for self.receiveScanner.Scan() {
			log.Printf("RECEIVE\t%#v\n", self.receiveScanner.Text())
		}
	}()

	for self.transmitScanner.Scan() {
		log.Printf("TRANSMIT\t%#v\n", self.transmitScanner.Text())
	}

	return
}
