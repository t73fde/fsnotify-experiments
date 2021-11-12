package main

import (
	"log"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer watcher.Close()

	done := make(chan bool)
	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write {
					log.Println("modified file:", event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()

	err = watcher.Add(abspath("."))
	if err != nil {
		log.Println("dir")
		log.Fatal(err)
	}
	err = watcher.Add(abspath("fsnotify"))
	if err != nil {
		log.Println("subdir")
		log.Fatal(err)
	}
	<-done
}

func abspath(path string) string {
	result, err := filepath.Abs(path)
	if err != nil {
		log.Fatal(err)
	}
	log.Println("PATH", path, result)
	return result
}
