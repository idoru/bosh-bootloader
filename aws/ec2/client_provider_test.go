package ec2_test

import (
	goaws "github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	awsec2 "github.com/aws/aws-sdk-go/service/ec2"
	"github.com/pivotal-cf-experimental/bosh-bootloader/aws"
	"github.com/pivotal-cf-experimental/bosh-bootloader/aws/ec2"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("ClientProvider", func() {
	var provider ec2.ClientProvider

	BeforeEach(func() {
		provider = ec2.NewClientProvider()
	})

	Describe("Client", func() {
		It("returns a Client with the provided configuration", func() {
			client, err := provider.Client(aws.Config{
				AccessKeyID:      "some-access-key-id",
				SecretAccessKey:  "some-secret-access-key",
				Region:           "some-region",
				EndpointOverride: "some-endpoint-override",
			})
			Expect(err).NotTo(HaveOccurred())

			_, ok := client.(ec2.Client)
			Expect(ok).To(BeTrue())

			ec2Client, ok := client.(*awsec2.EC2)
			Expect(ok).To(BeTrue())

			Expect(ec2Client.Config.Credentials).To(Equal(credentials.NewStaticCredentials("some-access-key-id", "some-secret-access-key", "")))
			Expect(ec2Client.Config.Region).To(Equal(goaws.String("some-region")))
			Expect(ec2Client.Config.Endpoint).To(Equal(goaws.String("some-endpoint-override")))
		})

		Context("failure cases", func() {
			It("returns an error when the credentials are not provided", func() {
				_, err := provider.Client(aws.Config{})
				Expect(err).To(MatchError("aws access key id must be provided"))
			})
		})
	})
})