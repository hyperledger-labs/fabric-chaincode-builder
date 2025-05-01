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

const goBuildScript = `
set -e
if [ -f "/chaincode/input/src/go.mod" ] && [ -d "/chaincode/input/src/vendor" ]; then
    cd /chaincode/input/src
    echo "GO111MODULE=on go build -v -mod=vendor -o /chaincode/output/chaincode %[1]s"
    GO111MODULE=on go build -v -mod=vendor -o /chaincode/output/chaincode %[1]s
elif [ -f "/chaincode/input/src/go.mod" ]; then
    cd /chaincode/input/src
    echo "GO111MODULE=on go build -v -mod=readonly -o /chaincode/output/chaincode %[1]s"
    GO111MODULE=on go build -v -mod=readonly -o /chaincode/output/chaincode %[1]s
elif [ -f "/chaincode/input/src/%[1]s/go.mod" ] && [ -d "/chaincode/input/src/%[1]s/vendor" ]; then
    cd /chaincode/input/src/%[1]s
    echo "GO111MODULE=on go build -v -mod=vendor -o /chaincode/output/chaincode ."
    GO111MODULE=on go build -v -mod=vendor -o /chaincode/output/chaincode .
elif [ -f "/chaincode/input/src/%[1]s/go.mod" ]; then
    cd /chaincode/input/src/%[1]s
    echo "GO111MODULE=on go build -v -mod=readonly -o /chaincode/output/chaincode ."
    GO111MODULE=on go build -v -mod=readonly -o /chaincode/output/chaincode .
else
    echo "GO111MODULE=off GOPATH=/chaincode/input:$GOPATH go build -v -o /chaincode/output/chaincode %[1]s"
    GO111MODULE=off GOPATH=/chaincode/input:$GOPATH go build -v -o /chaincode/output/chaincode %[1]s
fi
echo Done!
`

func golangBuildContainer(path string, image string) v1.Container {
	user := int64(7051)
	return v1.Container{
		Name:    "build-golang-chaincode",
		Image:   image,
		Command: []string{"/bin/sh"},
		Args:    []string{"-c", fmt.Sprintf(goBuildScript, path)},
		VolumeMounts: []v1.VolumeMount{
			{Name: "chaincode", MountPath: "/chaincode"},
		},
		SecurityContext: &v1.SecurityContext{
			RunAsUser: &user,
		},
	}
}

// Fabric code extracts chaincode output to /usr/local/bin before starting the
// container. In the case of Docker, that extration is done as root before the
// container user gets control.
//
// In Kubernetes, the container runs with the 'chaincode' ID inside the
// container but this user cannot write to /usr/local/bin so we'll run the
// chaincode from /chaincdode/output.

const golangRunScript = `/usr/local/bin/chaincode -peer.address %[1]s`

func golangRunContainer(rmd *runMetadata, image string) v1.Container {

	seccontext := getSecContext()
	return v1.Container{
		Name:            "run-go-chaincode",
		Image:           image,
		ImagePullPolicy: v1.PullAlways,
		Command:         []string{"/bin/sh"},
		Args: []string{
			"-c",
			fmt.Sprintf(golangRunScript, rmd.PeerAddress),
		},
		Env: rmd.toRunEnv(),
		VolumeMounts: []v1.VolumeMount{
			{Name: "chaincode", MountPath: "/chaincode/artifacts", SubPath: "artifacts"},
			{Name: "chaincode", MountPath: "/usr/local/bin", SubPath: "output"},
			{Name: "certs", MountPath: "/certs"},
		},
		SecurityContext: &seccontext,
	}
}
