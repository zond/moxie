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
	lock                   *sync.RWMutex
	consumptionInterrupts  map[string]func(string)
	transmissionInterrupts map[string]func([]string)
	addr                   *net.TCPAddr
}

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func (self *interruptHandler) InterruptorInterruptedTransmission(interrupt common.InterruptedTransmission, unused *struct{}) (err error) {
	self.lock.RLock()
	defer self.lock.RUnlock()
	f, found := self.transmissionInterrupts[interrupt.Name]
	if !found {
		err = fmt.Errorf("No registered interrupt %#v", interrupt.Name)
		return
	}
	f(interrupt.Match)
	return
}

func (self *interruptHandler) InterruptorInterruptedConsumption(interrupt common.InterruptedConsumption, unused *struct{}) (err error) {
	self.lock.RLock()
	defer self.lock.RUnlock()
	f, found := self.consumptionInterrupts[interrupt.Name]
	if !found {
		err = fmt.Errorf("No registered interrupt %#v", interrupt.Name)
		return
	}
	f(interrupt.Content)
	return
}

func (self *interruptHandler) registerTransmissionInterrupt(name string, f func([]string)) (err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	if len(self.transmissionInterrupts) == 0 {
		if self.addr, _, err = mdnsrpc.Service(self); err != nil {
			return
		}
	}
	self.transmissionInterrupts[name] = f
	return
}

func (self *interruptHandler) registerConsumptionInterrupt(name string, f func(string)) (err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	if len(self.consumptionInterrupts) == 0 {
		if self.addr, _, err = mdnsrpc.Service(self); err != nil {
			return
		}
	}
	self.consumptionInterrupts[name] = f
	return
}

var handler = interruptHandler{
	lock: &sync.RWMutex{},
	consumptionInterrupts:  map[string]func(string){},
	transmissionInterrupts: map[string]func([]string){},
}

func interruptConsumption(interrupt common.ConsumptionInterrupt, h func(string)) (err error) {
	consumers, err := mdnsrpc.LookupAll(common.Consumer)
	if err != nil {
		return
	}
	if err = handler.registerConsumptionInterrupt(interrupt.Name, h); err != nil {
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

func InterruptTransmission(name, pattern string, h func([]string)) (err error) {
	controllers, err := mdnsrpc.LookupAll(common.Controller)
	if err != nil {
		return
	}
	interrupt := common.TransmissionInterrupt{
		Name:    name,
		Pattern: pattern,
	}
	if err = handler.registerTransmissionInterrupt(interrupt.Name, h); err != nil {
		return
	}
	interrupt.Addr = fmt.Sprintf("%v:%v", handler.addr.IP.String(), handler.addr.Port)
	for _, client := range controllers {
		if err = client.Call(common.ControllerInterruptTransmission, interrupt, nil); err != nil {
			return
		}
	}
	return
}

func TransmitMany(lines []string) (err error) {
	for _, line := range lines {
		if err = Transmit(line); err != nil {
			return
		}
	}
	return
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
