// Copyright 2023 DoltHub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math"
	"math/big"
	"time"
)

type TLSBundle struct {
	// The PEM-encoded private key for the leaf certificate.
	Key string
	// The PEM-encoded certificate chain, currently is LEAF\nINTERMEDIATE.
	Chain string
	// The PEM-encoded root certificate.
	Root string
}

// Builds an ephemeral TLS bundle for the test deployment.  Currently, the
// structure is a single trusted root, a single intermediate certificate and a
// single leaf certificate. All RSA-4096 keys. The leaf certificate is created
// with the following SANs:
//
// * dolt-{0,1,...}.dolt-internal.dolt.svc.cluster.local
// * dolt-ro.dolt.svc.cluster.local
// * dolt-rw.dolt.svc.cluster.local
// * dolt.dolt.svc.cluster.local
func NewTLSBundle(namespace string, config StatefulSetConfig) (TLSBundle, error) {
	rootkey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return TLSBundle{}, err
	}
	intkey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return TLSBundle{}, err
	}
	leafkey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return TLSBundle{}, err
	}
	leafder := x509.MarshalPKCS1PrivateKey(leafkey)
	leafpem := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: leafder,
	})

	rootSerial, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return TLSBundle{}, err
	}
	rootTemplate := &x509.Certificate{
		IsCA:      true,
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(10 * time.Hour),
		KeyUsage:  x509.KeyUsageCertSign,
		Subject: pkix.Name{
			CommonName: "e2e Test Root",
		},
		SerialNumber: rootSerial,
	}
	rootbytes, err := x509.CreateCertificate(rand.Reader, rootTemplate, rootTemplate, rootkey.Public(), rootkey)
	if err != nil {
		return TLSBundle{}, err
	}
	rootpem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rootbytes,
	})
	root, err := x509.ParseCertificate(rootbytes)
	if err != nil {
		return TLSBundle{}, err
	}

	intSerial, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return TLSBundle{}, err
	}
	intTemplate := &x509.Certificate{
		IsCA:      true,
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(2 * time.Hour),
		KeyUsage:  x509.KeyUsageCertSign,
		Subject: pkix.Name{
			CommonName: "e2e Test Intermediate",
		},
		SerialNumber: intSerial,
	}
	intbytes, err := x509.CreateCertificate(rand.Reader, intTemplate, root, intkey.Public(), rootkey)
	if err != nil {
		return TLSBundle{}, err
	}
	intcert, err := x509.ParseCertificate(intbytes)
	if err != nil {
		return TLSBundle{}, err
	}

	namespaceDNS := fmt.Sprintf(".%s.svc.cluster.local", namespace)
	leafDNSNames := []string{
		"dolt-ro" + namespaceDNS,
		"dolt-rw" + namespaceDNS,
		"dolt" + namespaceDNS,
	}
	for i := int32(0); i < config.NumReplicas; i++ {
		leafDNSNames = append(leafDNSNames, fmt.Sprintf("dolt-%d%s", i, namespaceDNS))
	}

	leafSerial, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return TLSBundle{}, err
	}
	leafTemplate := &x509.Certificate{
		NotBefore: time.Now().Add(-1 * time.Hour),
		NotAfter:  time.Now().Add(2 * time.Hour),
		KeyUsage:  x509.KeyUsageKeyAgreement | x509.KeyUsageKeyEncipherment,
		Subject: pkix.Name{
			CommonName: "e2e Test Leaf",
		},
		ExtKeyUsage: []x509.ExtKeyUsage{
			x509.ExtKeyUsageServerAuth,
		},
		DNSNames:     leafDNSNames,
		SerialNumber: leafSerial,
	}
	leafbytes, err := x509.CreateCertificate(rand.Reader, leafTemplate, intcert, leafkey.Public(), intkey)
	if err != nil {
		return TLSBundle{}, err
	}

	var chain bytes.Buffer
	err = pem.Encode(&chain, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: leafbytes,
	})
	err = pem.Encode(&chain, &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: intbytes,
	})
	if err != nil {
		return TLSBundle{}, err
	}

	return TLSBundle{
		Key:   string(leafpem),
		Root:  string(rootpem),
		Chain: chain.String(),
	}, nil
}
