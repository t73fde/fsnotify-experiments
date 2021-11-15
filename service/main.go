package main

import (
	"log"
	"time"
)

func main() {
	notifier, err := NewDirNotifier("/tmp/fsnotify")
	if err != nil {
		log.Fatal(err)
	}
	defer notifier.Close()

	done := make(chan bool)
	go func() {
		for event := range notifier.Events() {
			log.Println(event.Op, event.Err, event.Name)
		}
	}()
	go func() {
		for {
			time.Sleep(10 * time.Second)
			log.Println("RELOAD")
			notifier.Reload()
		}
	}()
	<-done
}
