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
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/k8sbuilder/fakes"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/sidecar"

	. "github.com/onsi/gomega"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1fake "k8s.io/client-go/kubernetes/typed/core/v1/fake"
	restclient "k8s.io/client-go/rest"
	fakerest "k8s.io/client-go/rest/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestBuild(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "build")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	pmdTar, err := os.Create(filepath.Join(tempDir, "package-metadata.tar"))
	gt.Expect(err).NotTo(HaveOccurred())
	writeTar(t, pmdTar, map[string]string{
		"metadata.json": `{"type":"golang"}`,
	})
	badType, err := os.Create(filepath.Join(tempDir, "bad-type-metadata.tar"))
	gt.Expect(err).NotTo(HaveOccurred())
	writeTar(t, badType, map[string]string{
		"metadata.json": `{"type":"unknown"}`,
	})

	tests := []struct {
		name         string
		metadataPath string
		podPhase     v1.PodPhase
		errMatcher   interface{}
	}{
		{
			name:         "pod-succeeded",
			metadataPath: "package-metadata.tar",
			podPhase:     v1.PodSucceeded,
		},
		{
			name:         "pod-failed",
			metadataPath: "package-metadata.tar",
			podPhase:     v1.PodFailed,
			errMatcher:   Equal("rpc error: code = Unknown desc = build failed: pod terminated: pod status is failed"),
		},
		{
			name:         "bad-metadata",
			metadataPath: "missing-metadata.tar",
			podPhase:     v1.PodSucceeded,
			errMatcher:   ContainSubstring("missing-metadata.tar: no such file or directory"),
		},
		{
			name:         "bad-chaincode-type",
			metadataPath: "bad-type-metadata.tar",
			podPhase:     v1.PodSucceeded,
			errMatcher:   Equal("rpc error: code = InvalidArgument desc = unable to generate build container: unsupported chaincode type: unknown"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := NewGomegaWithT(t)
			fakeClientset := k8sfake.NewSimpleClientset()
			fakePodWatcher := watch.NewFakeWithChanSize(2, false)
			fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakePodWatcher, nil))
			fakePodsClient := newFakePods(fakeClientset.CoreV1().Pods("test-ns"))
			os.Setenv("PEER_POD_NAME", "peer-test-1234")
			os.Setenv("PEER_POD_UID", "1234")
			server := &Server{
				PodClient:         fakePodsClient,
				SharedPath:        tempDir,
				FSBaseURL:         "http://example.com/base-url/",
				FileTransferImage: "utils/filetransfer:0.9",
				PeerID:            "ut-peer-id",
				Auth: &BasicAuth{
					Username: "testuser",
					Password: "testpassword",
				},
				TLSCertString: "test cert",
			}

			req := &sidecar.BuildRequest{
				SourcePath:   "chaincode-source.tar",
				MetadataPath: tt.metadataPath,
				OutputPath:   "chaincode-output.tar",
			}
			fakeBuildStream := &fakes.BuildStream{}
			timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			fakeBuildStream.ContextReturns(timeoutCtx)
			fakePodWatcher.Action(watch.Modified, &v1.Pod{
				Status: v1.PodStatus{Phase: tt.podPhase},
			})

			err = server.Build(req, fakeBuildStream)
			if tt.errMatcher != nil {
				gt.Expect(err).To(MatchError(tt.errMatcher))
				return
			}
			gt.Expect(err).NotTo(HaveOccurred())
			fakePodsClient.Wg.Wait()
			os.Clearenv()
		})
	}
}

func TestSerializedBuildStreamDelegation(t *testing.T) {
	gt := NewGomegaWithT(t)

	fakeBuildStream := &fakes.BuildStream{}
	stream := &serializedBuildStream{Builder_BuildServer: fakeBuildStream}
	stream.Send(&sidecar.BuildResponse{})
	gt.Expect(fakeBuildStream.SendCallCount()).To(Equal(1))
	gt.Expect(fakeBuildStream.SendArgsForCall(0)).To(Equal(&sidecar.BuildResponse{}))

	fakeBuildStream = &fakes.BuildStream{}
	stream = &serializedBuildStream{Builder_BuildServer: fakeBuildStream}
	stream.Log("log-message")
	gt.Expect(fakeBuildStream.SendCallCount()).To(Equal(1))
	gt.Expect(fakeBuildStream.SendArgsForCall(0)).To(Equal(&sidecar.BuildResponse{
		Message: &sidecar.BuildResponse_StandardError{StandardError: "log-message"},
	}))
}

