package main

import (
	"os"
	"path/filepath"
)

type simpleDirNotifier struct {
	events chan NotifyEvent
	done   chan struct{}
	reload chan struct{}
	path   string
}

// NewSimpleDirNotifier creates a directory based notifier.
func NewSimpleDirNotifier(path string) (Notifier, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	w := &simpleDirNotifier{
		events: make(chan NotifyEvent),
		done:   make(chan struct{}),
		reload: make(chan struct{}),
		path:   absPath,
	}
	go w.readEvents()
	return w, nil
}

func (w *simpleDirNotifier) Events() <-chan NotifyEvent {
	return w.events
}

func (w *simpleDirNotifier) Reload() {
	w.reload <- struct{}{}
}
func (w *simpleDirNotifier) readEvents() {
	defer close(w.events)
	defer close(w.reload)
	if !w.listElements() {
		return
	}
	for {
		select {
		case <-w.done:
			return
		case <-w.reload:
			w.listElements()
		}
	}
}

func (w *simpleDirNotifier) listElements() bool {
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

func (w *simpleDirNotifier) Close() {
	close(w.done)
	close(w.events)
}
