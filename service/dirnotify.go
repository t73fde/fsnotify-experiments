package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

type dirNotifier struct {
	events chan NotifyEvent
	done   chan struct{}
	base   *fsnotify.Watcher
	path   string
	dir    string
}

// NewDirNotifier creates a directory based notifier.
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
	w := &dirNotifier{
		events: make(chan NotifyEvent),
		done:   make(chan struct{}),
		base:   watcher,
		path:   absPath,
		dir:    absDir,
	}
	go w.readEvents()
	return w, nil
}

func (w *dirNotifier) Events() <-chan NotifyEvent {
	return w.events
}

func (w *dirNotifier) Reload() {
	go w.listElements()
}

func (w *dirNotifier) Close() {
	close(w.done)
}

func (w *dirNotifier) readEvents() {
	defer w.base.Close()
	defer close(w.events)
	if !w.listElements() {
		return
	}
	for w.readEvent() {
	}
}

func (w *dirNotifier) readEvent() bool {
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

func (w *dirNotifier) processEvent(ev *fsnotify.Event) bool {
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

func (w *dirNotifier) processDirEvent(ev *fsnotify.Event) bool {
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

func (w *dirNotifier) processFileEvent(ev *fsnotify.Event) bool {
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

func (w *dirNotifier) listElements() bool {
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
