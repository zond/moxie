package scripting

import (
	"fmt"
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

func (self *interruptHandler) Interrupt(interrupt common.InterruptedConsumption, unused *struct{}) (err error) {
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

func InterruptConsumption(name, pattern string, interrupt func(string)) (err error) {
	consumers, err := mdnsrpc.LookupAll(common.Consumer)
	if err != nil {
		return
	}
	if err = handler.register(name, interrupt); err != nil {
		return
	}
	interruptRequest := common.ConsumptionInterrupt{
		Name:    name,
		Pattern: pattern,
		Addr:    fmt.Sprintf("%v:%v", handler.addr.IP.String(), handler.addr.Port),
	}
	for _, client := range consumers {
		if err = client.Call("InterruptConsumption", interruptRequest, nil); err != nil {
			return
		}
	}
	return
}
