package deployer_test

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"

	"github.com/compozed/deployadactyl/config"
	. "github.com/compozed/deployadactyl/controller/deployer"
	"github.com/compozed/deployadactyl/logger"
	"github.com/compozed/deployadactyl/mocks"
	"github.com/compozed/deployadactyl/randomizer"
	S "github.com/compozed/deployadactyl/structs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/op/go-logging"
	"github.com/spf13/afero"
)

const (
	deployAborted = "Deploy aborted, one or more CF foundations unavailable"
	testManifest  = `---
applications:
- name: deployadactyl
  memory: 256M
  disk_quota: 256M
`
	testManifestLocation       = "./manifest.yml"
	jsonRequest                = "application/json"
	zipRequest                 = "application/zip"
	eventManagerNotEnoughCalls = "event manager didn't have the right number of calls"
)

var _ = Describe("Deployer", func() {
	var (
		deployer Deployer

		c              config.Config
		blueGreener    *mocks.BlueGreener
		fetcher        *mocks.Fetcher
		prechecker     *mocks.Prechecker
		eventManager   *mocks.EventManager
		randomizerMock *mocks.Randomizer

		req             *http.Request
		reqBuffer       *bytes.Buffer
		appName         string
		appPath         string
		artifactURL     string
		domain          string
		environmentName string
		org             string
		space           string
		username        string
		uuid            string
		instances       uint16
		password        string
		buffer          *bytes.Buffer

		deploymentInfo  S.DeploymentInfo
		event           S.Event
		deployEventData S.DeployEventData
		foundations     []string
		environments    = map[string]config.Environment{}
		log             = logger.DefaultLogger(GinkgoWriter, logging.DEBUG, "test")
		af              *afero.Afero
	)

	BeforeEach(func() {
		blueGreener = &mocks.BlueGreener{}
		fetcher = &mocks.Fetcher{}
		prechecker = &mocks.Prechecker{}
		eventManager = &mocks.EventManager{}
		randomizerMock = &mocks.Randomizer{}

		appName = "appName-" + randomizer.StringRunes(10)
		appPath = "appPath-" + randomizer.StringRunes(10)
		artifactURL = "artifactURL-" + randomizer.StringRunes(10)
		domain = "domain-" + randomizer.StringRunes(10)
		environmentName = "environmentName-" + randomizer.StringRunes(10)
		org = "org-" + randomizer.StringRunes(10)
		password = "password-" + randomizer.StringRunes(10)
		space = "space-" + randomizer.StringRunes(10)
		username = "username-" + randomizer.StringRunes(10)
		uuid = "uuid-" + randomizer.StringRunes(10)
		instances = uint16(rand.Uint32())

		randomizerMock.RandomizeCall.Returns.Runes = uuid

		reqBuffer = bytes.NewBufferString(fmt.Sprintf(`{
		  		"artifact_url": "%s"
		  	}`,
			artifactURL,
		))
		req, _ = http.NewRequest("POST", "", reqBuffer)

		deploymentInfo = S.DeploymentInfo{
			ArtifactURL: artifactURL,
			Username:    username,
			Password:    password,
			Environment: environmentName,
			Org:         org,
			Space:       space,
			AppName:     appName,
			UUID:        uuid,
			Instances:   instances,
			Manifest:    "",
		}

		deployEventData = S.DeployEventData{
			Writer:         &bytes.Buffer{},
			DeploymentInfo: &deploymentInfo,
		}

		event = S.Event{
			Data: deployEventData,
		}

		randomizerMock.RandomizeCall.Returns.Runes = uuid

		foundations = []string{randomizer.StringRunes(10)}
		buffer = &bytes.Buffer{}

		environments = map[string]config.Environment{}
		environments[environmentName] = config.Environment{
			Name:        environmentName,
			Domain:      domain,
			Foundations: foundations,
			Instances:   instances,
		}

		c = config.Config{
			Username:     username,
			Password:     password,
			Environments: environments,
		}

		af = &afero.Afero{Fs: afero.NewMemMapFs()}

		deployer = Deployer{
			c,
			blueGreener,
			fetcher,
			prechecker,
			eventManager,
			randomizerMock,
			log,
			af,
		}
	})

	Describe("deploy JSON", func() {
		Context("when fetcher fails", func() {
			It("returns an error", func() {
				prechecker.AssertAllFoundationsUpCall.Returns.Error = nil

				fetcher.FetchCall.Returns.Error = errors.New("fetcher error")
				fetcher.FetchCall.Returns.AppPath = appPath

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, "application/json", buffer)
				Expect(err).To(MatchError("fetcher error"))

				Expect(statusCode).To(Equal(http.StatusInternalServerError))
				Expect(prechecker.AssertAllFoundationsUpCall.Received.Environment).To(Equal(environments[environmentName]))
				Expect(fetcher.FetchCall.Received.ArtifactURL).To(Equal(artifactURL))
				Expect(fetcher.FetchCall.Received.Manifest).To(BeEmpty())
			})
		})

		Context("with missing properties in the JSON", func() {
			It("returns an error", func() {
				By("sending empty JSON")
				reqBuffer = bytes.NewBufferString("{}")
				req, _ = http.NewRequest("POST", "", reqBuffer)

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
				Expect(err).To(MatchError("The following properties are missing: artifact_url"))
				Expect(statusCode).To(Equal(http.StatusInternalServerError))
			})
		})

		Context("when all applications start correctly", func() {
			It("is successful", func() {
				eventManager.EmitCall.Returns.Error = nil
				fetcher.FetchCall.Returns.AppPath = appPath
				fetcher.FetchCall.Returns.Error = nil
				blueGreener.PushCall.Returns.Error = nil
				prechecker.AssertAllFoundationsUpCall.Returns.Error = nil

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
				Expect(err).To(BeNil())
				Expect(statusCode).To(Equal(http.StatusOK))
				Expect(buffer).To(ContainSubstring("deploy was successful"))
				Expect(fetcher.FetchCall.Received.ArtifactURL).To(Equal(artifactURL))
				Expect(fetcher.FetchCall.Received.Manifest).To(BeEmpty())
				Expect(blueGreener.PushCall.Received.Environment).To(Equal(environments[environmentName]))
				Expect(blueGreener.PushCall.Received.AppPath).To(Equal(appPath))
				Expect(blueGreener.PushCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
				Expect(prechecker.AssertAllFoundationsUpCall.Received.Environment).To(Equal(environments[environmentName]))
			})
		})

		Context("when an application fails to start", func() {
			It("returns an error", func() {
				eventManager.EmitCall.Returns.Error = nil
				prechecker.AssertAllFoundationsUpCall.Returns.Error = nil
				fetcher.FetchCall.Returns.Error = nil
				fetcher.FetchCall.Returns.AppPath = appPath

				By("making bluegreener return an error")
				blueGreener.PushCall.Returns.Error = errors.New("blue green error")

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
				Expect(err).To(MatchError("blue green error"))
				Expect(statusCode).To(Equal(http.StatusInternalServerError))

				Expect(prechecker.AssertAllFoundationsUpCall.Received.Environment).To(Equal(environments[environmentName]))
				Expect(fetcher.FetchCall.Received.ArtifactURL).To(Equal(artifactURL))
				Expect(fetcher.FetchCall.Received.Manifest).To(BeEmpty())
				Expect(blueGreener.PushCall.Received.Environment).To(Equal(environments[environmentName]))
				Expect(blueGreener.PushCall.Received.AppPath).To(Equal(appPath))
				Expect(blueGreener.PushCall.Received.DeploymentInfo).To(Equal(deploymentInfo))
			})
		})

		Context("when eventmanager fails on deploy.start", func() {
			It("returns an error", func() {
				By("making eventmanager return an error")
				eventManager.EmitCall.Returns.Error = errors.New("event error")

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
				Expect(err).To(MatchError("an error occurred in the deploy.start event"))
				Expect(statusCode).To(Equal(http.StatusInternalServerError))
				Expect(buffer).To(ContainSubstring("event error"))
			})
		})

		Context("when custom manifest information is given in the request body", func() {
			It("properly decodes base64 encoding of the provided manifest information", func() {
				eventManager.EmitCall.Returns.Error = nil

				deploymentInfo.Manifest = "manifest-" + randomizer.StringRunes(10)

				By("base64 encoding a manifest")
				base64Manifest := base64.StdEncoding.EncodeToString([]byte(deploymentInfo.Manifest))

				By("including manifest in the JSON")
				reqBuffer = bytes.NewBufferString(fmt.Sprintf(`{
	 					"artifact_url": "%s",
	 					"manifest": "%s"
	 				}`,
					artifactURL,
					base64Manifest,
				))
				req, _ = http.NewRequest("POST", "", reqBuffer)

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
				Expect(err).ToNot(HaveOccurred())

				Expect(statusCode).To(Equal(http.StatusOK))
				Expect(buffer).To(ContainSubstring("deploy was successful"))
				Expect(eventManager.EmitCall.TimesCalled).To(Equal(3), eventManagerNotEnoughCalls)
			})

			It("returns an error if the provided manifest information is not base64 encoded", func() {
				deploymentInfo.Manifest = "manifest-" + randomizer.StringRunes(10)

				By("not base64 encoding a manifest")

				By("including manifest in the JSON")
				reqBuffer = bytes.NewBufferString(fmt.Sprintf(`{
	 					"artifact_url": "%s",
	 					"manifest": "%s"
	 				}`,
					artifactURL,
					deploymentInfo.Manifest,
				))
				req, _ = http.NewRequest("POST", "", reqBuffer)

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
				Expect(err).To(MatchError("cannot open manifest file"))
				Expect(statusCode).To(Equal(http.StatusBadRequest))

				Expect(eventManager.EmitCall.TimesCalled).To(Equal(0), eventManagerNotEnoughCalls)
			})
		})
	})

	Describe("deploy zip", func() {
		Context("when all applications start correctly", func() {
			It("accepts the request and returns http.StatusOK", func() {
				tempDir, err := af.TempDir("", "")
				Expect(err).To(BeNil())

				Expect(af.WriteFile(tempDir+"/manifest.yml", []byte(testManifest), 0644)).To(Succeed())

				eventManager.EmitCall.Returns.Error = nil
				prechecker.AssertAllFoundationsUpCall.Returns.Error = nil
				blueGreener.PushCall.Returns.Error = nil
				fetcher.FetchFromZipCall.Returns.AppPath = tempDir

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, zipRequest, buffer)
				Expect(err).To(BeNil())
				Expect(statusCode).To(Equal(http.StatusOK))

				Expect(buffer).To(ContainSubstring("deploy was successful"))
				Expect(prechecker.AssertAllFoundationsUpCall.Received.Environment).To(Equal(environments[environmentName]))
				Expect(blueGreener.PushCall.Received.Environment).To(Equal(environments[environmentName]))
				Expect(blueGreener.PushCall.Received.AppPath).To(Equal(tempDir))
				Expect(blueGreener.PushCall.Received.DeploymentInfo.Manifest).To(Equal(fmt.Sprintf("---\napplications:\n- name: deployadactyl\n  memory: 256M\n  disk_quota: 256M\n")))
				Expect(blueGreener.PushCall.Received.DeploymentInfo.ArtifactURL).To(Equal("Local Developer App Deploy" + " " + tempDir))
			})
		})

		Context("when manifest file cannot be found in the extracted zip", func() {
			It("continues to deploy successfully", func() {
				eventManager.EmitCall.Returns.Error = nil
				prechecker.AssertAllFoundationsUpCall.Returns.Error = nil
				blueGreener.PushCall.Returns.Error = nil

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, zipRequest, buffer)
				Expect(err).To(BeNil())

				Expect(statusCode).To(Equal(http.StatusOK))
				Expect(buffer).To(ContainSubstring("deploy was successful"))
			})
		})

		Context("push fails", func() {
			It("rejects the request with a http.StatusInternalServerError", func() {
				tempDir, err := af.TempDir("", "")
				Expect(err).To(BeNil())

				Expect(af.WriteFile(tempDir+"/manifest.yml", []byte(testManifest), 0644)).To(Succeed())

				eventManager.EmitCall.Returns.Error = nil
				prechecker.AssertAllFoundationsUpCall.Returns.Error = nil
				fetcher.FetchFromZipCall.Returns.AppPath = tempDir

				By("making bluegreener return an error")
				blueGreener.PushCall.Returns.Error = errors.New("blue green error")

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, zipRequest, buffer)
				Expect(err).To(MatchError("blue green error"))

				Expect(statusCode).To(Equal(http.StatusInternalServerError))
				Expect(prechecker.AssertAllFoundationsUpCall.Received.Environment).To(Equal(environments[environmentName]))
				Expect(blueGreener.PushCall.Received.Environment).To(Equal(environments[environmentName]))
				Expect(blueGreener.PushCall.Received.AppPath).To(Equal(tempDir))
				Expect(blueGreener.PushCall.Received.DeploymentInfo.Manifest).To(Equal(fmt.Sprintf("---\napplications:\n- name: deployadactyl\n  memory: 256M\n  disk_quota: 256M\n")))
				Expect(blueGreener.PushCall.Received.DeploymentInfo.ArtifactURL).To(Equal("Local Developer App Deploy" + " " + tempDir))
			})
		})

		Context("deploy event handler fails", func() {
			It("rejects the request with a http.StatusInternalServerError", func() {
				eventManager.EmitCall.Returns.Error = errors.New("event error")
				prechecker.AssertAllFoundationsUpCall.Returns.Error = nil

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, zipRequest, buffer)
				Expect(err).To(MatchError("an error occurred in the deploy.start event"))

				Expect(statusCode).To(Equal(http.StatusInternalServerError))
				Expect(buffer).To(ContainSubstring("event error"))
			})
		})
	})

	Context("when authentication is required and a username and password are provided", func() {
		It("accepts the request with a http.StatusOK", func() {
			eventManager.EmitCall.Returns.Error = nil

			By("setting authenticate to true")
			deployer.Config.Environments[environmentName] = config.Environment{Authenticate: true}

			By("setting basic auth")
			username = "username-" + deployer.Randomizer.StringRunes(10)
			password = "password-" + deployer.Randomizer.StringRunes(10)
			req.SetBasicAuth(username, password)

			err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
			Expect(err).ToNot(HaveOccurred())
			Expect(statusCode).To(Equal(http.StatusOK))

			Expect(buffer).To(ContainSubstring("deploy was successful"))
			Expect(eventManager.EmitCall.TimesCalled).To(Equal(3), eventManagerNotEnoughCalls)
			Expect(buffer).To(ContainSubstring(fmt.Sprintf("Username:     %s", username)))
		})
	})

	Context("when authentication is required", func() {
		Context("a username and password are not provided", func() {
			It("rejects the request with a http.StatusUnauthorized", func() {
				By("setting authenticate to true")
				deployer.Config.Environments[environmentName] = config.Environment{Authenticate: true}

				By("not setting basic auth")

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
				Expect(err).To(MatchError("basic auth header not found"))
				Expect(statusCode).To(Equal(http.StatusUnauthorized))

				Expect(eventManager.EmitCall.TimesCalled).To(Equal(0), eventManagerNotEnoughCalls)
			})
		})

		Context("the username and password are incorrect", func() {
			It("rejects the request with a http.StatusUnauthorized", func() {
				eventManager.EmitCall.Returns.Error = nil
				fetcher.FetchCall.Returns.AppPath = appPath
				fetcher.FetchCall.Returns.Error = nil
				blueGreener.PushCall.Returns.Error = errors.New("push failed: login failed")

				By("setting authenticate to true")
				deployer.Config.Environments[environmentName] = config.Environment{Authenticate: true}

				By("providing false credentials")
				username = "username-" + deployer.Randomizer.StringRunes(10)
				password = "password-" + deployer.Randomizer.StringRunes(10)
				req.SetBasicAuth(username, password)

				err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
				Expect(err).To(MatchError("push failed: login failed"))
				Expect(statusCode).To(Equal(http.StatusBadRequest))

				Expect(eventManager.EmitCall.TimesCalled).To(Equal(3), eventManagerNotEnoughCalls)
			})
		})
	})

	Context("when authentication is not required", func() {
		It("uses the config username and password and accepts the request with a http.StatusOK", func() {
			eventManager.EmitCall.Returns.Error = nil

			By("setting authenticate to true")
			deployer.Config.Environments[environmentName] = config.Environment{Authenticate: false}

			err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
			Expect(err).ToNot(HaveOccurred())
			Expect(statusCode).To(Equal(http.StatusOK))

			Expect(buffer).To(ContainSubstring("deploy was successful"))
			Expect(eventManager.EmitCall.TimesCalled).To(Equal(3), eventManagerNotEnoughCalls)
			Expect(buffer).To(ContainSubstring(fmt.Sprintf("Username:     %s", username)))
		})
	})

	Context("with no environments", func() {
		It("returns an error", func() {
			errorMessage := "environment not found: " + environmentName

			eventManager.EmitCall.Returns.Error = nil

			emptyConfiguration := config.Config{
				Username:     "",
				Password:     "",
				Environments: nil,
			}

			deployer = Deployer{emptyConfiguration,
				blueGreener,
				fetcher,
				prechecker,
				eventManager,
				randomizerMock,
				log,
				&afero.Afero{Fs: afero.NewMemMapFs()},
			}

			err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
			Expect(err).To(MatchError(errorMessage))
			Expect(buffer).To(ContainSubstring(errorMessage))

			Expect(statusCode).To(Equal(http.StatusInternalServerError))
		})
	})

	Context("deployer prechecker fails", func() {
		It("rejects the request with a http.StatusInternalServerError", func() {
			prechecker.AssertAllFoundationsUpCall.Returns.Error = errors.New(deployAborted)

			err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
			Expect(err).To(MatchError("Deploy aborted, one or more CF foundations unavailable"))
			Expect(statusCode).To(Equal(http.StatusInternalServerError))

			Expect(prechecker.AssertAllFoundationsUpCall.Received.Environment).To(Equal(environments[environmentName]))
		})
	})

	Describe("deployment output", func() {
		It("shows the user deployment info properties", func() {
			eventManager.EmitCall.Returns.Error = nil

			err, statusCode := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
			Expect(err).ToNot(HaveOccurred())
			Expect(statusCode).To(Equal(http.StatusOK))

			Expect(buffer).To(ContainSubstring(artifactURL))
			Expect(buffer).To(ContainSubstring(username))
			Expect(buffer).To(ContainSubstring(environmentName))
			Expect(buffer).To(ContainSubstring(org))
			Expect(buffer).To(ContainSubstring(space))
			Expect(buffer).To(ContainSubstring(appName))

			Expect(eventManager.EmitCall.TimesCalled).To(Equal(3), eventManagerNotEnoughCalls)
		})
	})

	Describe("setting number of instances in deployment", func() {
		Context("when a manifest with instances is provided", func() {
			It("uses manifest instances", func() {
				deploymentInfo.Manifest = `---
applications:
- name: deployadactyl
  instances: 1337
`
				Expect(ioutil.WriteFile(testManifestLocation, []byte(deploymentInfo.Manifest), 0644)).To(Succeed())

				base64Manifest := base64.StdEncoding.EncodeToString([]byte(deploymentInfo.Manifest))

				reqBuffer = bytes.NewBufferString(fmt.Sprintf(`{
	 					"artifact_url": "%s",
	 					"manifest": "%s"
	 				}`,
					artifactURL,
					base64Manifest,
				))
				req, _ = http.NewRequest("POST", "", reqBuffer)

				err, _ := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
				Expect(err).ToNot(HaveOccurred())

				Expect(blueGreener.PushCall.Received.DeploymentInfo.Instances).To(Equal(uint16(1337)))
			})
		})

		Context("when manifest is empty", func() {
			It("uses config instances", func() {
				instancesManifest := ""
				Expect(ioutil.WriteFile(testManifestLocation, []byte(instancesManifest), 0644)).To(Succeed())

				err, _ := deployer.Deploy(req, environmentName, org, space, appName, jsonRequest, buffer)
				Expect(err).ToNot(HaveOccurred())

				Expect(blueGreener.PushCall.Received.DeploymentInfo.Instances).To(Equal(instances))
			})
		})
	})
})
