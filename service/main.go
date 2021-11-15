package main

import (
	"log"
)

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
	// go func() {
	// 	for {
	// 		time.Sleep(10 * time.Second)
	// 		log.Println("RELOAD")
	// 		notifier.Reload()
	// 	}
	// }()
	<-done
}
