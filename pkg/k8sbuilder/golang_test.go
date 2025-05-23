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

package k8sbuilder

import (
	"fmt"
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

func TestGolangBuildContainer(t *testing.T) {
	user := int64(7051)
	gt := NewGomegaWithT(t)
	gt.Expect(golangBuildContainer("metadata-path", "hyperledger/fabric-ccenv:2.0")).To(Equal(v1.Container{
		Name:    "build-golang-chaincode",
		Image:   "hyperledger/fabric-ccenv:2.0",
		Command: []string{"/bin/sh"},
		Args:    []string{"-c", fmt.Sprintf(goBuildScript, "metadata-path")},
		VolumeMounts: []v1.VolumeMount{
			{Name: "chaincode", MountPath: "/chaincode"},
		},
		SecurityContext: &v1.SecurityContext{
			RunAsUser: &user,
		},
	}))
}

func TestGolangRunContainer(t *testing.T) {
	seccontext := getSecContext()
	gt := NewGomegaWithT(t)
	rmd := &runMetadata{
		PeerAddress: "peer-address:9999",
	}
	gt.Expect(golangRunContainer(rmd, "dockerhub.io/fabric-chaincode-baseimage/goenv:latest-amd64")).To(Equal(v1.Container{
		Name:            "run-go-chaincode",
		Image:           "dockerhub.io/fabric-chaincode-baseimage/goenv:latest-amd64",
		ImagePullPolicy: v1.PullAlways,
		Command:         []string{"/bin/sh"},
		Args: []string{
			"-c",
			"/usr/local/bin/chaincode -peer.address peer-address:9999",
		},
		Env: rmd.toRunEnv(),
		VolumeMounts: []v1.VolumeMount{
			{Name: "chaincode", MountPath: "/chaincode/artifacts", SubPath: "artifacts"},
			{Name: "chaincode", MountPath: "/usr/local/bin", SubPath: "output"},
			{Name: "certs", MountPath: "/certs"},
		},
		SecurityContext: &seccontext,
	}))
}
