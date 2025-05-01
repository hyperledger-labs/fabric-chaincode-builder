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
	"testing"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
)

func TestJavaBuildContainer(t *testing.T) {
	user := int64(7051)
	gt := NewGomegaWithT(t)
	gt.Expect(javaBuildContainer("hyperledger/fabric-javaenv:2.0")).To(Equal(v1.Container{
		Name:    "build-java-chaincode",
		Image:   "hyperledger/fabric-javaenv:2.0",
		Command: []string{"/bin/sh"},
		Args:    []string{"-c", "/root/chaincode-java/build.sh"},
		VolumeMounts: []v1.VolumeMount{
			{Name: "chaincode", MountPath: "/chaincode"},
		},
		SecurityContext: &v1.SecurityContext{
			RunAsUser: &user,
		},
	}))
}

func TestJavaRunContainer(t *testing.T) {
	seccontext := getSecContext()
	gt := NewGomegaWithT(t)
	rmd := &runMetadata{PeerAddress: "peer-address:9999"}
	gt.Expect(javaRunContainer(rmd, "hyperledger/fabric-javaenv:2.0")).To(Equal(v1.Container{
		Name:            "run-java-chaincode",
		Image:           "hyperledger/fabric-javaenv:2.0",
		ImagePullPolicy: v1.PullAlways,
		Command:         []string{"/bin/sh"},
		Args:            []string{"-c", "/root/chaincode-java/start --peerAddress peer-address:9999"},
		Env:             rmd.toRunEnv(),
		WorkingDir:      "/root/chaincode-java",
		VolumeMounts: []v1.VolumeMount{
			{Name: "chaincode", MountPath: "/chaincode/artifacts", SubPath: "artifacts"},
			{Name: "chaincode", MountPath: "/root/chaincode-java/chaincode", SubPath: "output"},
			{Name: "certs", MountPath: "/certs"},
		},
		SecurityContext: &seccontext,
	}))
}
