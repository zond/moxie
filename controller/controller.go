package controller

import "github.com/nsf/termbox-go"

type point struct {
	x int
	y int
}

type Controller struct {
	cursor point
}

func New() *Controller {
	return &Controller{}
}

func (self *Controller) updateCursor() (err error) {
	termbox.SetCursor(self.cursor.x, self.cursor.y)
	if err = termbox.Flush(); err != nil {
		return
	}
	return
}

func (self *Controller) write(ev termbox.Event) (err error) {
	if ev.Key == termbox.KeySpace {
		termbox.SetCell(self.cursor.x, self.cursor.y, 32, termbox.ColorDefault, termbox.ColorDefault)
	} else if ev.Key == termbox.KeyEnter {
		if err = termbox.Clear(termbox.ColorDefault, termbox.ColorDefault); err != nil {
			return
		}
		if err = termbox.Flush(); err != nil {
			return
		}
		self.cursor = point{0, 0}
		if err = self.updateCursor(); err != nil {
			return
		}
		return
	} else {
		termbox.SetCell(self.cursor.x, self.cursor.y, ev.Ch, termbox.ColorDefault, termbox.ColorDefault)
	}
	if err = termbox.Flush(); err != nil {
		return
	}
	if self.cursor.y >= ev.Width {
		self.cursor.y = 0
		self.cursor.x += 1
	} else {
		self.cursor.y += 1
	}
	if err = self.updateCursor(); err != nil {
		return
	}
	return
}

func (self *Controller) Control(unused struct{}, unused2 *struct{}) (err error) {
	if err = termbox.Init(); err != nil {
		return
	}
	defer termbox.Close()
	if err = self.updateCursor(); err != nil {
		return
	}
	for ev := termbox.PollEvent(); ; ev = termbox.PollEvent() {
		if ev.Type == termbox.EventKey {
			if ev.Key == termbox.KeyCtrlC {
				break
			} else {
				if err = self.write(ev); err != nil {
					return
				}
			}
		}
	}
	return
}