func TestRun(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "run")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	ccmdTar, err := os.Create(filepath.Join(tempDir, "chaincode-metadata.tar"))
	gt.Expect(err).NotTo(HaveOccurred())
	writeTar(t, ccmdTar, map[string]string{
		"metadata.json": `{"type":"golang"}`,
	})
	badType, err := os.Create(filepath.Join(tempDir, "bad-type-metadata.tar"))
	gt.Expect(err).NotTo(HaveOccurred())
	writeTar(t, badType, map[string]string{
		"metadata.json": `{"type":"unknown"}`,
	})
	err = ioutil.WriteFile(filepath.Join(tempDir, "chaincode.json"), []byte("{}"), 0644)
	gt.Expect(err).NotTo(HaveOccurred())

	tests := []struct {
		name              string
		chaincodeMetadata string
		runMetadata       string
		podPhase          v1.PodPhase
		errMatcher        interface{}
	}{
		{
			name:              "pod-succeeded",
			chaincodeMetadata: "chaincode-metadata.tar",
			runMetadata:       "chaincode.json",
			podPhase:          v1.PodSucceeded,
		},
		{
			name:              "pod-failed",
			chaincodeMetadata: "chaincode-metadata.tar",
			runMetadata:       "chaincode.json",
			podPhase:          v1.PodFailed,
			errMatcher:        Equal("rpc error: code = Unknown desc = run failed: pod terminated: pod status is failed"),
		},
		{
			name:              "bad-run-metadata",
			chaincodeMetadata: "chaincode-metadata.tar",
			runMetadata:       "missing.json",
			podPhase:          v1.PodSucceeded,
			errMatcher:        ContainSubstring("missing.json: no such file or directory"),
		},
		{
			name:              "bad-chaincode-type",
			chaincodeMetadata: "bad-type-metadata.tar",
			runMetadata:       "chaincode.json",
			podPhase:          v1.PodSucceeded,
			errMatcher:        Equal("rpc error: code = InvalidArgument desc = unable to generate run container: unsupported chaincode type: unknown"),
		},
		{
			name:              "bad-metadata",
			chaincodeMetadata: "missing-metadata.tar",
			runMetadata:       "chaincode.json",
			podPhase:          v1.PodSucceeded,
			errMatcher:        ContainSubstring("missing-metadata.tar: no such file or directory"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gt := NewGomegaWithT(t)
			fakeClientset := k8sfake.NewSimpleClientset()
			fakePodWatcher := watch.NewFakeWithChanSize(1, false)
			fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakePodWatcher, nil))
			fakePodsClient := newFakePods(fakeClientset.CoreV1().Pods("test-ns"))

			fakePodWatcher.Action(watch.Modified, &v1.Pod{
				Status: v1.PodStatus{Phase: tt.podPhase},
			})

			server := &Server{
				SecretClient:      fakeClientset.CoreV1().Secrets("test-ns"),
				PodClient:         fakePodsClient,
				SharedPath:        tempDir,
				FSBaseURL:         "http://example.com/base-url/",
				FileTransferImage: "utils/filetransfer:0.9",
				PeerID:            "ut-peer-id",
				Auth: &BasicAuth{
					Username: "testuser",
					Password: "testpassword",
				},
				TLSCertString: "test cert",
			}

			req := &sidecar.RunRequest{
				ChaincodePath: "chaincode-output.tar",
				MetadataPath:  tt.chaincodeMetadata,
				ArtifactsPath: tt.runMetadata,
			}

			fakeRunStream := &fakes.RunStream{}
			timeoutCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			fakeRunStream.ContextReturns(timeoutCtx)
			os.Setenv("PEER_POD_NAME", "peer-test-1234")
			os.Setenv("PEER_POD_UID", "1234")

			err = server.Run(req, fakeRunStream)
			if tt.errMatcher != nil {
				gt.Expect(err).To(MatchError(tt.errMatcher))
				return
			}
			gt.Expect(err).NotTo(HaveOccurred())
			os.Clearenv()
		})
	}
}

func TestSerializedRunStreamDelegation(t *testing.T) {
	gt := NewGomegaWithT(t)

	fakeRunStream := &fakes.RunStream{}
	stream := &serializedRunStream{Runner_RunServer: fakeRunStream}
	stream.Send(&sidecar.RunResponse{})
	gt.Expect(fakeRunStream.SendCallCount()).To(Equal(1))
	gt.Expect(fakeRunStream.SendArgsForCall(0)).To(Equal(&sidecar.RunResponse{}))

	fakeRunStream = &fakes.RunStream{}
	stream = &serializedRunStream{Runner_RunServer: fakeRunStream}
	stream.Log("log-message")
	gt.Expect(fakeRunStream.SendCallCount()).To(Equal(1))
	gt.Expect(fakeRunStream.SendArgsForCall(0)).To(Equal(&sidecar.RunResponse{
		Message: &sidecar.RunResponse_StandardError{StandardError: "log-message"},
	}))
}

