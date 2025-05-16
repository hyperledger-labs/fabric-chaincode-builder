/*
 * SPDX-License-Identifier: Apache-2.0
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 * 	  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tls_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"time"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/tls"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TLS", func() {
	var generate *tls.Generate

	BeforeEach(func() {
		generate = &tls.Generate{}
	})

	Context("self signed cert", func() {
		var (
			err     error
			params  tls.Param
			privKey *ecdsa.PrivateKey
		)

		BeforeEach(func() {
			privKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
			Expect(err).NotTo(HaveOccurred())

			params = tls.Param{
				Hosts:    []string{"crd-webhook-service.migration.svc"},
				NotAfter: time.Now().Add(time.Hour * 131400), // Certificate is going to be valid for 15 years
			}
		})

		It("generates cert", func() {
			pemCert, err := generate.SelfSignedCert(params, privKey)
			Expect(err).NotTo(HaveOccurred())

			By("setting DNSNames", func() {
				block, _ := pem.Decode(pemCert)
				cert, err := x509.ParseCertificate(block.Bytes)
				Expect(err).NotTo(HaveOccurred())
				Expect(cert.DNSNames).To(ContainElement(params.Hosts[0]))
			})
		})
	})

	Context("cert", func() {
		var template x509.Certificate
		BeforeEach(func() {
			template = x509.Certificate{
				Subject: pkix.Name{
					Organization: []string{"Acme Co"},
				},
				KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
				ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
				BasicConstraintsValid: true,
			}
		})

		It("returns error on invalid signer", func() {
			_, err := generate.Cert(template, template, nil, []string{})
			Expect(err).To(HaveOccurred())
			Expect(err).To(MatchError(ContainSubstring("failed to create certificate")))
		})
	})
})
