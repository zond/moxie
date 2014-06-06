package controller

import "github.com/nsf/termbox-go"

type Controller struct {
	cursor int
	buffer []rune
}

func New() *Controller {
	return &Controller{}
}

func (self *Controller) update() (err error) {
	width, height := termbox.Size()
	if err = termbox.Clear(termbox.ColorDefault, termbox.ColorDefault); err != nil {
		return
	}
	for index, ch := range self.buffer {
		x, y := index%width, index/width
		termbox.SetCell(x, y, ch, termbox.ColorDefault, termbox.ColorDefault)
	}
	cursorX, cursorY := self.cursor%width, self.cursor/width
	if cursorY >= height {
		cursorX, cursorY = width, height
	}
	termbox.SetCursor(cursorX, cursorY)
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

func (self *Controller) write(ev termbox.Event) (err error) {
	width, height := termbox.Size()
	switch ev.Key {
	case termbox.KeySpace:
		if self.cursor < width*height-1 {
			self.insert(32)
		}
	case termbox.KeyEnter:
		if err = termbox.Clear(termbox.ColorDefault, termbox.ColorDefault); err != nil {
			return
		}
		self.buffer = nil
		self.cursor = 0
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
	if err = self.update(); err != nil {
		return
	}
	return
}

func (self *Controller) Control(unused struct{}, unused2 *struct{}) (err error) {
	if err = termbox.Init(); err != nil {
		return
	}
	defer termbox.Close()
	if err = self.update(); err != nil {
		return
	}
	for ev := termbox.PollEvent(); ; ev = termbox.PollEvent() {
		switch ev.Type {
		case termbox.EventKey:
			if ev.Key == termbox.KeyCtrlC {
				return
			} else {
				if err = self.write(ev); err != nil {
					return
				}
			}
		case termbox.EventResize:
			if err = self.update(); err != nil {
				return
			}
		}
	}
	return
}
