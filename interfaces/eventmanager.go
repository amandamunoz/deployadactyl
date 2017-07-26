package interfaces

import S "github.com/amandamunoz/deployadactyl/structs"

// EventManager interface.
type EventManager interface {
	AddHandler(handler Handler, eventType string) error
	Emit(event S.Event) error
}
