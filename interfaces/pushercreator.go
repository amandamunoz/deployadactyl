package interfaces

import (
	"io"

	S "github.com/amandamunoz/deployadactyl/structs"
)

// PusherCreator interface.
type PusherCreator interface {
	CreatePusher(deploymentInfo S.DeploymentInfo, response io.ReadWriter) (Pusher, error)
}
