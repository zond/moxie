package scripting

import (
	"fmt"
	"math/rand"
	"net"
	"sync"

	"github.com/zond/mdnsrpc"
	"github.com/zond/moxie/common"
)

func Wait() {
	done := make(chan struct{})
	<-done
}

type interruptHandler struct {
	lock       *sync.RWMutex
	interrupts map[string]func(string)
	addr       *net.TCPAddr
}

func (self *interruptHandler) InterruptorInterrupt(interrupt common.InterruptedConsumption, unused *struct{}) (err error) {
	self.lock.RLock()
	defer self.lock.RUnlock()
	f, found := self.interrupts[interrupt.Name]
	if !found {
		err = fmt.Errorf("No registered interrupt %#v", interrupt.Name)
		return
	}
	f(interrupt.Content)
	return
}

func (self *interruptHandler) register(name string, f func(string)) (err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	if len(self.interrupts) == 0 {
		if self.addr, _, err = mdnsrpc.Service(self); err != nil {
			return
		}
	}
	self.interrupts[name] = f
	return
}

var handler = interruptHandler{
	lock:       &sync.RWMutex{},
	interrupts: map[string]func(string){},
}

func interruptConsumption(interrupt common.ConsumptionInterrupt, h func(string)) (err error) {
	consumers, err := mdnsrpc.LookupAll(common.Consumer)
	if err != nil {
		return
	}
	if err = handler.register(interrupt.Name, h); err != nil {
		return
	}
	interrupt.Addr = fmt.Sprintf("%v:%v", handler.addr.IP.String(), handler.addr.Port)
	for _, client := range consumers {
		if err = client.Call(common.ConsumerInterruptConsumption, interrupt, nil); err != nil {
			return
		}
	}
	return
}

func InterruptConsumption(name, pattern string, handler func(string)) (err error) {
	return interruptConsumption(common.ConsumptionInterrupt{
		Name:    name,
		Pattern: pattern,
	}, handler)
}

func TransmitAndInterruptN(n int, trans string, pattern string, h func(string)) (err error) {
	if err = interruptConsumption(common.ConsumptionInterrupt{
		Name:    fmt.Sprint(rand.Int63()),
		Pattern: pattern,
		Times:   n,
	}, h); err != nil {
		return
	}
	return Transmit(trans)
}

func TransmitAndInterruptOnce(trans string, pattern string, h func(string)) (err error) {
	return TransmitAndInterruptN(1, trans, pattern, h)
}

func Transmit(s string) (err error) {
	client, err := mdnsrpc.LookupOne(common.Proxy)
	if err != nil {
		return
	}
	if err = client.Call(common.ProxyTransmit, s+"\n", nil); err != nil {
		return
	}
	return
}
