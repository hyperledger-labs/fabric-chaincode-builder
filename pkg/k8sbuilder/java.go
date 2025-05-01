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

	v1 "k8s.io/api/core/v1"
)

const javaBuildScript = `/root/chaincode-java/build.sh`

func javaBuildContainer(image string) v1.Container {
	user := int64(7051)
	return v1.Container{
		Name:    "build-java-chaincode",
		Image:   image,
		Command: []string{"/bin/sh"},
		Args:    []string{"-c", javaBuildScript},
		VolumeMounts: []v1.VolumeMount{
			{Name: "chaincode", MountPath: "/chaincode"},
		},
		SecurityContext: &v1.SecurityContext{
			RunAsUser: &user,
		},
	}
}

const javaRunScript = `/root/chaincode-java/start --peerAddress %[1]s`

func javaRunContainer(rmd *runMetadata, image string) v1.Container {
	seccontext := getSecContext()
	return v1.Container{
		Name:            "run-java-chaincode",
		Image:           image,
		ImagePullPolicy: v1.PullAlways,
		Command:         []string{"/bin/sh"},
		Args: []string{
			"-c",
			fmt.Sprintf(javaRunScript, rmd.PeerAddress),
		},
		Env:        rmd.toRunEnv(),
		WorkingDir: "/root/chaincode-java",
		VolumeMounts: []v1.VolumeMount{
			{Name: "chaincode", MountPath: "/chaincode/artifacts", SubPath: "artifacts"},
			{Name: "chaincode", MountPath: "/root/chaincode-java/chaincode", SubPath: "output"},
			{Name: "certs", MountPath: "/certs"},
		},
		SecurityContext: &seccontext,
	}
}
