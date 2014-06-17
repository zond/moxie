package controller

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/boltdb/bolt"
	"github.com/nsf/termbox-go"
	"github.com/zond/mdnsrpc"
	"github.com/zond/moxie/common"
)

var history = []byte("history")
var splitReg = regexp.MustCompile("\\W+")

const (
	regular = iota
	historySearch
)

const (
	historySearchHeader = "(reverse-i-search)`"
)

type Controller struct {
	cursor        int
	buffer        []rune
	dir           string
	db            *bolt.DB
	lastHistory   []byte
	mode          int
	historySearch []rune
	completeTree  *common.CompleteNode
	interrupts    map[string]*common.TransmissionInterrupt
	lock          *sync.RWMutex
}

func New() *Controller {
	return &Controller{
		interrupts: map[string]*common.TransmissionInterrupt{},
		lock:       &sync.RWMutex{},
	}
}

func (self *Controller) Dir(d string) *Controller {
	self.dir = d
	return self
}

func (self *Controller) Publish(unused struct{}, unused2 *struct{}) (err error) {
	_, err = mdnsrpc.Publish(common.Subscriber, self)
	if err != nil {
		return
	}
	_, err = mdnsrpc.Publish(common.Controller, self)
	if err != nil {
		return
	}
	return
}

func (self *Controller) ControllerInterruptTransmission(interrupt common.TransmissionInterrupt, unused *struct{}) (err error) {
	self.lock.Lock()
	defer self.lock.Unlock()
	if _, err = interrupt.Compiled(); err != nil {
		return
	}
	self.interrupts[interrupt.Name] = &interrupt
	return
}

func (self *Controller) SubscriberTransmit(b []byte, unused *struct{}) (err error) {
	return
}

func (self *Controller) SubscriberReceive(b []byte, unused *struct{}) (err error) {
	for _, part := range splitReg.Split(string(b), -1) {
		self.rememberCompletion(part)
	}
	return
}

func (self *Controller) SubscriberLog(s string, unused *struct{}) (err error) {
	return
}

func (self *Controller) rememberCompletion(s string) {
	if len(s) > 3 {
		self.completeTree = self.completeTree.Insert([]byte(s))
	}
}

