package interfaces

import S "github.com/amandamunoz/deployadactyl/structs"

// Handler interface.
type Handler interface {
	OnEvent(event S.Event) error
}
