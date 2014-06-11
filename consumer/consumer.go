package consumer

import (
	"bytes"
	"fmt"
	"log"
	"net/rpc"
	"sync"
	"time"

	"github.com/zond/mdnsrpc"
	"github.com/zond/moxie/common"
)

type Consumer struct {
	client     *rpc.Client
	stream     chan []byte
	interrupts map[string]*common.ConsumptionInterrupt
	lock       *sync.RWMutex
}

func New() *Consumer {
	return &Consumer{
		stream:     make(chan []byte),
		interrupts: map[string]*common.ConsumptionInterrupt{},
		lock:       &sync.RWMutex{},
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
			if err := client.Call(common.SubscriberLog, s, nil); err != nil {
				log.Printf("%v", err.Error())
			}
		}
	}
	return
}

func (self *Consumer) checkInterrupts(buf *bytes.Buffer) {
	self.lock.Lock()
	defer self.lock.Unlock()
	for name, interrupt := range self.interrupts {
		before, content, after, found, err := interrupt.FindMatch(buf.String())
		if err != nil {
			self.Log(err.Error(), nil)
			delete(self.interrupts, name)
		} else if found {
			client, err := mdnsrpc.Connect(interrupt.Addr)
			if err != nil {
				self.Log(err.Error(), nil)
				delete(self.interrupts, name)
			} else {
				if err := client.Call(common.InterruptorInterrupt, common.InterruptedConsumption{
					Name:    name,
					Content: content,
				}, nil); err != nil {
					self.Log(err.Error(), nil)
					delete(self.interrupts, name)
				} else {
					if interrupt.Times != 0 {
						interrupt.Times -= 1
						if interrupt.Times == 0 {
							delete(self.interrupts, name)
						}
					}
					self.Log(fmt.Sprintf("buffer before: %#v", buf.String()), nil)
					buf.Reset()
					buf.WriteString(before)
					buf.WriteString(after)
					self.Log(fmt.Sprintf("buffer after: %#v", buf.String()), nil)
				}
			}
		}
	}
}

func (self *Consumer) ConsumerInterruptConsumption(interrupt common.ConsumptionInterrupt, unused *struct{}) (err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	if _, err = interrupt.Compiled(); err != nil {
		return
	}
	self.interrupts[interrupt.Name] = &interrupt
	return
}

func (self *Consumer) receive() (err error) {
	buf := &bytes.Buffer{}
	for {
		timedOut := false
		buf.Reset()
		for !timedOut {
			select {
			case b := <-self.stream:
				if _, err = buf.Write(b); err != nil {
					return
				}
			case <-time.After(time.Second / 2):
				timedOut = true
			}
		}
		if buf.Len() > 0 {
			self.checkInterrupts(buf)
		}
		fmt.Print(buf.String())
	}
	return
}

func (self *Consumer) ConsumerConsume(b []byte, unused *struct{}) (err error) {
	self.stream <- b
	return
}
