package mocks

import (
	"io"

	"github.com/amandamunoz/deployadactyl/config"
	S "github.com/amandamunoz/deployadactyl/structs"
)

// BlueGreener handmade mock for tests.
type BlueGreener struct {
	PushCall struct {
		Received struct {
			Environment    config.Environment
			AppPath        string
			DeploymentInfo S.DeploymentInfo
			Out            io.Writer
		}
		Returns struct {
			Error error
		}
	}
}

// Push mock method.
func (b *BlueGreener) Push(environment config.Environment, appPath string, deploymentInfo S.DeploymentInfo, out io.ReadWriter) error {
	b.PushCall.Received.Environment = environment
	b.PushCall.Received.AppPath = appPath
	b.PushCall.Received.DeploymentInfo = deploymentInfo
	b.PushCall.Received.Out = out

	return b.PushCall.Returns.Error
}
