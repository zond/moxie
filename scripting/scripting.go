package scripting

import (
	"fmt"
	"log"
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

type ReceiveHookHandles []*ReceiveHookHandle

func (self ReceiveHookHandles) Unregister() {
	for _, handle := range self {
		handle.Unregister()
	}
}

type ReceiveHookHandle struct {
	name         string
	regexp       *regexp.Regexp
	beforeGroup  int
	contentGroup int
	afterGroup   int
	fun          func([]string)
	times        int
}

func (self *ReceiveHookHandle) Unregister() {
	handler.unregisterReceiveHook(self.name)
}

type interruptHandler struct {
	lock                   *sync.RWMutex
	consumptionInterrupts  map[string]func(string)
	transmissionInterrupts map[string]func([]string)
	receiveHooks           map[string]*ReceiveHookHandle
	addr                   *net.TCPAddr
	published              bool
}

func Log(s string) (err error) {
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

func Must(err error) {
	if err != nil {
		panic(err)
	}
}

func MustReceiveHookHandle(h *ReceiveHookHandle, err error) *ReceiveHookHandle {
	if err != nil {
		panic(err)
	}
	return h
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
				hook.fun(match[hook.contentGroup:hook.afterGroup])
			}()
			go self.SubscriberReceive([]byte(match[hook.beforeGroup]+match[hook.afterGroup]), nil)
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

func (self *interruptHandler) unregisterReceiveHook(name string) {
	self.lock.Lock()
	defer self.lock.Unlock()
	delete(self.receiveHooks, name)
}

func (self *interruptHandler) registerReceiveHook(hook *ReceiveHookHandle) (err error) {
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
	receiveHooks:           map[string]*ReceiveHookHandle{},
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

func ReceiveHook(name, pattern string, h func([]string)) (result *ReceiveHookHandle, err error) {
	return ReceiveHookN(0, name, pattern, h)
}

func ReceiveHookOnce(name, pattern string, h func([]string)) (result *ReceiveHookHandle, err error) {
	return ReceiveHookN(1, name, pattern, h)
}

func ReceiveHookN(times int, name, pattern string, h func([]string)) (result *ReceiveHookHandle, err error) {
	reg, err := regexp.Compile("(?ms)(?P<BEFORE>.*?)(?P<CONTENT>" + pattern + ")(?P<AFTER>.*)")
	if err != nil {
		return
	}
	result = &ReceiveHookHandle{
		name:   name,
		regexp: reg,
		fun:    h,
		times:  times,
	}
	for index, name := range reg.SubexpNames() {
		switch name {
		case "BEFORE":
			result.beforeGroup = index
		case "CONTENT":
			result.contentGroup = index
		case "AFTER":
			result.afterGroup = index
		}
	}
	if err = handler.registerReceiveHook(result); err != nil {
		return
	}
	return
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