func TestBuildPodDefinition(t *testing.T) {
	gt := NewGomegaWithT(t)

	server := &Server{
		FileTransferImage: "file-transfer-image",
		FSBaseURL:         "scheme://host/baseurl/",
		PeerID:            "peer-id",
		ImagePullSecrets:  []string{"regcred"},
		Auth: &BasicAuth{
			Username: "testuser",
			Password: "testpassword",
		},
		TLSCertString: "test cert",
	}

	pod := server.buildPodDefinition(v1.Container{}, "source-path", "output-path", "peer-test-1234", "1234")
	gt.Expect(pod.Name).To(HavePrefix("chaincode-build-"))

	pod.Name = ""
	user := int64(7051)
	boolTrue := true

	gt.Expect(pod).To(Equal(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"peer-id": "peer-id",
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion:         "v1",
					Kind:               "Pod",
					Name:               "peer-test-1234",
					UID:                "1234",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				},
			},
		},
		Spec: v1.PodSpec{
			ImagePullSecrets: []v1.LocalObjectReference{
				v1.LocalObjectReference{
					Name: "regcred",
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: "chaincode",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
			},
			InitContainers: []v1.Container{
				{
					Name:            "setup-chaincode-volume",
					Image:           "file-transfer-image",
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{"/bin/sh"},
					Args: []string{
						"-c",
						"mkdir -p /chaincode/input /chaincode/output /chaincode/certs && chmod 777 /chaincode/input /chaincode/output /chaincode/certs && echo -e ${TLS_CERT} > /chaincode/certs/cacert.pem",
					},
					Env: []v1.EnvVar{
						v1.EnvVar{
							Name:  "TLS_CERT",
							Value: server.TLSCertString,
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{Name: "chaincode", MountPath: "/chaincode"},
					},
				},
				{
					Name:            "download-chaincode-source",
					Image:           "file-transfer-image",
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{"/bin/sh"},
					Args: []string{
						"-c",
						"curl -u ${USERNAME}:${PASSWORD} --cacert /chaincode/certs/cacert.pem -s -o- -L 'scheme://host/baseurl/source-path' | tar -C /chaincode/input -xvf - && chmod -R 777 /chaincode/input",
					},
					Env: []v1.EnvVar{
						v1.EnvVar{
							Name:  "USERNAME",
							Value: server.Auth.Username,
						},
						v1.EnvVar{
							Name:  "PASSWORD",
							Value: server.Auth.Password,
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{Name: "chaincode", MountPath: "/chaincode"},
					},
				},
				v1.Container{},
			},
			Containers: []v1.Container{
				{
					Name:            "upload-chaincode-output",
					Image:           "file-transfer-image",
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{"/bin/sh"},
					Args: []string{
						"-c",
						"cd /chaincode/output && tar cvf /chaincode/output.tar $(ls -A) && curl -u ${USERNAME}:${PASSWORD} --cacert /chaincode/certs/cacert.pem -s --upload-file /chaincode/output.tar 'scheme://host/baseurl/output-path'",
					},
					Env: []v1.EnvVar{
						v1.EnvVar{
							Name:  "USERNAME",
							Value: server.Auth.Username,
						},
						v1.EnvVar{
							Name:  "PASSWORD",
							Value: server.Auth.Password,
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{Name: "chaincode", MountPath: "/chaincode"},
					},
					SecurityContext: &v1.SecurityContext{
						RunAsUser: &user,
					},
				},
			},
		},
	}))
}

func TestBuildContainerDefinition(t *testing.T) {
	tests := []struct {
		typ       string
		path      string
		container v1.Container
		err       error
	}{
		{typ: "golang", path: "the-path", container: golangBuildContainer("the-path", "builder-image"), err: nil},
		{typ: "GOLANG", path: "the-path", container: golangBuildContainer("the-path", "builder-image"), err: nil},
		{typ: "java", container: javaBuildContainer("java-image"), err: nil},
		{typ: "JAVA", container: javaBuildContainer("java-image"), err: nil},
		{typ: "node", container: nodeBuildContainer("builder-image"), err: nil},
		{typ: "NODE", container: nodeBuildContainer("builder-image"), err: nil},
		{typ: "unknown", container: v1.Container{}, err: errors.New("unsupported chaincode type: unknown")},
	}

	b := Server{
		BuilderImage: "builder-image",
		JavaEnvImage: "java-image",
		NodeEnvImage: "node-image",
	}
	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			gt := NewGomegaWithT(t)

			md := &chaincodeMetadata{Type: tt.typ, Path: tt.path}
			container, err := b.buildContainerDefinition(md)
			if tt.err != nil {
				gt.Expect(err).To(Equal(tt.err))
				return
			}
			gt.Expect(err).NotTo(HaveOccurred())
			gt.Expect(container).To(Equal(tt.container))
		})
	}
}

func TestRunPodDefinition(t *testing.T) {
	gt := NewGomegaWithT(t)

	server := &Server{
		FileTransferImage: "file-transfer-image",
		FSBaseURL:         "scheme://host/baseurl/",
		PeerID:            "peer-id",
		ImagePullSecrets:  []string{"regcred"},
		Auth: &BasicAuth{
			Username: "testuser",
			Password: "testpassword",
		},
		TLSCertString: "test cert",
	}

	rmd := &runMetadata{ChaincodeID: "chaincode-id"}
	pod := server.runPodDefinition(v1.Container{}, rmd, "chaincode-path", "1234", "peer-test-1234", "1234")
	gt.Expect(pod.Name).To(HavePrefix("chaincode-execution-"))

	pod.Name = ""
	boolTrue := true

	affinity := &v1.Affinity{
		PodAffinity: &v1.PodAffinity{
			RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
				{
					LabelSelector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "peer-id",
						},
					},
					TopologyKey: "topology.kubernetes.io/zone",
				},
			},
		},
	}

	pod.Spec.Affinity = affinity
	gt.Expect(pod).To(Equal(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"peer-id":      "peer-id",
				"chaincode-id": "chaincode-id",
				"app":          "peer-id",
			},
			OwnerReferences: []metav1.OwnerReference{
				metav1.OwnerReference{
					APIVersion:         "v1",
					Kind:               "Pod",
					Name:               "peer-test-1234",
					UID:                "1234",
					Controller:         &boolTrue,
					BlockOwnerDeletion: &boolTrue,
				},
			},
		},
		Spec: v1.PodSpec{
			ImagePullSecrets: []v1.LocalObjectReference{
				v1.LocalObjectReference{
					Name: "regcred",
				},
			},
			RestartPolicy: v1.RestartPolicyNever,
			Volumes: []v1.Volume{
				{
					Name: "chaincode",
					VolumeSource: v1.VolumeSource{
						EmptyDir: &v1.EmptyDirVolumeSource{},
					},
				},
				{
					Name: "certs",
					VolumeSource: v1.VolumeSource{
						Secret: &v1.SecretVolumeSource{
							SecretName: "certs-1234",
						},
					},
				},
			},
			InitContainers: []v1.Container{
				{
					Name:            "download-chaincode-output",
					Image:           "file-transfer-image",
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{"/bin/sh"},
					Args: []string{
						"-c",
						"mkdir -p /chaincode/output /chaincode/certs && chmod -R 777 /chaincode/output /chaincode/certs && echo -e ${TLS_CERT} > /chaincode/certs/cacert.pem && curl -u ${USERNAME}:${PASSWORD} --cacert /chaincode/certs/cacert.pem -s -o- -L 'scheme://host/baseurl/chaincode-path' | tar -C /chaincode/output -xvf -",
					},
					Env: []v1.EnvVar{
						v1.EnvVar{
							Name:  "USERNAME",
							Value: server.Auth.Username,
						},
						v1.EnvVar{
							Name:  "PASSWORD",
							Value: server.Auth.Password,
						},
						v1.EnvVar{
							Name:  "TLS_CERT",
							Value: server.TLSCertString,
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{Name: "chaincode", MountPath: "/chaincode"},
					},
				},
			},
			Containers: []v1.Container{{}},
			Affinity:   affinity,
		},
	}))
}

