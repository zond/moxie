package common

import (
	"net/rpc"
	"regexp"
)

const (
	Proxy      = "moxie_Proxy"
	Consumer   = "moxie_Consumer"
	Subscriber = "moxie_Subscriber"
)

type ConsumptionInterrupt struct {
	Name         string
	Addr         string
	Pattern      string
	compiled     *regexp.Regexp
	client       *rpc.Client
	beforeGroup  int
	contentGroup int
	afterGroup   int
}

func (self *ConsumptionInterrupt) Compiled() (result *regexp.Regexp, err error) {
	if self.compiled == nil {
		self.compiled, err = regexp.Compile("(?ms)(?P<BEFORE>.*?)(?P<CONTENT>" + self.Pattern + ")(?P<AFTER>.*)")
		for index, name := range self.compiled.SubexpNames() {
			switch name {
			case "BEFORE":
				self.beforeGroup = index
			case "CONTENT":
				self.contentGroup = index
			case "AFTER":
				self.afterGroup = index
			}
		}
	}
	result = self.compiled
	return
}

func (self *ConsumptionInterrupt) FindMatch(s string) (before, content, after string, found bool, err error) {
	compiled, err := self.Compiled()
	if err != nil {
		return
	}
	if match := compiled.FindStringSubmatch(s); match != nil {
		before, content, after, found = match[self.beforeGroup], match[self.contentGroup], match[self.afterGroup], true
	}
	return
}

func (self *ConsumptionInterrupt) Client() (result *rpc.Client, err error) {
	if self.client == nil {
		self.client, err = rpc.Dial("tcp", self.Addr)
	}
	result = self.client
	return
}
