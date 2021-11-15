package main

import (
	"fmt"
)

// Notifier send events about their container and content.
type Notifier interface {
	// Return the channel
	Events() <-chan NotifyEvent

	// Signal a reload of the container. This will result in some events.
	Reload()

	// Close the notifier (and eventually the channel)
	Close()
}

// NotifyEventOp describe a notification operation.
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

// String representation of operation code.
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

// NotifyEvent represents a single container / element event.
type NotifyEvent struct {
	Op   NotifyEventOp
	Name string
	Err  error // Valid iff Op == Error
}