func TestRunContainerDefinition(t *testing.T) {
	rmd := &runMetadata{
		ChaincodeID: "chaincode-id",
		ClientCert:  "client-cert",
		ClientKey:   "client-key",
		RootCert:    "root-cert",
		PeerAddress: "peer-address",
	}

	tests := []struct {
		typ       string
		container v1.Container
		err       error
	}{
		{typ: "golang", container: golangRunContainer(rmd, "go-image"), err: nil},
		{typ: "GOLANG", container: golangRunContainer(rmd, "go-image"), err: nil},
		{typ: "java", container: javaRunContainer(rmd, "java-image"), err: nil},
		{typ: "JAVA", container: javaRunContainer(rmd, "java-image"), err: nil},
		{typ: "node", container: nodeRunContainer(rmd, "node-image"), err: nil},
		{typ: "NODE", container: nodeRunContainer(rmd, "node-image"), err: nil},
		{typ: "unknown", container: v1.Container{}, err: errors.New("unsupported chaincode type: unknown")},
	}

	b := Server{
		GoEnvImage:   "go-image",
		JavaEnvImage: "java-image",
		NodeEnvImage: "node-image",
	}
	for _, tt := range tests {
		t.Run(tt.typ, func(t *testing.T) {
			gt := NewGomegaWithT(t)

			container, err := b.runContainerDefinition(tt.typ, rmd)
			if tt.err != nil {
				gt.Expect(err).To(Equal(tt.err))
				return
			}
			gt.Expect(err).NotTo(HaveOccurred())
			gt.Expect(container).To(Equal(tt.container))
		})
	}
}

