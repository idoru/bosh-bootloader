package terraform_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cloudfoundry/bosh-bootloader/terraform"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("Cmd", func() {
	var (
		stdout *bytes.Buffer
		stderr *bytes.Buffer

		cmd terraform.Cmd

		fakeTerraformBackendServer *httptest.Server
		pathToFakeTerraform        string
		pathToTerraform            string
		fastFailTerraform          bool
		fastFailTerraformMutex     sync.Mutex

		terraformArgs      []string
		terraformArgsMutex sync.Mutex
	)

	var setFastFailTerraform = func(on bool) {
		fastFailTerraformMutex.Lock()
		defer fastFailTerraformMutex.Unlock()
		fastFailTerraform = on
	}

	var getFastFailTerraform = func() bool {
		fastFailTerraformMutex.Lock()
		defer fastFailTerraformMutex.Unlock()
		return fastFailTerraform
	}

	BeforeEach(func() {
		stdout = bytes.NewBuffer([]byte{})
		stderr = bytes.NewBuffer([]byte{})

		cmd = terraform.NewCmd(stderr)

		fakeTerraformBackendServer = httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
			if getFastFailTerraform() {
				responseWriter.WriteHeader(http.StatusInternalServerError)
			}

			if request.Method == "POST" {
				terraformArgsMutex.Lock()
				defer terraformArgsMutex.Unlock()
				body, err := ioutil.ReadAll(request.Body)
				if err != nil {
					panic(err)
				}

				err = json.Unmarshal(body, &terraformArgs)
				if err != nil {
					panic(err)
				}
			}
		}))

		var err error
		pathToFakeTerraform, err = gexec.Build("github.com/cloudfoundry/bosh-bootloader/bbl/faketerraform",
			"--ldflags", fmt.Sprintf("-X main.backendURL=%s", fakeTerraformBackendServer.URL))
		Expect(err).NotTo(HaveOccurred())

		pathToTerraform = filepath.Join(filepath.Dir(pathToFakeTerraform), "terraform")
		err = os.Rename(pathToFakeTerraform, pathToTerraform)
		Expect(err).NotTo(HaveOccurred())

		os.Setenv("PATH", strings.Join([]string{filepath.Dir(pathToTerraform), os.Getenv("PATH")}, ":"))
	})

	It("runs terraform with args", func() {
		err := cmd.Run(stdout, "/tmp", []string{"apply", "some-arg"}, false)
		Expect(err).NotTo(HaveOccurred())

		terraformArgsMutex.Lock()
		defer terraformArgsMutex.Unlock()
		Expect(terraformArgs).To(Equal([]string{"apply", "some-arg"}))

		Expect(stdout).NotTo(MatchRegexp("working directory: (.*)/tmp"))
		Expect(stdout).NotTo(ContainSubstring("apply some-arg"))
	})

	It("redirects command stdout to provided stdout when debug is true", func() {
		err := cmd.Run(stdout, "/tmp", []string{"apply", "some-arg"}, true)
		Expect(err).NotTo(HaveOccurred())

		Expect(stdout).To(MatchRegexp("working directory: (.*)/tmp"))
		Expect(stdout).To(ContainSubstring("apply some-arg"))
	})

	Context("failure case", func() {
		BeforeEach(func() {
			setFastFailTerraform(true)
		})

		AfterEach(func() {
			setFastFailTerraform(false)
		})

		It("returns an error when terraform fails", func() {
			err := cmd.Run(stdout, "", []string{"fast-fail"}, false)
			Expect(err).To(MatchError("exit status 1"))
		})

		It("redirects command stderr to provided stderr when debug is true", func() {
			_ = cmd.Run(stdout, "", []string{"fast-fail"}, true)
			Expect(stderr).To(ContainSubstring("failed to terraform"))
		})
	})
})
