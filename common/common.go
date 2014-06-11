package common

import (
	"bytes"
	"fmt"
	"regexp"
)

const (
	Proxy                        = "moxie_Proxy"
	Consumer                     = "moxie_Consumer"
	Subscriber                   = "moxie_Subscriber"
	ProxyTransmit                = "ProxyTransmit"
	SubscriberTransmit           = "SubscriberTransmit"
	SubscriberReceive            = "SubscriberReceive"
	SubscriberLog                = "SubscriberLog"
	ConsumerConsume              = "ConsumerConsume"
	ConsumerInterruptConsumption = "ConsumerInterruptConsumption"
	InterruptorInterrupt         = "InterruptorInterrupt"
)

type InterruptedConsumption struct {
	Name    string
	Content string
}

type ConsumptionInterrupt struct {
	Name         string
	Addr         string
	Pattern      string
	Times        int
	compiled     *regexp.Regexp
	beforeGroup  int
	contentGroup int
	afterGroup   int
}

func (self *ConsumptionInterrupt) Compiled() (result *regexp.Regexp, err error) {
	if self.compiled == nil {
		if self.compiled, err = regexp.Compile("(?ms)(?P<BEFORE>.*?)(?P<CONTENT>" + self.Pattern + ")(?P<AFTER>.*)"); err != nil {
			return
		}
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

type CompleteNode struct {
	terminates bool
	char       byte
	children   []*CompleteNode
}

func (self *CompleteNode) stringify(indent string, buf *bytes.Buffer) {
	anyChild := false
	firstChild := true
	for _, child := range self.children {
		if child != nil {
			anyChild = true
			if firstChild {
				firstChild = false
				fmt.Fprintf(buf, "%v", string([]byte{child.char}))
			} else {
				fmt.Fprintf(buf, "%v%v", indent, string([]byte{child.char}))
			}
			if child.terminates {
				fmt.Fprintf(buf, ".")
			}
			child.stringify(indent+" ", buf)
		}
	}
	if !anyChild {
		fmt.Fprintf(buf, "\n")
	}
}

func (self *CompleteNode) String() string {
	buf := bytes.NewBuffer(nil)
	self.stringify("", buf)
	return buf.String()
}

func (self *CompleteNode) Insert(b []byte) (result *CompleteNode) {
	if self == nil {
		self = &CompleteNode{
			children: make([]*CompleteNode, 2<<8),
		}
	}
	result = self
	if len(b) == 0 {
		self.terminates = true
		result = self
		return
	}
	if self.children[int(b[0])] == nil {
		newNode := &CompleteNode{
			children: make([]*CompleteNode, 2<<8),
			char:     b[0],
		}
		self.children[int(b[0])] = newNode.Insert(b[1:])
	} else {
		self.children[int(b[0])] = self.children[int(b[0])].Insert(b[1:])
	}
	return
}

func (self *CompleteNode) completeHelper(toTraverse, traversed []byte) (result []byte, found bool) {
	if len(toTraverse) == 0 {
		if self.terminates {
			result = traversed
			found = true
			return
		}
		nonNilChildren := []*CompleteNode{}
		for _, child := range self.children {
			if child != nil {
				nonNilChildren = append(nonNilChildren, child)
			}
		}
		if len(nonNilChildren) == 1 {
			return nonNilChildren[0].completeHelper(toTraverse, append(traversed, nonNilChildren[0].char))
		}
		if len(nonNilChildren) == 0 {
			result = traversed
			found = true
		}
		return
	}
	if self.children[int(toTraverse[0])] == nil {
		return
	}
	return self.children[int(toTraverse[0])].completeHelper(toTraverse[1:], append(traversed, toTraverse[0]))
}

func (self *CompleteNode) Complete(b []byte) (result []byte, found bool) {
	return self.completeHelper(b, nil)
}
