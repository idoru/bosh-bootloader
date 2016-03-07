package boshinit_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/pivotal-cf-experimental/bosh-bootloader/boshinit"
)

var _ = Describe("JobPropertiesManifestBuilder", func() {
	var jobPropertiesManifestBuilder boshinit.JobPropertiesManifestBuilder

	BeforeEach(func() {
		jobPropertiesManifestBuilder = boshinit.NewJobPropertiesManifestBuilder()
	})

	Describe("NATS", func() {
		It("returns job properties for NATS", func() {
			nats := jobPropertiesManifestBuilder.NATS()
			Expect(nats).To(Equal(
				boshinit.NATSJobProperties{
					Address:  "127.0.0.1",
					User:     "nats",
					Password: "nats-password",
				}))
		})
	})

	Describe("Redis", func() {
		It("returns job properties for Redis", func() {
			redis := jobPropertiesManifestBuilder.Redis()
			Expect(redis).To(Equal(
				boshinit.RedisJobProperties{
					ListenAddress: "127.0.0.1",
					Address:       "127.0.0.1",
					Password:      "redis-password",
				}))
		})
	})

	Describe("Postgres", func() {
		It("returns job properties for Postgres", func() {
			postgres := jobPropertiesManifestBuilder.Postgres()
			Expect(postgres).To(Equal(boshinit.PostgresJobProperties{
				ListenAddress: "127.0.0.1",
				Host:          "127.0.0.1",
				User:          "postgres",
				Password:      "postgres-password",
				Database:      "bosh",
				Adapter:       "postgres",
			}))
		})
	})

	Describe("Registry", func() {
		It("returns job properties for Registry", func() {
			registry := jobPropertiesManifestBuilder.Registry()
			Expect(registry).To(Equal(boshinit.RegistryJobProperties{
				Address:  "10.0.0.6",
				Host:     "10.0.0.6",
				Username: "admin",
				Password: "admin",
				Port:     25777,
				DB: boshinit.DBProperties{
					ListenAddress: "127.0.0.1",
					Host:          "127.0.0.1",
					User:          "postgres",
					Password:      "postgres-password",
					Database:      "bosh",
					Adapter:       "postgres",
				},
				HTTP: boshinit.HTTPProperties{
					User:     "admin",
					Password: "admin",
					Port:     25777,
				},
			}))
		})
	})

	Describe("Blobstore", func() {
		It("returns job properties for Blobstore", func() {
			blobstore := jobPropertiesManifestBuilder.Blobstore()
			Expect(blobstore).To(Equal(boshinit.BlobstoreJobProperties{
				Address:  "10.0.0.6",
				Port:     25250,
				Provider: "dav",
				Director: boshinit.Credentials{
					User:     "director",
					Password: "director-password",
				},
				Agent: boshinit.Credentials{
					User:     "agent",
					Password: "agent-password",
				},
			}))
		})
	})

	Describe("Director", func() {
		It("returns job properties for Director", func() {
			director := jobPropertiesManifestBuilder.Director()
			Expect(director).To(Equal(boshinit.DirectorJobProperties{
				Address:    "127.0.0.1",
				Name:       "my-bosh",
				CPIJob:     "aws_cpi",
				MaxThreads: 10,
				DB: boshinit.DBProperties{
					ListenAddress: "127.0.0.1",
					Host:          "127.0.0.1",
					User:          "postgres",
					Password:      "postgres-password",
					Database:      "bosh",
					Adapter:       "postgres",
				},
				UserManagement: boshinit.UserManagementProperties{
					Provider: "local",
					Local: boshinit.LocalProperties{
						Users: []boshinit.UserProperties{
							{
								Name:     "admin",
								Password: "admin",
							},
							{
								Name:     "hm",
								Password: "hm-password",
							},
						},
					},
				},
			}))
		})
	})

	Describe("HM", func() {
		It("returns job properties for HM", func() {
			hm := jobPropertiesManifestBuilder.HM()
			Expect(hm).To(Equal(boshinit.HMJobProperties{
				DirectorAccount: boshinit.Credentials{
					User:     "hm",
					Password: "hm-password",
				},
				ResurrectorEnabled: true,
			}))
		})
	})

	Describe("AWS", func() {
		It("returns job properties for AWS", func() {
			aws := jobPropertiesManifestBuilder.AWS()
			Expect(aws).To(Equal(boshinit.AWSProperties{
				AccessKeyId:           "ACCESS-KEY-ID",
				SecretAccessKey:       "SECRET-ACCESS-KEY",
				DefaultKeyName:        "bosh",
				DefaultSecurityGroups: []string{"bosh"},
				Region:                "REGION",
			}))
		})
	})

	Describe("Agent", func() {
		It("returns job properties for Agent", func() {
			agent := jobPropertiesManifestBuilder.Agent()
			Expect(agent).To(Equal(boshinit.AgentProperties{
				MBus: "nats://nats:nats-password@10.0.0.6:4222",
			}))
		})
	})

	Describe("NTP", func() {
		It("returns job properties for NTP", func() {
			agent := jobPropertiesManifestBuilder.NTP()
			Expect(agent).To(ConsistOf(
				[]string{"0.pool.ntp.org", "1.pool.ntp.org"},
			))
		})
	})
})