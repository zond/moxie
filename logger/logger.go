package logger

import (
	"log"

	"github.com/zond/mdnsrpc"
	"github.com/zond/moxie/common"
)

type Logger struct {
}

func New() (result *Logger) {
	result = &Logger{}
	return
}

func (self *Logger) Publish(unused struct{}, unused2 *struct{}) (err error) {
	done, err := mdnsrpc.Publish(common.Subscriber, self)
	if err != nil {
		return
	}

	<-done

	return
}

func (self *Logger) SubscriberTransmit(b []byte, unused *struct{}) (err error) {
	log.Printf("TRANSMIT\t%#v", string(b))
	return
}

func (self *Logger) SubscriberReceive(b []byte, unused *struct{}) (err error) {
	log.Printf("RECEIVE\t%#v", string(b))
	return
}

func (self *Logger) SubscriberLog(s string, unused *struct{}) (err error) {
	log.Printf("LOG\t%s\n", s)
	return
}