// This verifies the mainline for executePod. It's pretty long since it feeds
// watch events to the production code and verifies the appropriate actions for
// an entire execution.
func TestExecutePod(t *testing.T) {
	gt := NewGomegaWithT(t)

	fakeClientset := k8sfake.NewSimpleClientset()
	fakePodWatcher := watch.NewFakeWithChanSize(4, false) // buffered to hold 4 events
	fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakePodWatcher, nil))
	fakePodsClient := newFakePods(fakeClientset.CoreV1().Pods("test-ns"))

	// add event for pod with 2 init and 1 regular containers in waiting state
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-name"},
		Status: v1.PodStatus{
			Phase: v1.PodRunning,
			InitContainerStatuses: []v1.ContainerStatus{
				{Name: "init-container-1", State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{}}},
				{Name: "init-container-2", State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{}}},
			},
			ContainerStatuses: []v1.ContainerStatus{
				{Name: "container-1", State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{}}},
			},
		},
	}

	fakePodWatcher.Action(watch.Added, pod)
	fakePodsClient.Wg.Add(1)

	// modified event for pod with 1 init transitioning to terminated
	pod = pod.DeepCopyObject().(*v1.Pod)
	pod.Status.InitContainerStatuses = []v1.ContainerStatus{
		{Name: "init-container-1", State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{}}},
		{Name: "init-container-2", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
	}
	fakePodWatcher.Action(watch.Modified, pod)
	fakePodsClient.Wg.Add(1)

	// modified event for terminated init containers and running containers
	pod = pod.DeepCopyObject().(*v1.Pod)
	pod.Status.InitContainerStatuses = []v1.ContainerStatus{
		{Name: "init-container-1", State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{}}},
		{Name: "init-container-2", State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{}}},
	}
	pod.Status.ContainerStatuses = []v1.ContainerStatus{
		{Name: "container-1", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
	}
	fakePodWatcher.Action(watch.Modified, pod)
	// setup log stream client for first running container
	fakePodsClient.Wg.Add(1)

	pod = pod.DeepCopyObject().(*v1.Pod)
	pod.Status.Phase = v1.PodSucceeded
	pod.Status.ContainerStatuses = []v1.ContainerStatus{
		{Name: "container-1", State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{}}},
	}
	fakePodWatcher.Action(watch.Modified, pod)

	server := &Server{PodClient: fakePodsClient}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pod = &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-name"}}
	logger := &testLogger{}

	err := server.executePod(ctx, pod, logger, BUILD, nil)
	gt.Expect(err).NotTo(HaveOccurred())
	fakePodsClient.Wg.Wait()

	// verify expected actions
	// 0. watch pod
	// 1. create pod
	// 2. get logs for terminated init-container-1
	// 3. get logs for running init-container-2
	// 4. get logs for running container-1
	// 5. delete pod
	actions := fakeClientset.Actions()
	gt.Expect(actions).To(HaveLen(6))

	// watch
	gt.Expect(actions[0].GetVerb()).To(Equal("watch"))
	gt.Expect(actions[0].GetNamespace()).To(Equal("test-ns"))
	gt.Expect(actions[0].GetResource().Resource).To(Equal("pods"))
	gt.Expect(actions[0].(k8stesting.WatchAction).GetWatchRestrictions().Fields.String()).To(Equal("metadata.name=pod-name"))
	gt.Expect(actions[0].(k8stesting.WatchAction).GetWatchRestrictions().Labels).To(BeNil())
	gt.Expect(actions[0].(k8stesting.WatchAction).GetWatchRestrictions().ResourceVersion).To(Equal(""))

	// create
	gt.Expect(actions[1].GetVerb()).To(Equal("create"))
	gt.Expect(actions[1].GetNamespace()).To(Equal("test-ns"))
	gt.Expect(actions[1].GetResource().Resource).To(Equal("pods"))
	gt.Expect(actions[1].(k8stesting.CreateAction).GetObject()).To(Equal(&v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod-name"},
	}))

	// get log stream for init-container-1
	gt.Expect(actions[2].GetVerb()).To(Equal("get"))
	gt.Expect(actions[2].GetNamespace()).To(Equal("test-ns"))
	gt.Expect(actions[2].GetResource().Resource).To(Equal("pods"))
	gt.Expect(actions[2].GetSubresource()).To(Equal("log"))
	gt.Expect(actions[2].(k8stesting.GenericAction).GetValue()).To(Equal(&v1.PodLogOptions{
		Container: "init-container-1",
		Follow:    false,
	}))

	// get log stream for init-container-2
	gt.Expect(actions[3].GetVerb()).To(Equal("get"))
	gt.Expect(actions[3].GetNamespace()).To(Equal("test-ns"))
	gt.Expect(actions[3].GetResource().Resource).To(Equal("pods"))
	gt.Expect(actions[3].GetSubresource()).To(Equal("log"))
	gt.Expect(actions[3].(k8stesting.GenericAction).GetValue()).To(Equal(&v1.PodLogOptions{
		Container: "init-container-2",
		Follow:    true,
	}))

	// get log stream for container-1
	gt.Expect(actions[4].GetVerb()).To(Equal("get"))
	gt.Expect(actions[4].GetNamespace()).To(Equal("test-ns"))
	gt.Expect(actions[4].GetResource().Resource).To(Equal("pods"))
	gt.Expect(actions[4].GetSubresource()).To(Equal("log"))
	gt.Expect(actions[4].(k8stesting.GenericAction).GetValue()).To(Equal(&v1.PodLogOptions{
		Container: "container-1",
		Follow:    true,
	}))

	// delete the pod after termination
	gt.Expect(actions[5].GetVerb()).To(Equal("delete"))
	gt.Expect(actions[5].GetNamespace()).To(Equal("test-ns"))
	gt.Expect(actions[5].GetResource().Resource).To(Equal("pods"))
	gt.Expect(actions[5].(k8stesting.DeleteAction).GetName()).To(Equal("pod-name"))

	// verify logs
	gt.Expect(strings.Split(strings.TrimSpace(logger.String()), "\n")).To(ConsistOf([]string{
		"init-container-1: fake logs",
		"init-container-2: fake logs",
		"container-1: fake logs",
	}))
}

