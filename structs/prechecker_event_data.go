package structs

import "github.com/amandamunoz/deployadactyl/config"

// PrecheckerEventData has Environment variables and a description.
type PrecheckerEventData struct {
	Environment config.Environment
	Description string
}
