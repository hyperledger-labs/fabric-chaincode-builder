#
# Copyright contributors to the Hyperledger Fabric Operations Console project
#
# SPDX-License-Identifier: Apache-2.0
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at:
#
# 	  http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
#
ARG ARCH
ARG GO_VER

FROM registry.access.redhat.com/ubi9/ubi-minimal as builder
ARG GO_VER
ARG ARCH
# gcc required for cgo

RUN microdnf install -y make gcc tar gzip gcc-c++ && microdnf clean all
RUN echo "GO_VER=${GO_VER}" && echo "ARCH=${ARCH}"
RUN curl -sL https://go.dev/dl/go${GO_VER}.linux-${ARCH}.tar.gz | tar zxf - -C /usr/local
ENV PATH="/usr/local/go/bin:$PATH"
COPY . /go/src/github.ibm.com/fabric/fabric-chaincode-builder
WORKDIR /go/src/github.ibm.com/fabric/fabric-chaincode-builder
RUN GOOS=linux GOARCH=$(go env GOARCH) go build -o build/fabric-chaincode-builder ./cmd/fabric-builder

FROM registry.access.redhat.com/ubi9/ubi-minimal
ARG IBP_VER
ARG BUILD_ID
ARG BUILD_DATE


ENV BUILDER=/usr/local/bin/fabric-chaincode-builder \
    USER_UID=1001 \
    USER_NAME=fabric-chaincode-builder \
    CLIENT_TIMEOUT=5m \
    FILE_SERVER_LISTEN_IP=0.0.0.0 \
    FILE_SERVER_LISTEN_PORT=22222 \
    SHARED_VOLUME_PATH=/data \
    SIDECAR_LISTEN_ADDRESS=0.0.0.0:11111

RUN microdnf update -y
RUN microdnf install -y shadow-utils iputils
RUN groupadd -g 7051 ibp-user \
    && useradd -u 7051 -g ibp-user -s /bin/bash ibp-user \
    && microdnf remove -y shadow-utils \
    && microdnf clean -y all;

COPY --from=builder /go/src/github.ibm.com/ibp/chaincode-builder/build/fabric-chaincode-builder ${BUILDER}
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

USER ibp-user

ENTRYPOINT ["docker-entrypoint.sh"]
CMD [ "sh", "-c", "fabric-chaincode-builder --kubeconfig \"${KUBECONFIG}\" --kubeNamespace \"${KUBE_NAMESPACE}\" --clientTimeout \"${CLIENT_TIMEOUT}\" --peerID \"${PEER_ID}\" --sharedVolumePath \"${SHARED_VOLUME_PATH}\" --fileServerListenAddress \"${FILE_SERVER_LISTEN_IP}:${FILE_SERVER_LISTEN_PORT}\" --sidecarListenAddress \"${SIDECAR_LISTEN_ADDRESS}\" --fileServerBaseURL \"${FILE_SERVER_BASE_URL}\"" ]