func (self *Controller) Log(s string, unused *struct{}) (err error) {
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

func (self *Controller) setRunes(r []rune) (err error) {
	width, _ := termbox.Size()
	if err = termbox.Clear(termbox.ColorDefault, termbox.ColorDefault); err != nil {
		return
	}
	for index, ch := range r {
		x, y := index%width, index/width
		termbox.SetCell(x, y, ch, termbox.ColorDefault, termbox.ColorDefault)
	}
	return
}

func (self *Controller) setCursor(i int) {
	width, height := termbox.Size()
	cursorX, cursorY := i%width, i/width
	if cursorY >= height {
		cursorX, cursorY = width, height
	}
	termbox.SetCursor(cursorX, cursorY)
}

func (self *Controller) update() (err error) {
	switch self.mode {
	case regular:
		if err = self.setRunes(self.buffer); err != nil {
			return
		}
		self.setCursor(self.cursor)
	case historySearch:
		if err = self.setRunes([]rune(fmt.Sprintf("%s%s`: %s", historySearchHeader, string(self.buffer), string(self.historySearch)))); err != nil {
			return
		}
		self.setCursor(len(historySearchHeader) + self.cursor)
	}
	if err = termbox.Flush(); err != nil {
		return
	}
	return
}

func (self *Controller) insert(ch rune) {
	if self.cursor == len(self.buffer) {
		self.buffer = append(self.buffer, ch)
	} else if self.cursor == 0 {
		self.buffer = append([]rune{ch}, self.buffer...)
	} else {
		self.buffer = append(self.buffer[:self.cursor], append([]rune{ch}, self.buffer[self.cursor:]...)...)
	}
	self.cursor += 1
}

func (self *Controller) backspace() {
	if self.cursor == len(self.buffer) {
		self.buffer = self.buffer[:len(self.buffer)-1]
		self.cursor -= 1
	} else if self.cursor > 0 {
		self.buffer = append(self.buffer[:self.cursor-1], self.buffer[self.cursor:]...)
		self.cursor -= 1
	}
}

func (self *Controller) timeToBytes(t time.Time) (result []byte) {
	result = make([]byte, 8)
	ns := t.UnixNano()
	result[7] = byte(ns)
	result[6] = byte(ns >> 8)
	result[5] = byte(ns >> 16)
	result[4] = byte(ns >> 24)
	result[3] = byte(ns >> 32)
	result[2] = byte(ns >> 40)
	result[1] = byte(ns >> 48)
	result[0] = byte(ns >> 56)
	return
}

func (self *Controller) pushHistory(b []rune) (err error) {
	return self.db.Update(func(tx *bolt.Tx) (err error) {
		bucket, err := tx.CreateBucketIfNotExists(history)
		if err != nil {
			return
		}
		if err = bucket.Put(self.timeToBytes(time.Now()), []byte(string(b))); err != nil {
			return
		}
		return
	})
}

type CtrlC string

func (self CtrlC) Error() string {
	return string(self)
}

func (self *Controller) nextHistory(lastHistory []byte) (newHistory, result []byte, found bool, err error) {
	if lastHistory == nil {
		return
	}
	err = self.db.Update(func(tx *bolt.Tx) (err error) {
		bucket, err := tx.CreateBucketIfNotExists(history)
		if err != nil {
			return
		}
		cursor := bucket.Cursor()
		if checkOld, _ := cursor.Seek(lastHistory); checkOld == nil {
			found = false
		} else {
			newHistory, result = cursor.Next()
			found = true
		}
		return
	})
	return
}

func (self *Controller) prevHistory(lastHistory []byte) (newHistory, result []byte, found bool, err error) {
	err = self.db.Update(func(tx *bolt.Tx) (err error) {
		bucket, err := tx.CreateBucketIfNotExists(history)
		if err != nil {
			return
		}
		cursor := bucket.Cursor()
		if lastHistory == nil {
			newHistory, result = cursor.Last()
			found = newHistory != nil
		} else {
			if checkOld, _ := cursor.Seek(lastHistory); checkOld == nil {
				found = false
			} else {
				if newHistory, result = cursor.Prev(); newHistory == nil {
					newHistory = lastHistory
				} else {
					found = true
				}
			}
		}
		return
	})
	return
}

func (self *Controller) searchPrevHistory(lastHistory []byte, needle []rune) (newHistory, result []byte, found bool, err error) {
	err = self.db.Update(func(tx *bolt.Tx) (err error) {
		bucket, err := tx.CreateBucketIfNotExists(history)
		if err != nil {
			return
		}
		cursor := bucket.Cursor()
		tries := 0
		if lastHistory == nil {
			newHistory, result = cursor.Last()
		} else {
			if checkOld, _ := cursor.Seek(lastHistory); checkOld == nil {
				newHistory, result = cursor.Last()
			} else {
				newHistory, result = cursor.Prev()
				tries = 1
			}
		}
		for (tries > 0 || newHistory != nil) && !strings.Contains(string(result), string(needle)) {
			newHistory, result = cursor.Prev()
			if newHistory == nil && tries > 0 {
				tries -= 1
				newHistory, result = cursor.Last()
			}
		}
		found = newHistory != nil
		return
	})
	return
}

func (self *Controller) updateHistorySearch() (err error) {
	if len(self.buffer) > 0 {
		var hist []byte
		var found bool
		self.lastHistory, hist, found, err = self.searchPrevHistory(self.lastHistory, self.buffer)
		if err != nil {
			return
		}
		if found {
			self.historySearch = []rune(string(hist))
		}
	}
	return
}

func (self *Controller) sendToProxy(buffer []rune) (err error) {
	var client *mdnsrpc.Client
	if client, err = mdnsrpc.LookupOne(common.Proxy); err != nil {
		return
	}
	if err = client.Call(common.ProxyTransmit, string(self.buffer)+"\n", nil); err != nil {
		return
	}
	return
}

func (self *Controller) handle(ev termbox.Event) (err error) {
	switch ev.Type {
	case termbox.EventKey:
		if ev.Key == termbox.KeyCtrlC {
			err = CtrlC("QUIT")
			return
		} else {
			before := make([]rune, len(self.buffer))
			copy(before, self.buffer)
			width, height := termbox.Size()
			switch ev.Key {
			case termbox.KeySpace:
				if self.cursor < width*height-1 {
					self.insert(32)
				}
			case termbox.KeyEnter:
				switch self.mode {
				case regular:
					if len(self.buffer) > 0 {
						str := string(self.buffer)
						interrupted := false
						self.lock.Lock()
						func() {
							defer self.lock.Unlock()
							for name, interrupt := range self.interrupts {
								var compiled *regexp.Regexp
								compiled, err = interrupt.Compiled()
								if err != nil {
									self.Log(err.Error(), nil)
									delete(self.interrupts, name)
								} else {
									if match := compiled.FindStringSubmatch(str); match != nil {
										client, err := mdnsrpc.Connect(interrupt.Addr)
										if err != nil {
											self.Log(err.Error(), nil)
											delete(self.interrupts, name)
										} else {
											if err := client.Call(common.InterruptorInterruptedTransmission, common.InterruptedTransmission{
												Name:  name,
												Match: match,
											}, nil); err != nil {
												self.Log(err.Error(), nil)
												delete(self.interrupts, name)
											} else {
												interrupted = true
											}
										}
									}
								}
							}
						}()
						if !interrupted {
							for err := self.sendToProxy(self.buffer); err != nil; err = self.sendToProxy(self.buffer) {
								time.Sleep(time.Second / 2)
							}
						}
						if err = termbox.Clear(termbox.ColorDefault, termbox.ColorDefault); err != nil {
							return
						}
						if err = self.pushHistory(self.buffer); err != nil {
							return
						}
						for _, part := range splitReg.Split(string(self.buffer), -1) {
							self.rememberCompletion(part)
						}
						self.buffer = nil
						self.cursor = 0
						self.lastHistory = nil
					}
				case historySearch:
					self.buffer = self.historySearch
					self.historySearch = nil
					self.mode = regular
				}
			case termbox.KeyTab:
				completed, found := self.completeTree.Complete([]byte(string(self.buffer)))
				if found {
					self.buffer = []rune(string(completed))
					self.cursor = len(self.buffer)
				}
			case termbox.KeyArrowDown:
				if self.mode == regular {
					var hist []byte
					var found bool
					self.lastHistory, hist, found, err = self.nextHistory(self.lastHistory)
					if err != nil {
						return
					}
					if found {
						self.buffer = []rune(string(hist))
						self.cursor = len(self.buffer)
					}
				}
			case termbox.KeyCtrlR:
				self.mode = historySearch
				if err = self.updateHistorySearch(); err != nil {
					return
				}
			case termbox.KeyArrowUp:
				if self.mode == regular {
					var hist []byte
					var found bool
					self.lastHistory, hist, found, err = self.prevHistory(self.lastHistory)
					if err != nil {
						return
					}
					if found {
						self.buffer = []rune(string(hist))
						self.cursor = len(self.buffer)
					}
				}
			case termbox.KeyBackspace2:
				if self.cursor > 0 {
					self.backspace()
				}
			case termbox.KeyArrowLeft:
				if self.cursor > 0 {
					self.cursor -= 1
				}
			case termbox.KeyArrowRight:
				if self.cursor < len(self.buffer) {
					self.cursor += 1
				}
			default:
				if self.cursor < width*height-1 {
					self.insert(ev.Ch)
				}
			}
			if self.mode == historySearch && bytes.Compare([]byte(string(self.buffer)), []byte(string(before))) != 0 {
				self.lastHistory = nil
				if err = self.updateHistorySearch(); err != nil {
					return
				}
			}
			if err = self.update(); err != nil {
				return
			}
			return
		}
	case termbox.EventResize:
		if err = self.update(); err != nil {
			return
		}
	}
	return
}

func (self *Controller) Control(unused struct{}, unused2 *struct{}) (err error) {
	if err = os.MkdirAll(self.dir, 0700); err != nil && !os.IsExist(err) {
		return
	}
	if self.db, err = bolt.Open(filepath.Join(self.dir, "controller.db"), 0700); err != nil {
		return
	}
	if err = termbox.Init(); err != nil {
		return
	}
	defer func() {
		if e := recover(); e == nil && err == nil {
			termbox.Close()
		}
	}()
	if err = self.update(); err != nil {
		return
	}
	for ev := termbox.PollEvent(); ; ev = termbox.PollEvent() {
		if err = self.handle(ev); err != nil {
			if _, ok := err.(CtrlC); ok {
				err = nil
			}
			return
		}
	}
	return
}
