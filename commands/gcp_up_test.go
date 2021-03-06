package commands_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/cloudfoundry/bosh-bootloader/boshinit"
	"github.com/cloudfoundry/bosh-bootloader/cloudconfig/gcp"
	"github.com/cloudfoundry/bosh-bootloader/commands"
	"github.com/cloudfoundry/bosh-bootloader/fakes"
	"github.com/cloudfoundry/bosh-bootloader/ssl"
	"github.com/cloudfoundry/bosh-bootloader/storage"
	"github.com/cloudfoundry/bosh-bootloader/terraform"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("gcp up", func() {
	var (
		gcpUp                   commands.GCPUp
		stateStore              *fakes.StateStore
		keyPairUpdater          *fakes.GCPKeyPairUpdater
		gcpClientProvider       *fakes.GCPClientProvider
		terraformExecutor       *fakes.TerraformExecutor
		terraformOutputter      *fakes.TerraformOutputter
		boshDeployer            *fakes.BOSHDeployer
		stringGenerator         *fakes.StringGenerator
		boshClientProvider      *fakes.BOSHClientProvider
		boshClient              *fakes.BOSHClient
		gcpCloudConfigGenerator *fakes.GCPCloudConfigGenerator
		logger                  *fakes.Logger
		zones                   *fakes.Zones
		boshInitCredentials     map[string]string

		serviceAccountKeyPath     string
		serviceAccountKey         string
		expectedTerraformTemplate string
	)

	BeforeEach(func() {
		stateStore = &fakes.StateStore{}
		keyPairUpdater = &fakes.GCPKeyPairUpdater{}
		gcpClientProvider = &fakes.GCPClientProvider{}
		terraformExecutor = &fakes.TerraformExecutor{}
		zones = &fakes.Zones{}
		terraformExecutor.ApplyCall.Returns.TFState = "some-tf-state"
		stringGenerator = &fakes.StringGenerator{}
		stringGenerator.GenerateCall.Stub = func(prefix string, length int) (string, error) {
			return fmt.Sprintf("%s%s", prefix, "some-random-string"), nil
		}
		boshClientProvider = &fakes.BOSHClientProvider{}
		boshClient = &fakes.BOSHClient{}
		boshClientProvider.ClientCall.Returns.Client = boshClient
		gcpCloudConfigGenerator = &fakes.GCPCloudConfigGenerator{}

		boshInitCredentials = map[string]string{
			"mbusUsername":              "some-mbus-username",
			"natsUsername":              "some-nats-username",
			"postgresUsername":          "some-postgres-username",
			"registryUsername":          "some-registry-username",
			"blobstoreDirectorUsername": "some-blobstore-director-username",
			"blobstoreAgentUsername":    "some-blobstore-agent-username",
			"hmUsername":                "some-hm-username",
			"mbusPassword":              "some-mbus-password",
			"natsPassword":              "some-nats-password",
			"postgresPassword":          "some-postgres-password",
			"registryPassword":          "some-registry-password",
			"blobstoreDirectorPassword": "some-blobstore-director-password",
			"blobstoreAgentPassword":    "some-blobstore-agent-password",
			"hmPassword":                "some-hm-password",
		}

		logger = &fakes.Logger{}
		boshDeployer = &fakes.BOSHDeployer{}
		boshDeployer.DeployCall.Returns.Output = boshinit.DeployOutput{
			DirectorSSLKeyPair: ssl.KeyPair{
				CA:          []byte("updated-ca"),
				Certificate: []byte("updated-certificate"),
				PrivateKey:  []byte("updated-private-key"),
			},
			BOSHInitState: boshinit.State{
				"updated-key": "updated-value",
			},
			BOSHInitManifest: "name: bosh",
			Credentials:      boshInitCredentials,
		}

		terraformOutputter = &fakes.TerraformOutputter{}
		terraformOutputter.GetCall.Stub = func(output string) (string, error) {
			switch output {
			case "network_name":
				return "bbl-lake-time:stamp-network", nil
			case "subnetwork_name":
				return "bbl-lake-time:stamp-subnet", nil
			case "bosh_open_tag_name":
				return "bbl-lake-time:stamp-bosh-open", nil
			case "internal_tag_name":
				return "bbl-lake-time:stamp-internal", nil
			case "external_ip":
				return "some-external-ip", nil
			case "director_address":
				return "some-director-address", nil
			default:
				return "", nil
			}
		}

		gcpUp = commands.NewGCPUp(stateStore, keyPairUpdater, gcpClientProvider, terraformExecutor, boshDeployer,
			stringGenerator, logger, boshClientProvider, gcpCloudConfigGenerator, terraformOutputter, zones)

		tempFile, err := ioutil.TempFile("", "gcpServiceAccountKey")
		Expect(err).NotTo(HaveOccurred())

		serviceAccountKeyPath = tempFile.Name()
		serviceAccountKey = `{"real": "json"}`
		err = ioutil.WriteFile(serviceAccountKeyPath, []byte(serviceAccountKey), os.ModePerm)
		Expect(err).NotTo(HaveOccurred())

		body, err := ioutil.ReadFile("fixtures/terraform_template_no_lb.tf")
		Expect(err).NotTo(HaveOccurred())

		expectedTerraformTemplate = string(body)
	})

	AfterEach(func() {
		commands.ResetMarshal()
	})

	Context("Execute", func() {
		It("saves gcp details to the state", func() {
			keyPairUpdater.UpdateCall.Returns.KeyPair = storage.KeyPair{
				PrivateKey: "some-private-key",
				PublicKey:  "some-public-key",
			}

			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: serviceAccountKeyPath,
				ProjectID:             "some-project-id",
				Zone:                  "some-zone",
				Region:                "us-west1",
			}, storage.State{})
			Expect(err).NotTo(HaveOccurred())

			Expect(stateStore.SetCall.Receives.State.IAAS).To(Equal("gcp"))
			Expect(stateStore.SetCall.Receives.State.GCP).To(Equal(storage.GCP{
				ServiceAccountKey: serviceAccountKey,
				ProjectID:         "some-project-id",
				Zone:              "some-zone",
				Region:            "us-west1",
			}))
			Expect(stateStore.SetCall.Receives.State.KeyPair).To(Equal(storage.KeyPair{
				PrivateKey: "some-private-key",
				PublicKey:  "some-public-key",
			}))
			Expect(gcpClientProvider.SetConfigCall.CallCount).To(Equal(1))
			Expect(gcpClientProvider.SetConfigCall.Receives.ServiceAccountKey).To(Equal(`{"real": "json"}`))
			Expect(gcpClientProvider.SetConfigCall.Receives.ProjectID).To(Equal("some-project-id"))
			Expect(gcpClientProvider.SetConfigCall.Receives.Zone).To(Equal("some-zone"))
		})

		It("uploads the ssh keys", func() {
			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: serviceAccountKeyPath,
				ProjectID:             "some-project-id",
				Zone:                  "some-zone",
				Region:                "us-west1",
			}, storage.State{})
			Expect(err).NotTo(HaveOccurred())

			Expect(keyPairUpdater.UpdateCall.CallCount).To(Equal(1))
		})

		Context("terraform apply", func() {
			It("creates gcp resources via terraform", func() {
				gcpUpConfig := commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					ProjectID:             "some-project-id",
					Zone:                  "some-zone",
					Region:                "us-west1",
				}

				err := gcpUp.Execute(gcpUpConfig, storage.State{
					EnvID: "some-env-id",
				})

				Expect(err).NotTo(HaveOccurred())

				Expect(terraformExecutor.ApplyCall.CallCount).To(Equal(1))
				Expect(terraformExecutor.ApplyCall.Receives.Credentials).To(Equal(serviceAccountKey))
				Expect(terraformExecutor.ApplyCall.Receives.EnvID).To(Equal("some-env-id"))
				Expect(terraformExecutor.ApplyCall.Receives.ProjectID).To(Equal("some-project-id"))
				Expect(terraformExecutor.ApplyCall.Receives.Zone).To(Equal("some-zone"))
				Expect(terraformExecutor.ApplyCall.Receives.Region).To(Equal("us-west1"))
				Expect(terraformExecutor.ApplyCall.Receives.Template).To(Equal(expectedTerraformTemplate))
				Expect(stateStore.SetCall.Receives.State.TFState).To(Equal("some-tf-state"))
			})

			It("saves the tf state even if the applier fails", func() {
				expectedError := terraform.NewTerraformApplyError("some-tf-state", errors.New("failed to apply"))
				terraformExecutor.ApplyCall.Returns.Error = expectedError

				err := gcpUp.Execute(commands.GCPUpConfig{}, storage.State{
					IAAS: "gcp",
					GCP: storage.GCP{
						ServiceAccountKey: serviceAccountKey,
						ProjectID:         "some-project-id",
						Zone:              "some-zone",
						Region:            "us-west1",
					},
					EnvID: "bbl-lake-time:stamp",
				})

				Expect(err).To(MatchError("failed to apply"))
				Expect(stateStore.SetCall.CallCount).To(Equal(3))
				Expect(stateStore.SetCall.Receives.State.TFState).To(Equal("some-tf-state"))

			})
		})

		Context("bosh", func() {
			It("deploys a bosh", func() {
				err := gcpUp.Execute(commands.GCPUpConfig{}, storage.State{
					IAAS: "gcp",
					GCP: storage.GCP{
						ServiceAccountKey: serviceAccountKey,
						ProjectID:         "some-project-id",
						Zone:              "some-zone",
						Region:            "us-west1",
					},
					EnvID: "bbl-lake-time:stamp",
				})
				Expect(err).NotTo(HaveOccurred())

				Expect(boshDeployer.DeployCall.Receives.Input).To(Equal(boshinit.DeployInput{
					IAAS:             "gcp",
					DirectorName:     "bosh-bbl-lake-time:stamp",
					DirectorUsername: "user-some-random-string",
					DirectorPassword: "p-some-random-string",
					State:            map[string]interface{}{},
					InfrastructureConfiguration: boshinit.InfrastructureConfiguration{
						ExternalIP: "some-external-ip",
						GCP: boshinit.InfrastructureConfigurationGCP{
							Zone:           "some-zone",
							NetworkName:    "bbl-lake-time:stamp-network",
							SubnetworkName: "bbl-lake-time:stamp-subnet",
							BOSHTag:        "bbl-lake-time:stamp-bosh-open",
							InternalTag:    "bbl-lake-time:stamp-internal",
							Project:        "some-project-id",
							JsonKey:        serviceAccountKey,
						},
					},
				}))
			})

			Context("state manipulation", func() {
				Context("when the state file does not exist", func() {
					It("saves the bosh credentials, manifest and bosh-init state", func() {
						err := gcpUp.Execute(commands.GCPUpConfig{
							ServiceAccountKeyPath: serviceAccountKeyPath,
							ProjectID:             "some-project-id",
							Zone:                  "some-zone",
							Region:                "us-west1",
						}, storage.State{
							EnvID: "bbl-lake-time:stamp",
						})
						Expect(err).NotTo(HaveOccurred())

						Expect(stateStore.SetCall.Receives.State.BOSH).To(Equal(storage.BOSH{
							DirectorName:           "bosh-bbl-lake-time:stamp",
							DirectorUsername:       "user-some-random-string",
							DirectorPassword:       "p-some-random-string",
							DirectorAddress:        "some-director-address",
							DirectorSSLCA:          "updated-ca",
							DirectorSSLCertificate: "updated-certificate",
							DirectorSSLPrivateKey:  "updated-private-key",
							Credentials:            boshInitCredentials,
							State: map[string]interface{}{
								"updated-key": "updated-value",
							},
							Manifest: "name: bosh",
						}))
					})
				})
				Context("when the state file exists", func() {
					It("does not override the bosh credentials", func() {
						err := gcpUp.Execute(commands.GCPUpConfig{
							ServiceAccountKeyPath: serviceAccountKeyPath,
							ProjectID:             "some-project-id",
							Zone:                  "some-zone",
							Region:                "us-west1",
						}, storage.State{
							EnvID: "bbl-lake-time:stamp",
							BOSH: storage.BOSH{
								DirectorName:           "old-director-name",
								DirectorUsername:       "old-director-username",
								DirectorPassword:       "old-director-password",
								DirectorAddress:        "some--oldexternal-ip",
								DirectorSSLCA:          "old-ca",
								DirectorSSLCertificate: "old-certificate",
								DirectorSSLPrivateKey:  "old-private-key",
								Credentials:            map[string]string{"old": "credentials"},
							},
						})
						Expect(err).NotTo(HaveOccurred())

						Expect(stateStore.SetCall.Receives.State.BOSH).To(Equal(storage.BOSH{
							DirectorName:           "old-director-name",
							DirectorUsername:       "old-director-username",
							DirectorPassword:       "old-director-password",
							DirectorAddress:        "some--oldexternal-ip",
							DirectorSSLCA:          "old-ca",
							DirectorSSLCertificate: "old-certificate",
							DirectorSSLPrivateKey:  "old-private-key",
							Credentials:            map[string]string{"old": "credentials"},
							State: map[string]interface{}{
								"updated-key": "updated-value",
							},
							Manifest: "name: bosh",
						}))
					})

					It("updates the bosh manifest and state", func() {
						err := gcpUp.Execute(commands.GCPUpConfig{
							ServiceAccountKeyPath: serviceAccountKeyPath,
							ProjectID:             "some-project-id",
							Zone:                  "some-zone",
							Region:                "us-west1",
						}, storage.State{
							EnvID: "bbl-lake-time:stamp",
							BOSH: storage.BOSH{
								State: map[string]interface{}{
									"old-key": "old-value",
								},
								Manifest: "name: old-bosh",
							},
						})
						Expect(err).NotTo(HaveOccurred())

						Expect(stateStore.SetCall.Receives.State.BOSH.State).To(Equal(map[string]interface{}{
							"updated-key": "updated-value",
						}))
						Expect(stateStore.SetCall.Receives.State.BOSH.Manifest).To(Equal("name: bosh"))
					})
				})
			})

			Context("failure cases", func() {
				DescribeTable("returns an error when we fail to get an output", func(outputName string) {
					terraformOutputter.GetCall.Stub = func(output string) (string, error) {
						if output == outputName {
							return "", errors.New("failed to get output")
						}
						return "", nil
					}

					err := gcpUp.Execute(commands.GCPUpConfig{
						ServiceAccountKeyPath: serviceAccountKeyPath,
						ProjectID:             "some-project-id",
						Zone:                  "some-zone",
						Region:                "us-west1",
					}, storage.State{})
					Expect(err).To(MatchError("failed to get output"))
				},
					Entry("failed to get external_ip", "external_ip"),
					Entry("failed to get network_name", "network_name"),
					Entry("failed to get subnetwork_name", "subnetwork_name"),
					Entry("failed to get bosh_open_tag_name", "bosh_open_tag_name"),
					Entry("failed to get internal_tag_name", "internal_tag_name"),
					Entry("failed to get director_address", "director_address"),
				)

				It("returns an error if applier fails with non terraform apply error", func() {
					terraformExecutor.ApplyCall.Returns.Error = errors.New("failed to apply")
					err := gcpUp.Execute(commands.GCPUpConfig{
						ServiceAccountKeyPath: serviceAccountKeyPath,
						ProjectID:             "some-project-id",
						Zone:                  "some-zone",
						Region:                "us-west1",
					}, storage.State{
						IAAS: "gcp",
						Stack: storage.Stack{
							LBType: "concourse",
						},
					})
					Expect(err).To(MatchError("failed to apply"))
					Expect(stateStore.SetCall.CallCount).To(Equal(2))
				})

				It("returns an error when boshinit fails to create the deploy input", func() {
					stringGenerator.GenerateCall.Stub = nil
					stringGenerator.GenerateCall.Returns.Error = errors.New("failed to generate string")

					err := gcpUp.Execute(commands.GCPUpConfig{
						ServiceAccountKeyPath: serviceAccountKeyPath,
						ProjectID:             "some-project-id",
						Zone:                  "some-zone",
						Region:                "us-west1",
					}, storage.State{
						BOSH: storage.BOSH{
							DirectorUsername: "some-username",
						},
					})
					Expect(err).To(MatchError("failed to generate string"))
				})

				It("returns an error when boshdeployer fails to deploy", func() {
					boshDeployer.DeployCall.Returns.Error = errors.New("failed to deploy")

					err := gcpUp.Execute(commands.GCPUpConfig{
						ServiceAccountKeyPath: serviceAccountKeyPath,
						ProjectID:             "some-project-id",
						Zone:                  "some-zone",
						Region:                "us-west1",
					}, storage.State{})
					Expect(err).To(MatchError("failed to deploy"))
				})

				It("returns an error when the state fails to be set after deploying bosh", func() {
					stateStore.SetCall.Returns = []fakes.SetCallReturn{{}, {}, {}, {errors.New("state failed to be set")}}

					err := gcpUp.Execute(commands.GCPUpConfig{
						ServiceAccountKeyPath: serviceAccountKeyPath,
						ProjectID:             "some-project-id",
						Zone:                  "some-zone",
						Region:                "us-west1",
					}, storage.State{})
					Expect(err).To(MatchError("state failed to be set"))
				})
			})
		})
	})

	Context("cloud config", func() {
		It("generates and uploads a cloud config", func() {
			zones.GetCall.Returns.Zones = []string{"zone-1", "zone-2", "zone-3"}
			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: serviceAccountKeyPath,
				ProjectID:             "some-project-id",
				Zone:                  "some-zone",
				Region:                "some-region",
			}, storage.State{
				EnvID: "bbl-lake-time:stamp",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(boshClientProvider.ClientCall.Receives.DirectorAddress).To(Equal("some-director-address"))
			Expect(boshClientProvider.ClientCall.Receives.DirectorUsername).To(Equal("user-some-random-string"))
			Expect(boshClientProvider.ClientCall.Receives.DirectorPassword).To(Equal("p-some-random-string"))

			Expect(zones.GetCall.CallCount).To(Equal(1))
			Expect(zones.GetCall.Receives.Region).To(Equal("some-region"))

			gcpCloudConfigGenerator.GenerateCall.Returns.CloudConfig = gcp.CloudConfig{}
			Expect(gcpCloudConfigGenerator.GenerateCall.Receives.CloudConfigInput).To(Equal(gcp.CloudConfigInput{
				AZs:            []string{"zone-1", "zone-2", "zone-3"},
				Tags:           []string{"bbl-lake-time:stamp-internal"},
				NetworkName:    "bbl-lake-time:stamp-network",
				SubnetworkName: "bbl-lake-time:stamp-subnet",
			}))

			Expect(boshClient.UpdateCloudConfigCall.CallCount).To(Equal(1))
		})

		Context("failure cases", func() {
			It("returns an error when the cloud config fails to be generated", func() {
				gcpCloudConfigGenerator.GenerateCall.Returns.Error = errors.New("failed to generate cloud config")

				err := gcpUp.Execute(commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					ProjectID:             "some-project-id",
					Zone:                  "some-zone",
					Region:                "us-west1",
				}, storage.State{})
				Expect(err).To(MatchError("failed to generate cloud config"))
			})

			It("returns an error when the cloud config fails to be marshaled", func() {
				commands.SetMarshal(func(interface{}) ([]byte, error) {
					return []byte{}, errors.New("failed to marshal")
				})

				err := gcpUp.Execute(commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					ProjectID:             "some-project-id",
					Zone:                  "some-zone",
					Region:                "us-west1",
				}, storage.State{})
				Expect(err).To(MatchError("failed to marshal"))
			})

			It("returns an error when the cloud config fails to be updated", func() {
				boshClient.UpdateCloudConfigCall.Returns.Error = errors.New("failed to update cloud config")

				err := gcpUp.Execute(commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					ProjectID:             "some-project-id",
					Zone:                  "some-zone",
					Region:                "us-west1",
				}, storage.State{})
				Expect(err).To(MatchError("failed to update cloud config"))
			})
		})
	})

	Context("when state contains gcp details", func() {
		var (
			updatedServiceAccountKey     string
			updatedServiceAccountKeyPath string
		)

		BeforeEach(func() {
			tempFile, err := ioutil.TempFile("", "updatedGcpServiceAccountKey")
			Expect(err).NotTo(HaveOccurred())

			updatedServiceAccountKeyPath = tempFile.Name()
			updatedServiceAccountKey = `{"another-real": "json-file"}`
			err = ioutil.WriteFile(updatedServiceAccountKeyPath, []byte(updatedServiceAccountKey), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())
		})

		It("does not create a new ssh key", func() {
			err := gcpUp.Execute(commands.GCPUpConfig{}, storage.State{
				IAAS: "gcp",
				GCP: storage.GCP{
					ServiceAccountKey: serviceAccountKey,
					ProjectID:         "some-project-id",
					Zone:              "some-zone",
					Region:            "us-west1",
				},
				KeyPair: storage.KeyPair{
					Name: "some-key-name",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(keyPairUpdater.UpdateCall.CallCount).To(Equal(0))
		})

		It("calls terraform executor with previous tf state", func() {
			err := gcpUp.Execute(commands.GCPUpConfig{}, storage.State{
				IAAS: "gcp",
				GCP: storage.GCP{
					ServiceAccountKey: serviceAccountKey,
					ProjectID:         "some-project-id",
					Zone:              "some-zone",
					Region:            "us-west1",
				},
				TFState: "some-tf-state",
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(terraformExecutor.ApplyCall.CallCount).To(Equal(1))
			Expect(terraformExecutor.ApplyCall.Receives.TFState).To(Equal("some-tf-state"))
		})

		It("does not require details from up config", func() {
			err := gcpUp.Execute(commands.GCPUpConfig{}, storage.State{
				IAAS: "gcp",
				GCP: storage.GCP{
					ServiceAccountKey: serviceAccountKey,
					ProjectID:         "some-project-id",
					Zone:              "some-zone",
					Region:            "us-west1",
				},
			})
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTable("up config contains subset of the details", func(upConfig func() commands.GCPUpConfig, expectedErr string) {
			err := gcpUp.Execute(upConfig(), storage.State{
				IAAS: "gcp",
				GCP:  storage.GCP{},
			})
			Expect(err).To(MatchError(expectedErr))
		},
			Entry("returns an error when the service account key is not provided", func() commands.GCPUpConfig {
				return commands.GCPUpConfig{
					ProjectID: "new-project-id",
					Zone:      "new-zone",
					Region:    "new-region",
				}
			}, "GCP service account key must be provided"),
			Entry("returns an error when the project ID is not provided", func() commands.GCPUpConfig {
				return commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					Zone:   "new-zone",
					Region: "new-region",
				}
			}, "GCP project ID must be provided"),
			Entry("returns an error when the zone is not provided", func() commands.GCPUpConfig {
				return commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					ProjectID:             "new-project-id",
					Region:                "new-region",
				}
			}, "GCP zone must be provided"),
			Entry("returns an error when the region is not provided", func() commands.GCPUpConfig {
				return commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					ProjectID:             "new-project-id",
					Zone:                  "new-zone",
				}
			}, "GCP region must be provided"),
		)
	})

	Context("when lb type exists in the state", func() {
		It("applies the correct cf template and args for cf lb type", func() {
			zones.GetCall.Returns.Zones = []string{"some-zone", "some-other-zone"}
			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: serviceAccountKeyPath,
				ProjectID:             "some-project-id",
				Zone:                  "some-zone",
				Region:                "some-region",
			}, storage.State{
				LB: storage.LB{
					Type:   "cf",
					Cert:   "some-cert",
					Key:    "some-key",
					Domain: "some-domain",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(terraformExecutor.ApplyCall.CallCount).To(Equal(1))
			Expect(terraformExecutor.ApplyCall.Receives.Template).To(Equal(expectedCFTemplate))
			Expect(terraformExecutor.ApplyCall.Receives.Cert).To(Equal("some-cert"))
			Expect(terraformExecutor.ApplyCall.Receives.Key).To(Equal("some-key"))
			Expect(terraformExecutor.ApplyCall.Receives.Domain).To(Equal("some-domain"))
		})

		It("applies the correct concourse template and args for concourse lb type", func() {
			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: serviceAccountKeyPath,
				ProjectID:             "some-project-id",
				Zone:                  "some-zone",
				Region:                "some-region",
			}, storage.State{
				LB: storage.LB{
					Type: "concourse",
				},
			})
			Expect(err).NotTo(HaveOccurred())

			Expect(terraformExecutor.ApplyCall.CallCount).To(Equal(1))
			Expect(terraformExecutor.ApplyCall.Receives.Template).To(Equal(expectedConcourseTemplate))
			Expect(terraformExecutor.ApplyCall.Receives.Cert).To(Equal(""))
			Expect(terraformExecutor.ApplyCall.Receives.Key).To(Equal(""))
			Expect(terraformExecutor.ApplyCall.Receives.Domain).To(Equal(""))
		})
	})

	Context("failure cases", func() {
		Context("when calling up with different gcp flags then the state", func() {
			It("returns an error when the --gcp-region is different", func() {
				err := gcpUp.Execute(commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					ProjectID:             "some-project-id",
					Zone:                  "some-zone",
					Region:                "some-other-region",
				}, storage.State{
					GCP: storage.GCP{
						ServiceAccountKey: serviceAccountKey,
						ProjectID:         "some-project-id",
						Zone:              "some-zone",
						Region:            "us-west1",
					},
				})
				Expect(err).To(MatchError("The region cannot be changed for an existing environment. The current region is us-west1."))
			})

			It("returns an error when the --gcp-zone is different", func() {
				err := gcpUp.Execute(commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					ProjectID:             "some-project-id",
					Zone:                  "some-other-zone",
					Region:                "us-west1",
				}, storage.State{
					GCP: storage.GCP{
						ServiceAccountKey: serviceAccountKey,
						ProjectID:         "some-project-id",
						Zone:              "some-zone",
						Region:            "us-west1",
					},
				})
				Expect(err).To(MatchError("The zone cannot be changed for an existing environment. The current zone is some-zone."))
			})

			It("returns an error when the --gcp-project-id is different", func() {
				err := gcpUp.Execute(commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					ProjectID:             "some-other-project-id",
					Zone:                  "some-zone",
					Region:                "us-west1",
				}, storage.State{
					GCP: storage.GCP{
						ServiceAccountKey: serviceAccountKey,
						ProjectID:         "some-project-id",
						Zone:              "some-zone",
						Region:            "us-west1",
					},
				})
				Expect(err).To(MatchError("The project id cannot be changed for an existing environment. The current project id is some-project-id."))
			})
		})

		It("returns an error when state store fails", func() {
			stateStore.SetCall.Returns = []fakes.SetCallReturn{{Error: errors.New("set call failed")}}
			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: serviceAccountKeyPath,
				ProjectID:             "p",
				Zone:                  "z",
				Region:                "us-west1",
			}, storage.State{})
			Expect(err).To(MatchError("set call failed"))
		})

		It("should not store the state if the provided flags are not valid", func() {
			err := gcpUp.Execute(
				commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
				}, storage.State{})
			Expect(err).To(MatchError("GCP project ID must be provided"))
			Expect(stateStore.SetCall.CallCount).To(Equal(0))
		})

		DescribeTable("up config validation", func(upConfig func() commands.GCPUpConfig, expectedErr string) {
			err := gcpUp.Execute(upConfig(), storage.State{})
			Expect(err).To(MatchError(expectedErr))
		},
			Entry("returns an error when no flags are passed in", func() commands.GCPUpConfig {
				return commands.GCPUpConfig{}
			},
				"GCP service account key must be provided"),
			Entry("returns an error when service account key is missing", func() commands.GCPUpConfig {
				return commands.GCPUpConfig{
					ProjectID: "p",
					Zone:      "z",
					Region:    "us-west1",
				}
			}, "GCP service account key must be provided"),
			Entry("returns an error when project ID is missing", func() commands.GCPUpConfig {
				return commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					Zone:   "z",
					Region: "us-west1",
				}
			}, "GCP project ID must be provided"),
			Entry("returns an error when zone is missing", func() commands.GCPUpConfig {
				return commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					ProjectID:             "p",
					Region:                "us-west1",
				}
			}, "GCP zone must be provided"),
			Entry("returns an error when region is missing", func() commands.GCPUpConfig {
				return commands.GCPUpConfig{
					ServiceAccountKeyPath: serviceAccountKeyPath,
					ProjectID:             "p",
					Zone:                  "z",
				}
			}, "GCP region must be provided"),
		)

		It("returns an error when the service account key file does not exist", func() {
			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: "/some/non/existent/file",
				ProjectID:             "p",
				Zone:                  "z",
				Region:                "us-west1",
			}, storage.State{})
			Expect(err).To(MatchError("error reading service account key: open /some/non/existent/file: no such file or directory"))
		})

		It("returns an error when the service account key file does not contain valid json", func() {
			tempFile, err := ioutil.TempFile("", "")
			Expect(err).NotTo(HaveOccurred())

			invalidServiceAccountKeyPath := tempFile.Name()
			err = ioutil.WriteFile(invalidServiceAccountKeyPath, []byte(`%%%not-valid-json%%%`), os.ModePerm)
			Expect(err).NotTo(HaveOccurred())

			err = gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: invalidServiceAccountKeyPath,
				ProjectID:             "p",
				Zone:                  "z",
				Region:                "us-west1",
			}, storage.State{})
			Expect(err).To(MatchError("error parsing service account key: invalid character '%' looking for beginning of value"))
		})

		It("returns an error when the keypair could not be updated", func() {
			keyPairUpdater.UpdateCall.Returns.Error = errors.New("keypair update failed")

			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: serviceAccountKeyPath,
				ProjectID:             "some-project-id",
				Zone:                  "some-zone",
				Region:                "us-west1",
			}, storage.State{})
			Expect(err).To(MatchError("keypair update failed"))
		})

		It("returns an error when setting config fails", func() {
			gcpClientProvider.SetConfigCall.Returns.Error = errors.New("setting config failed")

			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: serviceAccountKeyPath,
				ProjectID:             "some-project-id",
				Zone:                  "some-zone",
				Region:                "us-west1",
			}, storage.State{})
			Expect(err).To(MatchError("setting config failed"))
		})

		It("saves the keypair when the terraform fails", func() {
			terraformExecutor.ApplyCall.Returns.Error = errors.New("terraform executor failed")
			keyPairUpdater.UpdateCall.Returns.KeyPair = storage.KeyPair{
				Name: "some-key-pair",
			}

			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: serviceAccountKeyPath,
				ProjectID:             "some-project-id",
				Zone:                  "some-zone",
				Region:                "us-west1",
			}, storage.State{})
			Expect(err).To(MatchError("terraform executor failed"))

			Expect(stateStore.SetCall.Receives.State.KeyPair.IsEmpty()).To(BeFalse())
		})

		It("returns an error when terraform executor fails", func() {
			terraformExecutor.ApplyCall.Returns.Error = errors.New("terraform executor failed")

			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: serviceAccountKeyPath,
				ProjectID:             "some-project-id",
				Zone:                  "some-zone",
				Region:                "us-west1",
			}, storage.State{})
			Expect(err).To(MatchError("terraform executor failed"))
		})

		It("returns an error when the state fails to be set after updating keypair", func() {
			stateStore.SetCall.Returns = []fakes.SetCallReturn{{}, {errors.New("state failed to be set")}}

			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: serviceAccountKeyPath,
				ProjectID:             "some-project-id",
				Zone:                  "some-zone",
				Region:                "us-west1",
			}, storage.State{})
			Expect(err).To(MatchError("state failed to be set"))
		})

		It("returns an error when both the applier fails and state fails to be set", func() {
			expectedError := terraform.NewTerraformApplyError("some-tf-state", errors.New("failed to apply"))
			terraformExecutor.ApplyCall.Returns.Error = expectedError
			terraformExecutor.ApplyCall.Returns.TFState = "some-tf-state"

			stateStore.SetCall.Returns = []fakes.SetCallReturn{{}, {}, {errors.New("state failed to be set")}}
			err := gcpUp.Execute(commands.GCPUpConfig{}, storage.State{
				IAAS: "gcp",
				GCP: storage.GCP{
					ServiceAccountKey: serviceAccountKey,
					ProjectID:         "some-project-id",
					Zone:              "some-zone",
					Region:            "us-west1",
				},
				EnvID: "bbl-lake-time:stamp",
			})

			Expect(err).To(MatchError("the following errors occurred:\nfailed to apply,\nstate failed to be set"))
			Expect(stateStore.SetCall.CallCount).To(Equal(3))
			Expect(stateStore.SetCall.Receives.State.TFState).To(Equal("some-tf-state"))
		})

		It("returns an error when the state fails to be set after applying terraform", func() {
			stateStore.SetCall.Returns = []fakes.SetCallReturn{{}, {}, {errors.New("state failed to be set")}}

			err := gcpUp.Execute(commands.GCPUpConfig{
				ServiceAccountKeyPath: serviceAccountKeyPath,
				ProjectID:             "some-project-id",
				Zone:                  "some-zone",
				Region:                "us-west1",
			}, storage.State{})
			Expect(err).To(MatchError("state failed to be set"))
		})
	})
})
