package consumer

import (
	"fmt"
	"log"
	"net/rpc"

	"github.com/zond/mdnsrpc"
	"github.com/zond/moxie/common"
)

type Consumer struct {
	client *rpc.Client
	stream chan []byte
}

func New() *Consumer {
	return &Consumer{
		stream: make(chan []byte),
	}
}

func (self *Consumer) Publish(unused struct{}, unused2 *struct{}) (err error) {
	_, err = mdnsrpc.Publish(common.Consumer, self)
	if err != nil {
		return
	}
	if err = self.receive(); err != nil {
		return
	}
	return
}

func (self *Consumer) Log(s string, unused *struct{}) (err error) {
	loggers, err := mdnsrpc.LookupAll(common.Subscriber)
	if err != nil {
		return
	}
	if len(loggers) == 0 {
		log.Printf("%v", err.Error())
	} else {
		for _, client := range loggers {
			if err := client.Call("rpc.Log", s, nil); err != nil {
				log.Printf("%v", err.Error())
			}
		}
	}
	return
}

func (self *Consumer) receive() (err error) {
	for b := range self.stream {
		fmt.Print(string(b))
	}
	return
}

func (self *Consumer) Consume(b []byte, unused *struct{}) (err error) {
	self.stream <- b
	return
}
