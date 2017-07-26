package config_test

import (
	"io/ioutil"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/amandamunoz/deployadactyl/config"
	"github.com/amandamunoz/deployadactyl/mocks"
	"github.com/amandamunoz/deployadactyl/randomizer"
)

const (
	customConfigPath = "./custom_test_config.yml"
	testConfig       = `---
environments:
- name: Test
  domain: test.example.com
  foundations:
  - api1.example.com
  - api2.example.com
  skip_ssl: true
  instances: 3
- name: Prod
  domain: example.com
  foundations:
  - api3.example.com
  - api4.example.com
  skip_ssl: false
`
	badConfigPath = "./test_bad_config.yml"
)

var _ = Describe("Config", func() {
	var (
		env        *mocks.Env
		envMap     map[string]Environment
		cfUsername string
		cfPassword string
	)

	BeforeEach(func() {
		cfUsername = "cfUsername-" + randomizer.StringRunes(10)
		cfPassword = "cfPassword-" + randomizer.StringRunes(10)

		env = &mocks.Env{}
		env.GetCall.Returns.Values = map[string]string{}

		envMap = map[string]Environment{
			"test": {
				Name:        "Test",
				Foundations: []string{"api1.example.com", "api2.example.com"},
				Domain:      "test.example.com",
				SkipSSL:     true,
				Instances:   3,
			},
			"prod": {
				Name:        "Prod",
				Foundations: []string{"api3.example.com", "api4.example.com"},
				Domain:      "example.com",
				SkipSSL:     false,
				Instances:   1,
			},
		}

		Expect(ioutil.WriteFile(customConfigPath, []byte(testConfig), 0644)).To(Succeed())
	})

	AfterEach(func() {
		Expect(os.RemoveAll(customConfigPath)).To(Succeed())
		Expect(os.RemoveAll(badConfigPath)).To(Succeed())
	})

	Context("when all environment variables are present", func() {
		It("returns a valid config", func() {
			env.GetCall.Returns.Values["CF_USERNAME"] = cfUsername
			env.GetCall.Returns.Values["CF_PASSWORD"] = cfPassword
			env.GetCall.Returns.Values["PORT"] = ""

			config, err := Custom(env.Get, customConfigPath)
			Expect(err).ToNot(HaveOccurred())

			Expect(config.Username).To(Equal(cfUsername))
			Expect(config.Password).To(Equal(cfPassword))
			Expect(config.Environments).To(Equal(envMap))
			Expect(config.Port).To(Equal(8080))
		})
	})

	Context("when PORT is in the environment", func() {
		It("uses the value as the port", func() {
			env.GetCall.Returns.Values["CF_USERNAME"] = cfUsername
			env.GetCall.Returns.Values["CF_PASSWORD"] = cfPassword
			env.GetCall.Returns.Values["PORT"] = "42"

			config, err := Custom(env.Get, customConfigPath)
			Expect(err).ToNot(HaveOccurred())

			Expect(config.Port).To(Equal(42))
		})
	})

	Context("when an environment variable is missing", func() {
		It("returns an error", func() {
			env.GetCall.Returns.Values["CF_USERNAME"] = ""
			env.GetCall.Returns.Values["CF_PASSWORD"] = cfPassword

			_, err := Custom(env.Get, customConfigPath)

			Expect(err).To(MatchError("missing environment variables: CF_USERNAME"))
		})
	})

	Context("when a bad config is given", func() {
		It("returns an error when environments key is empty", func() {
			env.GetCall.Returns.Values["CF_USERNAME"] = cfUsername
			env.GetCall.Returns.Values["CF_PASSWORD"] = cfPassword
			env.GetCall.Returns.Values["PORT"] = "42"

			testBadConfig := `--- ~`
			Expect(ioutil.WriteFile(badConfigPath, []byte(testBadConfig), 0644)).To(Succeed())

			badConfig, err := Custom(env.Get, badConfigPath)
			Expect(err).To(MatchError(EnvironmentsNotSpecifiedError{}))

			Expect(badConfig.Environments).To(BeEmpty())
		})

		Context("missing required parameters", func() {
			It("returns an error when name is missing", func() {
				testBadConfig := `---
environments:
  - name:
    foundations:
    - api1.example.com
`
				Expect(ioutil.WriteFile(badConfigPath, []byte(testBadConfig), 0644)).To(Succeed())

				badConfig, err := Custom(env.Get, badConfigPath)
				Expect(err).To(MatchError(MissingParameterError{}))

				Expect(badConfig.Environments).To(BeEmpty())
			})

			It("returns an error when foundations is missing", func() {
				testBadConfig := `---
environments:
- name: production
  domain: test.example.com
`
				Expect(ioutil.WriteFile(badConfigPath, []byte(testBadConfig), 0644)).To(Succeed())

				badConfig, err := Custom(env.Get, badConfigPath)
				Expect(err).To(MatchError(MissingParameterError{}))

				Expect(badConfig.Environments).To(BeEmpty())
			})
		})

		Context("when the number of instances is zero", func() {
			It("sets the number of instances to one", func() {
				env.GetCall.Returns.Values["CF_USERNAME"] = cfUsername
				env.GetCall.Returns.Values["CF_PASSWORD"] = cfPassword

				testBadConfig := `---
environments:
- name: production
  foundations:
  - api1.example.com
  - api2.example.com
  domain: example.com
  instances: 0
`

				Expect(ioutil.WriteFile(badConfigPath, []byte(testBadConfig), 0644)).To(Succeed())

				badConfig, err := Custom(env.Get, badConfigPath)

				Expect(badConfig.Environments["production"].Instances).To(Equal(uint16(1)))
				Expect(err).ToNot(HaveOccurred())

			})
		})
	})
})
