package logger

import (
	"bufio"
	"io"
	"log"

	"github.com/zond/mdnsrpc"
	"github.com/zond/moxie/common"
)

type Logger struct {
	transmitWriter  io.Writer
	transmitReader  io.Reader
	transmitScanner *bufio.Scanner
	receiveWriter   io.Writer
	receiveReader   io.Reader
	receiveScanner  *bufio.Scanner
}

func New() (result *Logger) {
	result = &Logger{}
	result.receiveReader, result.receiveWriter = io.Pipe()
	result.receiveScanner = bufio.NewScanner(result.receiveReader)
	result.transmitReader, result.transmitWriter = io.Pipe()
	result.transmitScanner = bufio.NewScanner(result.transmitReader)
	return
}

func (self *Logger) Publish(unused struct{}, unused2 *struct{}) (err error) {
	_, err = mdnsrpc.Publish(common.Subscriber, self)
	if err != nil {
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

func (self *Logger) SubscriberTransmit(b []byte, unused *struct{}) (err error) {
	_, err = self.transmitWriter.Write(b)
	return
}

func (self *Logger) SubscriberReceive(b []byte, unused *struct{}) (err error) {
	_, err = self.receiveWriter.Write(b)
	return
}

func (self *Logger) SubscriberLog(s string, unused *struct{}) (err error) {
	log.Printf("LOG\t%s\n", s)
	return
}
