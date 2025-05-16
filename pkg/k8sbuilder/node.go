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

const nodeBuildScript = `
set -e
if [ -x /chaincode/build.sh ]; then
	/chaincode/build.sh
else
	cp -R /chaincode/input/src/. /chaincode/output && cd /chaincode/output && npm install --production
fi
`

func nodeBuildContainer(image string) v1.Container {
	user := int64(7051)
	return v1.Container{
		Name:    "build-node-chaincode",
		Image:   image,
		Command: []string{"/bin/sh"},
		Args:    []string{"-c", nodeBuildScript},
		VolumeMounts: []v1.VolumeMount{
			{Name: "chaincode", MountPath: "/chaincode"},
		},
		SecurityContext: &v1.SecurityContext{
			RunAsUser: &user,
		},
	}
}

const nodeStartScript = `
set -e
if [ -x /chaincode/start.sh ]; then
	/chaincode/start.sh --peer.address %[1]s
else
	cd /usr/local/src
	npm start -- --peer.address %[1]s
fi
`

func nodeRunContainer(rmd *runMetadata, image string) v1.Container {

	seccontext := getSecContext()
	return v1.Container{
		Name:            "run-node-chaincode",
		Image:           image,
		ImagePullPolicy: v1.PullAlways,
		Command:         []string{"/bin/sh"},
		Args: []string{
			"-c",
			fmt.Sprintf(nodeStartScript, rmd.PeerAddress),
		},
		Env: rmd.toRunEnv(),
		VolumeMounts: []v1.VolumeMount{
			{Name: "chaincode", MountPath: "/chaincode/artifacts", SubPath: "artifacts"},
			{Name: "chaincode", MountPath: "/usr/local/src", SubPath: "output"},
			{Name: "certs", MountPath: "/certs"},
		},
		SecurityContext: &seccontext,
	}
}
