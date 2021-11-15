package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// Notifier send events about their container and content.
type Notifier interface {
	Events() chan NotifyEvent
	Close()
}

type NotifyEventOp uint8

const (
	_       NotifyEventOp = iota
	Error                 // Error while operating
	Make                  // Make container
	List                  // List container
	Destroy               // Destroy container
	Update                // Update element
	Delete                // Delete element
)

func (c NotifyEventOp) String() string {
	switch c {
	case Error:
		return "ERROR"
	case Make:
		return "MAKE"
	case List:
		return "NOTICE"
	case Destroy:
		return "DESTROY"
	case Update:
		return "UPDATE"
	case Delete:
		return "DELETE"
	default:
		return fmt.Sprintf("UNKNOWN(%d)", c)
	}
}

type NotifyEvent struct {
	Op   NotifyEventOp
	Name string
	Err  error
}

type DirNotifier struct {
	events chan NotifyEvent
	done   chan struct{}
	base   *fsnotify.Watcher
	path   string
	dir    string
}

func (w *DirNotifier) Events() chan NotifyEvent { return w.events }
func NewDirNotifier(path string) (Notifier, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	absDir := filepath.Dir(absPath)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = watcher.Add(absDir)
	if err != nil {
		watcher.Close()
		return nil, err
	}
	watcher.Add(absPath) // Not a problem, if container is not available. It might become available later.
	w := &DirNotifier{
		events: make(chan NotifyEvent),
		done:   make(chan struct{}),
		base:   watcher,
		path:   absPath,
		dir:    absDir,
	}
	go w.readEvents()
	return w, nil
}

func (w *DirNotifier) Close() {
	close(w.done)
}

func (w *DirNotifier) readEvents() {
	defer w.base.Close()
	defer close(w.events)
	if !w.listElements() {
		return
	}
	for {
		if !w.readEvent() {
			return
		}
	}
}

func (w *DirNotifier) readEvent() bool {
	select {
	case <-w.done:
		return false
	default:
	}
	select {
	case <-w.done:
		return false
	case err, ok := <-w.base.Errors:
		if !ok {
			return false
		}
		select {
		case w.events <- NotifyEvent{Op: Error, Err: err}:
		case <-w.done:
			return false
		}
	case ev, ok := <-w.base.Events:
		if !ok {
			return false
		}
		if !w.processEvent(&ev) {
			return false
		}
	}
	return true
}

func (w *DirNotifier) processEvent(ev *fsnotify.Event) bool {
	if strings.HasPrefix(ev.Name, w.path) {
		if len(ev.Name) == len(w.path) {
			return w.processDirEvent(ev)
		}
		return w.processFileEvent(ev)
	}
	return true
}

const deleteOps = fsnotify.Remove | fsnotify.Rename
const updateOps = fsnotify.Create | fsnotify.Write

func (w *DirNotifier) processDirEvent(ev *fsnotify.Event) bool {
	if ev.Op&deleteOps != 0 {
		w.base.Remove(w.path)
		select {
		case w.events <- NotifyEvent{Op: Destroy}:
		case <-w.done:
			return false
		}
		return true
	}
	if ev.Op&fsnotify.Create != 0 {
		err := w.base.Add(w.path)
		if err != nil {
			select {
			case w.events <- NotifyEvent{Op: Error, Err: err}:
			case <-w.done:
				return false
			}
		}
		return w.listElements()
	}
	return true
}

func (w *DirNotifier) processFileEvent(ev *fsnotify.Event) bool {
	if ev.Op&deleteOps != 0 {
		select {
		case w.events <- NotifyEvent{Op: Delete, Name: filepath.Base(ev.Name)}:
		case <-w.done:
			return false
		}
		return true
	}
	if ev.Op&updateOps != 0 {
		if fi, err := os.Lstat(ev.Name); err != nil || !fi.Mode().IsRegular() {
			return true
		}
		select {
		case w.events <- NotifyEvent{Op: Update, Name: filepath.Base(ev.Name)}:
		case <-w.done:
			return false
		}
	}
	return true
}

func (w *DirNotifier) listElements() bool {
	select {
	case w.events <- NotifyEvent{Op: Make}:
	case <-w.done:
		return false
	}
	entries, err := os.ReadDir(w.path)
	if err != nil {
		select {
		case w.events <- NotifyEvent{Op: Error, Err: err}:
		case <-w.done:
			return false
		}
	}
	for _, entry := range entries {
		if info, err1 := entry.Info(); err1 != nil || !info.Mode().IsRegular() {
			continue
		}
		select {
		case w.events <- NotifyEvent{Op: List, Name: entry.Name()}:
		case <-w.done:
			return false
		}
	}

	select {
	case w.events <- NotifyEvent{Op: List}:
	case <-w.done:
		return false
	}
	return true
}

func main() {
	path := "/tmp/fsnotify"
	notifier, err := NewDirNotifier(path)
	if err != nil {
		log.Println("HAE")
		log.Fatal(err)
	}
	defer notifier.Close()

	done := make(chan bool)
	go func() {
		for event := range notifier.Events() {
			log.Println(event.Op, event.Err, event.Name)
		}
	}()
	<-done
}
