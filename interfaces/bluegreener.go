package interfaces

import (
	"io"

	"github.com/amandamunoz/deployadactyl/config"
	S "github.com/amandamunoz/deployadactyl/structs"
)

// BlueGreener interface.
type BlueGreener interface {
	Push(
		environment config.Environment,
		appPath string,
		deploymentInfo S.DeploymentInfo,
		response io.ReadWriter,
	) error
}
