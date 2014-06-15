package scripting

import (
	"fmt"
	"math/rand"
	"net"
	"regexp"
	"sync"

	"github.com/zond/mdnsrpc"
	"github.com/zond/moxie/common"
)

func Wait() {
	done := make(chan struct{})
	<-done
}

type receiveHook struct {
	name   string
	regexp *regexp.Regexp
	fun    func([]string)
	times  int
}

type interruptHandler struct {
	lock                   *sync.RWMutex
	consumptionInterrupts  map[string]func(string)
	transmissionInterrupts map[string]func([]string)
	receiveHooks           map[string]*receiveHook
	addr                   *net.TCPAddr
	published              bool
}

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func (self *interruptHandler) SubscriberTransmit(b []byte, unused *struct{}) (err error) {
	return
}

func (self *interruptHandler) SubscriberReceive(b []byte, unused *struct{}) (err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	for name, hook := range self.receiveHooks {
		if match := hook.regexp.FindStringSubmatch(string(b)); match != nil {
			self.lock.Unlock()
			func() {
				defer self.lock.Lock()
				hook.fun(match)
			}()
			if hook.times != 0 {
				hook.times -= 1
				if hook.times == 0 {
					delete(self.receiveHooks, name)
				}
			}
		}
	}
	return
}

func (self *interruptHandler) SubscriberLog(s string, unused *struct{}) (err error) {
	return
}
func (self *interruptHandler) InterruptorInterruptedTransmission(interrupt common.InterruptedTransmission, unused *struct{}) (err error) {
	self.lock.RLock()
	var f func([]string)
	if err = func() (err error) {
		defer self.lock.RUnlock()
		found := false
		f, found = self.transmissionInterrupts[interrupt.Name]
		if !found {
			err = fmt.Errorf("No registered interrupt %#v", interrupt.Name)
			return
		}
		return
	}(); err != nil {
		return
	}
	f(interrupt.Match)
	return
}

func (self *interruptHandler) InterruptorInterruptedConsumption(interrupt common.InterruptedConsumption, unused *struct{}) (err error) {
	self.lock.RLock()
	var f func(string)
	if err = func() (err error) {
		defer self.lock.RUnlock()
		found := false
		f, found = self.consumptionInterrupts[interrupt.Name]
		if !found {
			err = fmt.Errorf("No registered interrupt %#v", interrupt.Name)
			return
		}
		return
	}(); err != nil {
		return
	}
	f(interrupt.Content)
	return
}

func (self *interruptHandler) registerReceiveHook(hook *receiveHook) (err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	if err = self.publish(); err != nil {
		return
	}
	self.receiveHooks[hook.name] = hook
	return
}

func (self *interruptHandler) registerTransmissionInterrupt(name string, f func([]string)) (err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	if err = self.publish(); err != nil {
		return
	}
	self.transmissionInterrupts[name] = f
	return
}

func (self *interruptHandler) publish() (err error) {
	if !self.published {
		if self.addr, _, err = mdnsrpc.Service(self); err != nil {
			return
		}
		if _, err = mdnsrpc.Publish(common.Subscriber, self); err != nil {
			return
		}
		self.published = true
	}
	return
}

func (self *interruptHandler) registerConsumptionInterrupt(name string, f func(string)) (err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	if err = self.publish(); err != nil {
		return
	}
	self.consumptionInterrupts[name] = f
	return
}

var handler = interruptHandler{
	lock: &sync.RWMutex{},
	consumptionInterrupts:  map[string]func(string){},
	transmissionInterrupts: map[string]func([]string){},
	receiveHooks:           map[string]*receiveHook{},
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

func InterruptConsumptionN(n int, name, pattern string, handler func(string)) (err error) {
	return interruptConsumption(common.ConsumptionInterrupt{
		Name:    name,
		Pattern: pattern,
		Times:   n,
	}, handler)
}

func InterruptConsumptionOnce(name, pattern string, handler func(string)) (err error) {
	return interruptConsumption(common.ConsumptionInterrupt{
		Name:    name,
		Pattern: pattern,
		Times:   1,
	}, handler)
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

func ReceiveHookOnce(name, pattern string, h func([]string)) (err error) {
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return
	}
	hook := &receiveHook{
		name:   name,
		regexp: reg,
		fun:    h,
		times:  1,
	}
	return handler.registerReceiveHook(hook)
}

func ReceiveHookN(times int, name, pattern string, h func([]string)) (err error) {
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return
	}
	hook := &receiveHook{
		name:   name,
		regexp: reg,
		fun:    h,
		times:  times,
	}
	return handler.registerReceiveHook(hook)
}

func TransmitMany(lines []string) (err error) {
	client, err := mdnsrpc.LookupOne(common.Proxy)
	if err != nil {
		return
	}
	for _, line := range lines {
		if err = client.Call(common.ProxyTransmit, line+"\n", nil); err != nil {
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
	return TransmitMany([]string{s})
}
