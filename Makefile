#
# Copyright contributors to the Hyperledger Fabric Operator project
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


IMAGE ?= ghcr.io/hyperledger-labs/chaincode-builder
TAG ?= $(shell git rev-parse --short HEAD)
ARCH ?= $(shell go env GOARCH)
BRANCH ?= $(shell git branch --show-current)
DOCKER_IMAGE_REPO ?= ghcr.io
REGISTRY ?= $(DOCKER_IMAGE_REPO)/ibp-golang
GO_VER ?= 1.24.3
BUILD_DATE = $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
GOOS ?= $(shell go env GOOS)


BUILD_ARGS=--build-arg ARCH=$(ARCH)
BUILD_ARGS+=--build-arg REGISTRY=$(REGISTRY)
BUILD_ARGS+=--build-arg BUILD_ID=$(TAG)
BUILD_ARGS+=--build-arg BUILD_DATE=$(BUILD_DATE)
BUILD_ARGS+=--build-arg GO_VER=$(GO_VER)


.PHONY: build login

int-tests:
	@ginkgo -v ./integration

build:
	GOOS=$(GOOS) GOARCH=$(ARCH) go build -o build/chaincode-builder ./cmd/ibp-builder
	GOOS=$(GOOS) GOARCH=$(ARCH) go build -o build/chaincode-builder-client ./cmd/ibp-builder-client

image: ## Builds a x86 based image
	@go mod vendor
	docker build --rm . -f Dockerfile $(BUILD_ARGS) -t $(IMAGE):$(TAG)-$(ARCH)
	docker tag $(IMAGE):$(TAG)-$(ARCH) $(IMAGE):latest-$(ARCH)

image-nologin:
	@go mod vendor
	docker build --rm . -f Dockerfile $(BUILD_ARGS) -t $(IMAGE):$(TAG)-$(ARCH)
	docker tag $(IMAGE):$(TAG)-$(ARCH) $(IMAGE):latest-$(ARCH)
image-push: 
	docker push $(IMAGE):$(TAG)-$(ARCH)

unit-tests:
	go test `go list ./... | grep -v integration`

gosec:
	@scripts/go-sec.sh

checks: license
	@scripts/checks

.PHONY: license
license:
	chmod +x @scripts/check-license.sh
