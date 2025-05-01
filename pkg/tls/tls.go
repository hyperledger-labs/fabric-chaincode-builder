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

package tls

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"time"

	"github.com/pkg/errors"
)

func ConfigTLS(certFile, keyFile []byte) (*tls.Config, error) {
	sCert, err := tls.X509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		MinVersion:   tls.VersionTLS12, // TLS 1.2 recommended, TLS 1.3 (current latest version) encouraged
		Certificates: []tls.Certificate{sCert},
	}, nil
}

type Param struct {
	Hosts    []string
	NotAfter time.Time
}

type Generate struct{}

func (g *Generate) PrivateKey(curve elliptic.Curve) (*ecdsa.PrivateKey, []byte, error) {
	priv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate private key")
	}

	bytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, errors.Wrap(err, "failed to generate private key bytes")
	}
	pemBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: bytes}

	return priv, pem.EncodeToMemory(pemBlock), nil
}

func (g *Generate) SelfSignedCert(params Param, privKey interface{}) ([]byte, error) {
	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate serial number")
	}

	notBefore := time.Now().Add(-5 * time.Second)
	notAfter := params.NotAfter

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"IBM IBP"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	return g.Cert(template, template, privKey, params.Hosts)
}

func (g *Generate) Cert(cert, parent x509.Certificate, privKey interface{}, hosts []string) ([]byte, error) {
	for _, h := range hosts {
		if ip := net.ParseIP(h); ip != nil {
			cert.IPAddresses = append(cert.IPAddresses, ip)
		} else {
			cert.DNSNames = append(cert.DNSNames, h)
		}
	}

	bytes, err := x509.CreateCertificate(rand.Reader, &cert, &parent, publicKey(privKey), privKey)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create certificate")
	}
	pemBlock := &pem.Block{Type: "CERTIFICATE", Bytes: bytes}

	return pem.EncodeToMemory(pemBlock), nil
}

func publicKey(priv interface{}) interface{} {
	switch k := priv.(type) {
	case *rsa.PrivateKey:
		return &k.PublicKey
	case *ecdsa.PrivateKey:
		return &k.PublicKey
	default:
		return nil
	}
}