func TestExecutePodErrors(t *testing.T) {
	t.Run("PodWatchFailed", func(t *testing.T) {
		gt := NewGomegaWithT(t)
		fakeClientset := k8sfake.NewSimpleClientset()
		fakeClientset.PrependWatchReactor("pods", func(action k8stesting.Action) (bool, watch.Interface, error) {
			return true, nil, errors.New("broken-watch-time-flies")
		})

		server := &Server{PodClient: fakeClientset.CoreV1().Pods("test-ns")}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := server.executePod(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-name"}}, &testLogger{}, BUILD, nil)
		gt.Expect(err).To(MatchError("pod watch: broken-watch-time-flies"))
	})

	t.Run("PodCreateFailed", func(t *testing.T) {
		gt := NewGomegaWithT(t)
		fakeClientset := k8sfake.NewSimpleClientset()
		fakeClientset.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, errors.New("no-pod-create-for-you")
		})

		server := &Server{PodClient: fakeClientset.CoreV1().Pods("test-ns")}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := server.executePod(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-name"}}, &testLogger{}, BUILD, nil)
		gt.Expect(err).To(MatchError("pod create: no-pod-create-for-you"))
	})

	t.Run("ContextCanceled", func(t *testing.T) {
		gt := NewGomegaWithT(t)
		fakeClientset := k8sfake.NewSimpleClientset()

		server := &Server{PodClient: fakeClientset.CoreV1().Pods("test-ns")}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		err := server.executePod(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-name"}}, &testLogger{}, BUILD, nil)
		gt.Expect(err).To(MatchError("execute pod: context canceled"))
	})

	t.Run("PodWatchClosed", func(t *testing.T) {
		gt := NewGomegaWithT(t)
		fakeClientset := k8sfake.NewSimpleClientset()
		fakePodWatcher := watch.NewFakeWithChanSize(1, false)
		fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakePodWatcher, nil))

		server := &Server{PodClient: fakeClientset.CoreV1().Pods("test-ns")}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fakePodWatcher.Stop()
		err := server.executePod(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-name"}}, &testLogger{}, BUILD, nil)
		gt.Expect(err).To(MatchError("execute pod: watch closed before pod termination"))
	})

	t.Run("WatchErrorEvent", func(t *testing.T) {
		gt := NewGomegaWithT(t)
		fakeClientset := k8sfake.NewSimpleClientset()
		fakePodWatcher := watch.NewFakeWithChanSize(1, false)
		fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakePodWatcher, nil))

		server := &Server{PodClient: fakeClientset.CoreV1().Pods("test-ns")}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		fakePodWatcher.Error(&metav1.Status{Code: 500})
		err := server.executePod(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-name"}}, &testLogger{}, BUILD, nil)
		gt.Expect(err).To(MatchError(fmt.Sprintf("error event from watch: %v", &metav1.Status{Code: 500})))
	})

	t.Run("PodGetLogsFailed", func(t *testing.T) {
		gt := NewGomegaWithT(t)
		fakeClientset := k8sfake.NewSimpleClientset()
		fakePodWatcher := watch.NewFakeWithChanSize(2, false)
		fakePodWatcher.Action(watch.Modified, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-name"},
			Status: v1.PodStatus{
				Phase: v1.PodRunning,
				ContainerStatuses: []v1.ContainerStatus{
					{Name: "container-name", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
				},
			},
		})
		fakePodWatcher.Action(watch.Modified, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-name"},
			Status:     v1.PodStatus{Phase: v1.PodSucceeded},
		})
		fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakePodWatcher, nil))

		fakePodsClient := newFakePods(fakeClientset.CoreV1().Pods("test-ns"))
		// fakePodsClient.fakeClients["pod-name/container-name"] = &fakeHTTPClient{err: errors.New("no-logs-for-you")}
		fakePodsClient.LogError = errors.New("no-logs-for-you")
		server := &Server{PodClient: fakePodsClient}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		logger := &testLogger{}
		err := server.executePod(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-name"}}, logger, BUILD, nil)
		gt.Expect(err).NotTo(HaveOccurred())
		gt.Expect(logger.String()).To(ContainSubstring("no-logs-for-you\n"))
	})

	// Disabling this test, because I do not know how to break a stream in GetLogs function

	// t.Run("PodGetLogsStreamScanError", func(t *testing.T) {
	// 	gt := NewGomegaWithT(t)
	// 	fakeClientset := k8sfake.NewSimpleClientset()
	// 	fakePodWatcher := watch.NewFakeWithChanSize(2, false)
	// 	fakePodWatcher.Action(watch.Modified, &v1.Pod{
	// 		ObjectMeta: metav1.ObjectMeta{Name: "pod-name"},
	// 		Status: v1.PodStatus{
	// 			Phase: v1.PodRunning,
	// 			ContainerStatuses: []v1.ContainerStatus{
	// 				{Name: "container-name", State: v1.ContainerState{Running: &v1.ContainerStateRunning{}}},
	// 			},
	// 		},
	// 	})
	// 	fakePodWatcher.Action(watch.Modified, &v1.Pod{
	// 		ObjectMeta: metav1.ObjectMeta{Name: "pod-name"},
	// 		Status:     v1.PodStatus{Phase: v1.PodSucceeded},
	// 	})
	// 	fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakePodWatcher, nil))

	// 	httpClient := newFakeHTTPClient()
	// 	httpClient.pw.CloseWithError(errors.New("closed-with-error"))
	// 	fakePodsClient := newFakePods(fakeClientset.CoreV1().Pods("test-ns"))
	// 	fakePodsClient.fakeClients["pod-name/container-name"] = httpClient
	// 	server := &Server{PodClient: fakePodsClient}
	// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	// 	defer cancel()

	// 	logger := &testLogger{}
	// 	err := server.executePod(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-name"}}, logger, BUILD)
	// 	gt.Expect(err).NotTo(HaveOccurred())
	// 	gt.Expect(logger.String()).To(Equal("container-name: scanning log lines failed: closed-with-error\n"))
	// })

	t.Run("PodDeleteFailed", func(t *testing.T) {
		gt := NewGomegaWithT(t)
		fakeClientset := k8sfake.NewSimpleClientset()
		fakePodWatcher := watch.NewFakeWithChanSize(1, false)
		fakePodWatcher.Action(watch.Modified, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-name"},
			Status:     v1.PodStatus{Phase: v1.PodSucceeded},
		})
		fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakePodWatcher, nil))

		fakeClientset.PrependReactor("delete", "pods", func(action k8stesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, errors.New("no-pod-delete-for-you")
		})

		server := &Server{PodClient: fakeClientset.CoreV1().Pods("test-ns")}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		logger := &testLogger{}
		err := server.executePod(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-name"}}, logger, BUILD, nil)
		gt.Expect(err).NotTo(HaveOccurred())
		gt.Expect(logger.String()).To(Equal("pod delete: no-pod-delete-for-you\n"))
	})

	t.Run("PodStatusFailed", func(t *testing.T) {
		gt := NewGomegaWithT(t)
		fakeClientset := k8sfake.NewSimpleClientset()
		fakePodWatcher := watch.NewFakeWithChanSize(1, false)
		fakePodWatcher.Action(watch.Modified, &v1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "pod-name"},
			Status:     v1.PodStatus{Phase: v1.PodFailed},
		})
		fakeClientset.PrependWatchReactor("pods", k8stesting.DefaultWatchReactor(fakePodWatcher, nil))

		server := &Server{PodClient: fakeClientset.CoreV1().Pods("test-ns")}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := server.executePod(ctx, &v1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-name"}}, &testLogger{}, BUILD, nil)
		gt.Expect(err).To(MatchError("pod terminated: pod status is failed"))
	})
}

