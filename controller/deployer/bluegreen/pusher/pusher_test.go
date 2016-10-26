package pusher_test

import (
	"errors"
	"fmt"
	"math/rand"

	. "github.com/compozed/deployadactyl/controller/deployer/bluegreen/pusher"
	"github.com/compozed/deployadactyl/logger"
	"github.com/compozed/deployadactyl/mocks"
	"github.com/compozed/deployadactyl/randomizer"
	S "github.com/compozed/deployadactyl/structs"
	"github.com/op/go-logging"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
)

var _ = Describe("Pusher", func() {
	var (
		courier *mocks.Courier
		pusher  Pusher

		foundationURL    string
		username         string
		password         string
		org              string
		space            string
		skipSSL          bool
		domain           string
		appPath          string
		appName          string
		appNameVenerable string
		instances        uint16
		appExists        bool
		deploymentInfo   S.DeploymentInfo
		response         *gbytes.Buffer
		logBuffer        *gbytes.Buffer
	)

	BeforeEach(func() {
		courier = &mocks.Courier{}

		foundationURL = "foundationURL-" + randomizer.StringRunes(10)
		username = "username-" + randomizer.StringRunes(10)
		password = "password-" + randomizer.StringRunes(10)
		org = "org-" + randomizer.StringRunes(10)
		space = "space-" + randomizer.StringRunes(10)
		domain = "domain-" + randomizer.StringRunes(10)
		appPath = "appPath-" + randomizer.StringRunes(10)
		appName = "appName-" + randomizer.StringRunes(10)
		appNameVenerable = appName + "-venerable"
		instances = uint16(rand.Uint32())
		appExists = false

		response = gbytes.NewBuffer()
		logBuffer = gbytes.NewBuffer()

		pusher = Pusher{
			courier,
			logger.DefaultLogger(logBuffer, logging.DEBUG, "extractor_test"),
		}

		deploymentInfo = S.DeploymentInfo{
			Username:  username,
			Password:  password,
			Org:       org,
			Space:     space,
			AppName:   appName,
			SkipSSL:   skipSSL,
			Instances: instances,
			Domain:    domain,
		}
	})

	Describe("logging in", func() {
		Context("when login succeeds", func() {
			It("gives the correct info to the courier", func() {

				Expect(pusher.Login(foundationURL, deploymentInfo, response)).To(Succeed())

				Expect(courier.LoginCall.Received.FoundationURL).To(Equal(foundationURL))
				Expect(courier.LoginCall.Received.Username).To(Equal(username))
				Expect(courier.LoginCall.Received.Password).To(Equal(password))
				Expect(courier.LoginCall.Received.Org).To(Equal(org))
				Expect(courier.LoginCall.Received.Space).To(Equal(space))
				Expect(courier.LoginCall.Received.SkipSSL).To(Equal(skipSSL))
			})

			It("writes the output of the courier to the response", func() {
				courier.LoginCall.Returns.Output = []byte("login succeeded")

				Expect(pusher.Login(foundationURL, deploymentInfo, response)).To(Succeed())

				Eventually(response).Should(gbytes.Say("login succeeded"))
			})
		})

		Context("when login fails", func() {
			It("writes the output of the courier to the writer", func() {
				courier.LoginCall.Returns.Output = []byte("login failed")
				courier.LoginCall.Returns.Error = errors.New("bork")

				err := pusher.Login(foundationURL, deploymentInfo, response)
				Expect(err).To(MatchError(fmt.Sprintf("cannot login to %s: %s", foundationURL, "bork")))

				Eventually(response).Should(gbytes.Say("login failed"))
			})
		})
	})

	Describe("pushing an app", func() {
		Context("when an app with the same name already exists", func() {
			It("renames the existing app", func() {
				courier.RenameCall.Returns.Output = nil
				courier.RenameCall.Returns.Error = nil

				appExists = true

				Expect(pusher.Push(appPath, appExists, deploymentInfo, response)).To(Succeed())

				Expect(courier.RenameCall.Received.AppName).To(Equal(appName))
				Expect(courier.RenameCall.Received.AppNameVenerable).To(Equal(appNameVenerable))

				Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf("renamed app from %s to %s", appName, appNameVenerable)))
			})

			Context("renaming the existing app fails", func() {
				It("returns an error", func() {
					courier.RenameCall.Returns.Output = []byte("rename failed")
					courier.RenameCall.Returns.Error = errors.New("bork")

					appExists = true

					err := pusher.Push(appPath, appExists, deploymentInfo, response)
					Expect(err).To(MatchError("rename failed: bork"))

					Expect(courier.RenameCall.Received.AppName).To(Equal(appName))
					Expect(courier.RenameCall.Received.AppNameVenerable).To(Equal(appNameVenerable))
				})
			})
		})

		Context("when no app with the same name exists", func() {
			It("reports that the app is new", func() {
				Expect(pusher.Push(appPath, appExists, deploymentInfo, response)).To(Succeed())

				Eventually(logBuffer).Should(gbytes.Say("new app detected"))
			})
		})

		It("pushes the new app", func() {
			courier.PushCall.Returns.Output = []byte("push succeeded")

			Expect(pusher.Push(appPath, appExists, deploymentInfo, response)).To(Succeed())

			Expect(courier.PushCall.Received.AppName).To(Equal(appName))
			Expect(courier.PushCall.Received.AppPath).To(Equal(appPath))
			Expect(courier.PushCall.Received.Instances).To(Equal(instances))

			Eventually(response).Should(gbytes.Say("push succeeded"))

			Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf("pushing app %s to %s", appName, domain)))
			Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf("tempdir for app %s: %s", appName, appPath)))
			Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf("push succeeded")))
		})

		It("maps the route to the app", func() {
			courier.MapRouteCall.Returns.Output = []byte("mapped route")
			courier.MapRouteCall.Returns.Error = nil

			Expect(pusher.Push(appPath, appExists, deploymentInfo, response)).To(Succeed())

			Expect(courier.MapRouteCall.Received.AppName).To(Equal(appName))
			Expect(courier.MapRouteCall.Received.Domain).To(Equal(domain))

			Eventually(response).Should(gbytes.Say("mapped route"))

			Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf("mapping route for %s to %s", appName, domain)))
		})

		Context("when the push fails", func() {
			It("returns an error", func() {
				courier.PushCall.Returns.Error = errors.New("push error")

				err := pusher.Push(appPath, appExists, deploymentInfo, response)

				Expect(err).To(MatchError("push error"))

			})
		})
	})

	Describe("rolling back a deployment", func() {
		It("logs in, deletes, and renames", func() {
			courier.RenameCall.Returns.Output = nil
			courier.RenameCall.Returns.Error = nil
			courier.DeleteCall.Returns.Output = nil
			courier.DeleteCall.Returns.Error = nil

			Expect(pusher.Rollback(true, deploymentInfo)).To(Succeed())

			Expect(courier.RenameCall.Received.AppName).To(Equal(appNameVenerable))
			Expect(courier.RenameCall.Received.AppNameVenerable).To(Equal(appName))
			Expect(courier.DeleteCall.Received.AppName).To(Equal(appName))

			Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf("rolling back deploy of %s", appName)))
			Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf("deleted %s", appName)))
			Eventually(logBuffer).Should(gbytes.Say("renamed app from %s to %s", appNameVenerable, appName))
		})
	})

	Describe("completing a deployment", func() {
		It("deletes venerable", func() {
			courier.DeleteCall.Returns.Output = nil
			courier.DeleteCall.Returns.Error = nil

			Expect(pusher.DeleteVenerable(deploymentInfo, foundationURL)).To(Succeed())

			Expect(courier.DeleteCall.Received.AppName).To(Equal(appNameVenerable))

			Eventually(logBuffer).Should(gbytes.Say(fmt.Sprintf("deleted %s", appNameVenerable)))
		})
	})

	Describe("getting CF logs", func() {
		Context("when a push fails", func() {
			It("gets logs from the courier", func() {
				courier.PushCall.Returns.Error = errors.New("push error")
				courier.LogsCall.Returns.Output = []byte("cf logs")

				Expect(pusher.Push(appPath, appExists, deploymentInfo, response)).ToNot(Succeed())

				Eventually(response).Should(gbytes.Say(("cf logs")))
			})

			Context("when the courier log call fails", func() {
				It("returns an error", func() {
					courier.PushCall.Returns.Error = errors.New("push error")
					courier.LogsCall.Returns.Error = errors.New("logs error")

					err := pusher.Push(appPath, appExists, deploymentInfo, response)

					Expect(err).To(MatchError("push error: cannot get Cloud Foundry logs: logs error"))
				})
			})
		})

		Context("when MapRoute returns an error", func() {
			It("handles the error", func() {
				courier.MapRouteCall.Returns.Error = errors.New("map route failed")
				courier.LogsCall.Returns.Output = []byte("cf logs")

				err := pusher.Push(appPath, appExists, deploymentInfo, response)

				Expect(err).To(MatchError("map route failed"))

				Eventually(response).Should(gbytes.Say(("cf logs")))
			})

			Context("when the courier log call fails", func() {
				It("returns an error", func() {
					courier.MapRouteCall.Returns.Error = errors.New("map route failed")
					courier.LogsCall.Returns.Error = errors.New("logs error")

					err := pusher.Push(appPath, appExists, deploymentInfo, response)

					Expect(err).To(MatchError("cannot get Cloud Foundry logs: logs error"))
				})
			})
		})
	})

	Describe("cleaning up temporary directories", func() {
		It("is successful", func() {
			courier.CleanUpCall.Returns.Error = nil

			Expect(pusher.CleanUp()).To(Succeed())
		})
	})

	Describe("checking for an existing application", func() {
		It("it is successful", func() {
			courier.ExistsCall.Returns.Bool = true

			Expect(pusher.Exists(appName)).To(Equal(true))
		})
	})
})
