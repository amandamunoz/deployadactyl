package interfaces

import "github.com/amandamunoz/deployadactyl/config"

// Prechecker interface.
type Prechecker interface {
	AssertAllFoundationsUp(environment config.Environment) error
}