func TestRunMetadataToPopulateScript(t *testing.T) {
	t.Skip()
	gt := NewGomegaWithT(t)

	rmd := &runMetadata{
		ChaincodeID: "chaincode-id",
		ClientCert:  "client-cert",
		ClientKey:   "client-key",
		RootCert:    "root-cert",
		PeerAddress: "peer-address",
	}

	expectedScript := `mkdir -p /certs/
head -c -1 <<EOF_1 > /certs/peer.crt
root-cert
EOF_1
head -c -1 <<EOF_2 > /certs/client_pem.key
client-key
EOF_2
head -c -1 <<EOF_3 > /certs/client_pem.crt
client-cert
EOF_3
head -c -1 <<EOF_4 > /certs/client.key
Y2xpZW50LWtleQ==
EOF_4
head -c -1 <<EOF_5 > /certs/client.crt
Y2xpZW50LWNlcnQ=
EOF_5
`
	gt.Expect(rmd.toPopulateScript()).To(Equal(expectedScript))
}

func TestRunMetadataToRunEnv(t *testing.T) {
	gt := NewGomegaWithT(t)
	os.Setenv("CORE_PEER_LOCALMSPID", "test-org")
	rmd := &runMetadata{ChaincodeID: "chaincode-id"}
	gt.Expect(rmd.toRunEnv()).To(ConsistOf([]v1.EnvVar{
		{Name: "CORE_CHAINCODE_ID_NAME", Value: "chaincode-id"},
		{Name: "CORE_PEER_TLS_ENABLED", Value: "true"},
		{Name: "CORE_PEER_TLS_ROOTCERT_FILE", Value: "/certs/peer.crt"},
		{Name: "CORE_TLS_CLIENT_KEY_PATH", Value: "/certs/client.key"},
		{Name: "CORE_TLS_CLIENT_CERT_PATH", Value: "/certs/client.crt"},
		{Name: "CORE_TLS_CLIENT_KEY_FILE", Value: "/certs/client_pem.key"},
		{Name: "CORE_TLS_CLIENT_CERT_FILE", Value: "/certs/client_pem.crt"},
		{Name: "CORE_PEER_LOCALMSPID", Value: "test-org"},
	}))
}

func TestExtractChaincodeMetadata(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "extract-cc-metadata")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	empty, err := os.Create(filepath.Join(tempDir, "empty.tar"))
	gt.Expect(err).NotTo(HaveOccurred())
	writeTar(t, empty, map[string]string{".": ""})

	good, err := os.Create(filepath.Join(tempDir, "good.tar"))
	gt.Expect(err).NotTo(HaveOccurred())
	writeTar(t, good, map[string]string{"metadata.json": "{}"})

	err = ioutil.WriteFile(filepath.Join(tempDir, "bad.tar"), []byte("goo"), 0644)
	gt.Expect(err).NotTo(HaveOccurred())

	badJSON, err := os.Create(filepath.Join(tempDir, "bad-json.tar"))
	gt.Expect(err).NotTo(HaveOccurred())
	writeTar(t, badJSON, map[string]string{"metadata.json": "xx"})

	server := &Server{SharedPath: tempDir}
	tests := []struct {
		fn         string
		errMatcher interface{}
		md         *chaincodeMetadata
	}{
		{fn: "good.tar", errMatcher: nil, md: &chaincodeMetadata{}},
		{fn: "../metadata.tar", errMatcher: Equal("invalid metadata path: ../metadata.tar"), md: nil},
		{fn: "bad.tar", errMatcher: Equal("unexpected EOF"), md: nil},
		{fn: "missing.tar", errMatcher: ContainSubstring("missing.tar: no such file or directory"), md: nil},
		{fn: "empty.tar", errMatcher: Equal("metadata.json not found"), md: nil},
		{fn: "bad-json.tar", errMatcher: Equal("invalid character 'x' looking for beginning of value"), md: nil},
	}
	for _, tt := range tests {
		t.Run(tt.fn, func(t *testing.T) {
			gt := NewGomegaWithT(t)

			md, err := server.extractChaincodeMetadata(tt.fn)
			if tt.errMatcher != nil {
				gt.Expect(err).To(MatchError(tt.errMatcher))
				return
			}
			gt.Expect(err).NotTo(HaveOccurred())
			gt.Expect(md).To(Equal(tt.md))
		})
	}
}

func TestReadRunMetadata(t *testing.T) {
	gt := NewGomegaWithT(t)
	tempDir, err := ioutil.TempDir("", "extract-run-metadata")
	gt.Expect(err).NotTo(HaveOccurred())
	defer os.RemoveAll(tempDir)

	err = ioutil.WriteFile(filepath.Join(tempDir, "good.json"), []byte("{}"), 0644)
	gt.Expect(err).NotTo(HaveOccurred())
	err = ioutil.WriteFile(filepath.Join(tempDir, "bad.json"), []byte("xx"), 0644)
	gt.Expect(err).NotTo(HaveOccurred())

	server := &Server{SharedPath: tempDir}
	tests := []struct {
		fn         string
		errMatcher interface{}
		rmd        *runMetadata
	}{
		{fn: "good.json", errMatcher: nil, rmd: &runMetadata{}},
		{fn: "bad.json", errMatcher: Equal("invalid character 'x' looking for beginning of value"), rmd: nil},
		{fn: "missing.json", errMatcher: ContainSubstring("missing.json: no such file or directory"), rmd: &runMetadata{}},
		{fn: "../rel.json", errMatcher: Equal("invalid metadata path: ../rel.json"), rmd: nil},
	}

	for _, tt := range tests {
		t.Run(tt.fn, func(t *testing.T) {
			gt := NewGomegaWithT(t)

			rmd, err := server.readRunMetadata(tt.fn)
			if tt.errMatcher != nil {
				gt.Expect(err).To(MatchError(tt.errMatcher))
				return
			}
			gt.Expect(err).NotTo(HaveOccurred())
			gt.Expect(rmd).To(Equal(tt.rmd))
		})
	}
}

func TestUUID(t *testing.T) {
	gt := NewGomegaWithT(t)
	for u, i := UUID(), 0; i < 3; i++ {
		uuid := UUID()
		gt.Expect(uuid).To(HaveLen(36))
		gt.Expect(uuid).NotTo(Equal(u))
	}
}

func writeTar(t *testing.T, w io.WriteCloser, entries map[string]string) {
	gt := NewGomegaWithT(t)
	defer w.Close()

	tw := tar.NewWriter(w)
	defer tw.Close()

	for name, contents := range entries {
		mode := int64(0644)
		if strings.HasSuffix(name, "/") {
			mode = 0755
		}

		err := tw.WriteHeader(&tar.Header{
			Name: name,
			Size: int64(len(contents)),
			Mode: mode,
		})
		gt.Expect(err).NotTo(HaveOccurred())
		_, err = tw.Write([]byte(contents))
		gt.Expect(err).NotTo(HaveOccurred())
	}
}

type testLogger struct {
	bytes.Buffer
	lock sync.Mutex
}

func (tl *testLogger) Log(msg string) {
	tl.lock.Lock()
	defer tl.lock.Unlock()
	tl.Buffer.WriteString(msg)
}

// Workaround https://github.com/kubernetes/kubernetes/issues/84203 by wrapping
// GetLogs and returning a *restclient.Request we control.
type FakePods struct {
	*corev1fake.FakePods
	fakeClients map[string]*fakeHTTPClient
	Wg          *sync.WaitGroup
	LogError    error
}

func newFakePods(pods corev1.PodInterface) *FakePods {
	return &FakePods{
		FakePods:    pods.(*corev1fake.FakePods),
		fakeClients: map[string]*fakeHTTPClient{},
		Wg:          &sync.WaitGroup{},
		LogError:    nil,
	}
}

func (f *FakePods) GetLogs(name string, opts *v1.PodLogOptions) *restclient.Request {
	f.FakePods.GetLogs(name, opts) // delegate to capture action

	fakeClient := &fakerest.RESTClient{
		Client: fakerest.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
			if f.LogError != nil {
				return nil, f.LogError
			}
			resp := &http.Response{
				StatusCode: http.StatusOK,
				Body:       ioutil.NopCloser(strings.NewReader("fake logs")),
			}

			f.Wg.Done()
			return resp, nil
		}),
	}

	return fakeClient.Request()
}

func (f *FakePods) httpClient(name, container string) *fakeHTTPClient {
	if client, ok := f.fakeClients[name+"/"+container]; ok {
		return client
	}
	client := newFakeHTTPClient()
	client.pw.Close()
	return client
}

// This is going to to include io.PipeReader and io.PipeWriter so we can
// control the body.
type fakeHTTPClient struct {
	pw  *io.PipeWriter
	pr  *io.PipeReader
	err error
}

func newFakeHTTPClient() *fakeHTTPClient {
	pr, pw := io.Pipe()
	return &fakeHTTPClient{
		pr: pr,
		pw: pw,
	}
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	if f.err != nil {
		return nil, f.err
	}

	return &http.Response{
		Status:     "200 OK",
		StatusCode: http.StatusOK,
		Body:       f.pr,
	}, nil
}

//go:generate counterfeiter -o fakes/buildstream.go --fake-name BuildStream . buildStream
//go:generate counterfeiter -o fakes/runstream.go --fake-name RunStream . runStream

type buildStream interface{ sidecar.Builder_BuildServer }
type runStream interface{ sidecar.Runner_RunServer }
