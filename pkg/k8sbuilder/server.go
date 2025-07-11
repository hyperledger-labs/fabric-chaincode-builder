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
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/sidecar"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

// Server is the server side implementation of the external Build and Run contracts.
type Server struct {
	SecretClient      corev1.SecretInterface
	PodClient         corev1.PodInterface
	NodeClient        corev1.NodeInterface
	SharedPath        string
	FSBaseURL         string
	FileTransferImage string
	BuilderImage      string
	GoEnvImage        string
	JavaEnvImage      string
	NodeEnvImage      string
	ImagePullSecrets  []string
	PeerID            string // PeerID is the unique peer identifier.
	Auth              *BasicAuth
	TLSCertString     string
}

type BasicAuth struct {
	Username string
	Password string
}

var _ sidecar.BuilderServer = &Server{}
var _ sidecar.RunnerServer = &Server{}

type Stage string

var (
	BUILD Stage = "build"
	RUN   Stage = "run"
)

// Build creates a Kubernetes bare Pod to build Fabric chaincode.
func (b *Server) Build(req *sidecar.BuildRequest, stream sidecar.Builder_BuildServer) error {
	stream = &serializedBuildStream{Builder_BuildServer: stream}

	metadata, err := b.extractChaincodeMetadata(req.MetadataPath)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to read metadata: %s", err)
	}

	buildContainer, err := b.buildContainerDefinition(metadata)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "unable to generate build container: %s", err)
	}

	peerPodName := os.Getenv("PEER_POD_NAME")
	if peerPodName == "" {
		return status.Errorf(codes.Unknown, "failed to get peer pod name, env var not passed")
	}
	peerPodUID := os.Getenv("PEER_POD_UID")
	if peerPodUID == "" {
		return status.Errorf(codes.Unknown, "failed to get peer pod uid, env var not passed")
	}

	pod := b.buildPodDefinition(buildContainer, req.SourcePath, req.OutputPath, peerPodName, peerPodUID)

	err = b.executePod(stream.Context(), pod, stream.(logger), BUILD, nil)
	if err != nil {
		return status.Errorf(codes.Unknown, "build failed: %s", err)
	}

	return nil
}

type serializedBuildStream struct {
	sidecar.Builder_BuildServer
	mutex sync.Mutex
}

func (s *serializedBuildStream) Send(msg *sidecar.BuildResponse) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.Builder_BuildServer.Send(msg)
}

func (s *serializedBuildStream) Log(msg string) {
	_ = s.Send(&sidecar.BuildResponse{
		Message: &sidecar.BuildResponse_StandardError{StandardError: msg},
	})
}

// Run creates a Kubernetes bare Pod to execute Fabric chaincode.
func (b *Server) Run(req *sidecar.RunRequest, stream sidecar.Runner_RunServer) (err error) {
	stream = &serializedRunStream{Runner_RunServer: stream}

	metadata, err := b.extractChaincodeMetadata(req.MetadataPath)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "failed to read chaincode metadata: %s", err)
	}

	if metadata.Type == "external" {
		return nil
	}

	runMetadata, err := b.readRunMetadata(req.ArtifactsPath)
	if err != nil {
		return status.Errorf(codes.Unknown, "failed to extract run metadata: %s", err)
	}

	uuid := UUID()
	if err := b.createCryptoSecret(runMetadata, uuid); err != nil {
		return status.Errorf(codes.InvalidArgument, "unable to create crypto secret: %s", err)
	}
	defer func() {
		if deleteErr := b.deleteCryptoSecret(uuid, stream.(logger)); deleteErr != nil {
			err = deleteErr
		}
	}()

	runContainer, err := b.runContainerDefinition(metadata.Type, runMetadata)
	if err != nil {
		return status.Errorf(codes.InvalidArgument, "unable to generate run container: %s", err)
	}

	peerPodName := os.Getenv("PEER_POD_NAME")
	if peerPodName == "" {
		return status.Errorf(codes.Unknown, "failed to get peer pod name, env var not passed")
	}
	peerPodUID := os.Getenv("PEER_POD_UID")
	if peerPodUID == "" {
		return status.Errorf(codes.Unknown, "failed to get peer pod UID, env var not passed")
	}

	pod := b.runPodDefinition(runContainer, runMetadata, req.ChaincodePath, uuid, peerPodName, peerPodUID)
	err = b.executePod(stream.Context(), pod, stream.(logger), RUN, &executeOptions{UUID: uuid})
	if err != nil {
		return status.Errorf(codes.Unknown, "run failed: %s", err)
	}

	return nil
}

type serializedRunStream struct {
	sidecar.Runner_RunServer
	mutex sync.Mutex
}

func (s *serializedRunStream) Send(msg *sidecar.RunResponse) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	return s.Runner_RunServer.Send(msg)
}

func (s *serializedRunStream) Log(msg string) {
	_ = s.Send(&sidecar.RunResponse{
		Message: &sidecar.RunResponse_StandardError{StandardError: msg},
	})
}

// A logger processes log messages.
type logger interface {
	// Log the provided message.
	Log(msg string)
}

// executeOptions contains additional fields to execute pod if required
type executeOptions struct {
	UUID string
}

func (b *Server) executePod(ctx context.Context, pod *v1.Pod, logger logger, stage Stage, opts *executeOptions) error {
	selector := fields.OneTermEqualSelector("metadata.name", pod.Name).String()
	watcher, err := b.PodClient.Watch(context.TODO(), metav1.ListOptions{FieldSelector: selector})
	if err != nil {
		return fmt.Errorf("pod watch: %s", err)
	}
	defer watcher.Stop()

	// Create the pod *after* setting up the watches.
	pod, err = b.PodClient.Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		return fmt.Errorf("pod create: %s", err)
	}

	// Need to set controller references to crypto secret in run container.
	if stage == RUN && opts != nil {
		if err = b.addControllerReferenceToCryptoSecret(pod, opts.UUID); err != nil {
			logger.Log(fmt.Sprintf("failed to add controller reference to crypto secret: %s\n", err))
			return err
		}
	}

	// Want to keep run container to be long living, so will not explicitly delete the pod on the run flow.
	// However, the build container is not useful after it has completed and can be deleted.
	if stage == BUILD {
		defer b.deletePod(pod.Name, logger)
	}

	var wg sync.WaitGroup
	var activeLogStreams = make(map[string]io.ReadCloser)

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("execute pod: %s", ctx.Err())

		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("execute pod: watch closed before pod termination")
			}

			switch event.Type {
			case watch.Added, watch.Modified:
			case watch.Error:
				return fmt.Errorf("error event from watch: %v", event.Object)
			default:
				continue
			}

			pod := event.Object.(*v1.Pod)
			for _, status := range containerStatuses(pod) {
				if status.State.Running == nil && status.State.Terminated == nil {
					continue
				}
				if _, ok := activeLogStreams[status.Name]; ok {
					continue
				}

				podLogOptions := &v1.PodLogOptions{
					Container: status.Name,
					Follow:    status.State.Running != nil,
				}
				stream, err := b.PodClient.GetLogs(pod.Name, podLogOptions).Stream(context.TODO())
				if err != nil {
					logger.Log(fmt.Sprintf("%s: get logs: %s\n", pod.Name, err))
					continue
				}

				defer func() {
					if err := stream.Close(); err != nil {
						logger.Log(fmt.Sprintf("Error closing stream: %s\n", err))
					}
				}()

				activeLogStreams[status.Name] = stream

				wg.Add(1)
				go streamLogs(stream, status.Name, logger, &wg)
			}

			if pod.Status.Phase == v1.PodFailed || pod.Status.Phase == v1.PodSucceeded {
				wg.Wait() // wait for log streaming to complete
				if pod.Status.Phase == v1.PodFailed {
					return fmt.Errorf("pod terminated: pod status is failed")
				}
				return nil
			}
		}
	}
}

// deletePod deletes a pod and logs any errors that occur.
func (b *Server) deletePod(podName string, logger logger) {
	err := b.PodClient.Delete(context.TODO(), podName, metav1.DeleteOptions{})
	if err != nil {
		logger.Log(fmt.Sprintf("pod delete: %s\n", err))
	}
}

// streamLogs extracts lines from logStream, prepends the log name, and logs
// each line with the provided logger.
//
// wg.Done is called when the stream processing has completed.
func streamLogs(logStream io.Reader, name string, logger logger, wg *sync.WaitGroup) {
	defer wg.Done()

	scanner := bufio.NewScanner(logStream)
	for scanner.Scan() {
		logger.Log(fmt.Sprintf("%s: %s\n", name, scanner.Text()))
	}
	if err := scanner.Err(); err != nil {
		logger.Log(fmt.Sprintf("%s: scanning log lines failed: %s\n", name, err))
	}
}

func containerStatuses(pod *v1.Pod) []v1.ContainerStatus {
	var cs []v1.ContainerStatus
	cs = append(cs, pod.Status.InitContainerStatuses...)
	cs = append(cs, pod.Status.ContainerStatuses...)
	return cs
}

func (b *Server) buildPodDefinition(buildContainer v1.Container, sourcePath, outputPath, peerPodName, peerPodUID string) *v1.Pod {
	sourceURL := urlJoin(b.FSBaseURL, sourcePath)
	outputURL := urlJoin(b.FSBaseURL, outputPath)
	user := int64(7051)

	boolTrue := true
	ref := metav1.OwnerReference{
		APIVersion:         "v1",
		Kind:               "Pod",
		Name:               peerPodName,
		UID:                types.UID(peerPodUID),
		BlockOwnerDeletion: &boolTrue,
		Controller:         &boolTrue,
	}

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "chaincode-build-" + UUID(),
			Labels: map[string]string{
				"peer-id": b.PeerID,
			},
			OwnerReferences: []metav1.OwnerReference{ref},
		},
		Spec: v1.PodSpec{
			ImagePullSecrets: genPullSecretList(b.ImagePullSecrets),
			RestartPolicy:    v1.RestartPolicyNever,
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
					Image:           b.FileTransferImage,
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{"/bin/sh"},
					Args: []string{
						"-c",
						"mkdir -p /chaincode/input /chaincode/output /chaincode/certs && chmod 777 /chaincode/input /chaincode/output /chaincode/certs && echo -e ${TLS_CERT} > /chaincode/certs/cacert.pem",
					},
					Env: []v1.EnvVar{
						v1.EnvVar{
							Name:  "TLS_CERT",
							Value: b.TLSCertString,
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{Name: "chaincode", MountPath: "/chaincode"},
					},
				},
				{
					Name:            "download-chaincode-source",
					Image:           b.FileTransferImage,
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{"/bin/sh"},
					Args: []string{
						"-c",
						fmt.Sprintf("curl -u ${USERNAME}:${PASSWORD} --cacert /chaincode/certs/cacert.pem -s -o- -L '%s' | tar -C /chaincode/input -xvf - && chmod -R 777 /chaincode/input", sourceURL),
					},
					Env: []v1.EnvVar{
						v1.EnvVar{
							Name:  "USERNAME",
							Value: b.Auth.Username,
						},
						v1.EnvVar{
							Name:  "PASSWORD",
							Value: b.Auth.Password,
						},
					},
					VolumeMounts: []v1.VolumeMount{
						{Name: "chaincode", MountPath: "/chaincode"},
					},
				},
				buildContainer,
			},
			Containers: []v1.Container{
				{
					Name:            "upload-chaincode-output",
					Image:           b.FileTransferImage,
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{"/bin/sh"},
					Args: []string{
						"-c",
						fmt.Sprintf("cd /chaincode/output && tar cvf /chaincode/output.tar $(ls -A) && curl -u ${USERNAME}:${PASSWORD} --cacert /chaincode/certs/cacert.pem -s --upload-file /chaincode/output.tar '%s'", outputURL),
					},
					Env: []v1.EnvVar{
						v1.EnvVar{
							Name:  "USERNAME",
							Value: b.Auth.Username,
						},
						v1.EnvVar{
							Name:  "PASSWORD",
							Value: b.Auth.Password,
						},
					},
					SecurityContext: &v1.SecurityContext{
						RunAsUser: &user,
					},
					VolumeMounts: []v1.VolumeMount{
						{Name: "chaincode", MountPath: "/chaincode"},
					},
				},
			},
		},
	}
}

func (b *Server) buildContainerDefinition(md *chaincodeMetadata) (v1.Container, error) {
	switch strings.ToLower(md.Type) {
	case "golang":
		return golangBuildContainer(md.Path, b.BuilderImage), nil
	case "java":
		return javaBuildContainer(b.JavaEnvImage), nil
	case "node":
		return nodeBuildContainer(b.BuilderImage), nil
	case "executable":
		return executableBuildContainer(b.BuilderImage), nil
	default:
		return v1.Container{}, fmt.Errorf("unsupported chaincode type: %s", md.Type)
	}
}

func (b *Server) runPodDefinition(runContainer v1.Container, runMetadata *runMetadata, chaincodePath, uuid string, peerPodName, peerPodUID string) *v1.Pod {
	chaincodeURL := urlJoin(b.FSBaseURL, chaincodePath)

	chaincodeID := runMetadata.ChaincodeID
	chaincodeID = strings.Replace(chaincodeID, ":", "-", -1)
	if len(chaincodeID) > 63 {
		chaincodeID = chaincodeID[:62]
	}
	boolTrue := true
	ref := metav1.OwnerReference{
		APIVersion:         "v1",
		Kind:               "Pod",
		Name:               peerPodName,
		UID:                types.UID(peerPodUID),
		BlockOwnerDeletion: &boolTrue,
		Controller:         &boolTrue,
	}

	var affinity *v1.Affinity = nil
	if b.hasZoneLabels() {
		affinity = &v1.Affinity{
			PodAffinity: &v1.PodAffinity{
				RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
					{
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": b.PeerID,
							},
						},
						TopologyKey: "topology.kubernetes.io/zone",
					},
				},
			},
		}
	}

	return &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "chaincode-execution-" + uuid,
			Labels: map[string]string{
				"peer-id":      b.PeerID,
				"chaincode-id": chaincodeID,
				"app":          b.PeerID,
			},
			OwnerReferences: []metav1.OwnerReference{ref},
		},
		Spec: v1.PodSpec{
			RestartPolicy:    v1.RestartPolicyNever,
			ImagePullSecrets: genPullSecretList(b.ImagePullSecrets),
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
							SecretName: "certs-" + uuid,
						},
					},
				},
			},
			InitContainers: []v1.Container{
				{
					Name:            "download-chaincode-output",
					Image:           b.FileTransferImage,
					ImagePullPolicy: v1.PullAlways,
					Command:         []string{"/bin/sh"},
					Args: []string{
						"-c",
						fmt.Sprintf("mkdir -p /chaincode/output /chaincode/certs && chmod -R 777 /chaincode/output /chaincode/certs && echo -e ${TLS_CERT} > /chaincode/certs/cacert.pem && curl -u ${USERNAME}:${PASSWORD} --cacert /chaincode/certs/cacert.pem -s -o- -L '%s' | tar -C /chaincode/output -xvf -", chaincodeURL),
					},
					Env: []v1.EnvVar{
						{Name: "USERNAME", Value: b.Auth.Username},
						{Name: "PASSWORD", Value: b.Auth.Password},
						{Name: "TLS_CERT", Value: b.TLSCertString},
					},
					VolumeMounts: []v1.VolumeMount{
						{Name: "chaincode", MountPath: "/chaincode"},
					},
				},
			},
			Containers: []v1.Container{
				runContainer,
			},
			Affinity: affinity, // Updated affinity here
		},
	}
}

func (b *Server) hasZoneLabels() bool {
	nodes, err := b.NodeClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		// If API call fails, better to assume zone labels are absent
		return false
	}
	for _, node := range nodes.Items {
		if _, ok := node.Labels["topology.kubernetes.io/zone"]; ok {
			return true
		}
	}
	return false
}

func (b *Server) runContainerDefinition(ccType string, rmd *runMetadata) (v1.Container, error) {
	switch strings.ToLower(ccType) {
	case "golang":
		return golangRunContainer(rmd, b.GoEnvImage), nil
	case "java":
		return javaRunContainer(rmd, b.JavaEnvImage), nil
	case "node":
		return nodeRunContainer(rmd, b.NodeEnvImage), nil
	case "executable":
		return executableRunContainer(rmd, b.GoEnvImage), nil
	case "external":
		// should not come here
		return v1.Container{}, fmt.Errorf("something went wrong, chaincode type: %s", ccType)
	default:
		return v1.Container{}, fmt.Errorf("unsupported chaincode type: %s", ccType)
	}
}

// containsDotDot checks for parent path traversal elements.
func containsDotDot(s string) bool {
	if !strings.Contains(s, "..") {
		return false
	}
	for _, segment := range strings.FieldsFunc(s, func(r rune) bool { return r == os.PathSeparator }) {
		if segment == ".." {
			return true
		}
	}
	return false
}

// The chaincodeMetadata contains the chaincode metadata from the package
// installed to the peer.
type chaincodeMetadata struct {
	Path string `json:"path"` // Path is used when building go chaincode.
	Type string `json:"type"` // Type determines the chaincode language.
}

// runMetadata contains the information provided by the peer that is required
// to run chaincode.
type runMetadata struct {
	ChaincodeID string `json:"chaincode_id"`
	ClientCert  string `json:"client_cert"`
	ClientKey   string `json:"client_key"`
	RootCert    string `json:"root_cert"`
	PeerAddress string `json:"peer_address"`
}

// NOTE: Certificate and key are being put in secrets. The primary purpose of this
// method was to put certificate and keys in the /certs folder, Leaving it here for now,
// since in the future the runMetadata might contain other artificats that can be handled here.

// toPopulateScript generates a bash script fragment that populates chincode
// artifacts from the run metadata provided by the peer.
func (r runMetadata) toPopulateScript() string {
	buf := &bytes.Buffer{}
	return buf.String()
}

// toRunEnv generates the environment variables that reference the runtime
// artifacts populated by toPopulateScript.
func (r runMetadata) toRunEnv() []v1.EnvVar {
	return []v1.EnvVar{
		{Name: "CORE_CHAINCODE_ID_NAME", Value: r.ChaincodeID},
		{Name: "CORE_PEER_TLS_ENABLED", Value: "true"},
		{Name: "CORE_PEER_TLS_ROOTCERT_FILE", Value: "/certs/peer.crt"},
		{Name: "CORE_TLS_CLIENT_KEY_PATH", Value: "/certs/client.key"},
		{Name: "CORE_TLS_CLIENT_CERT_PATH", Value: "/certs/client.crt"},
		{Name: "CORE_TLS_CLIENT_KEY_FILE", Value: "/certs/client_pem.key"},
		{Name: "CORE_TLS_CLIENT_CERT_FILE", Value: "/certs/client_pem.crt"},
		{Name: "CORE_PEER_LOCALMSPID", Value: os.Getenv("CORE_PEER_LOCALMSPID")},
	}
}

func (r runMetadata) toSecretData() map[string]string {
	return map[string]string{
		"peer.crt":       r.RootCert,
		"client_pem.crt": r.ClientCert,
		"client_pem.key": r.ClientKey,
		"client.crt":     base64.StdEncoding.EncodeToString([]byte(r.ClientCert)),
		"client.key":     base64.StdEncoding.EncodeToString([]byte(r.ClientKey)),
	}
}

func (b *Server) extractChaincodeMetadata(metadataPath string) (*chaincodeMetadata, error) {
	if containsDotDot(metadataPath) {
		return nil, fmt.Errorf("invalid metadata path: %s", metadataPath)
	}
	metadataPath = filepath.Join(b.SharedPath, metadataPath)

	var md chaincodeMetadata
	archive, err := os.Open(filepath.Clean(metadataPath))
	if err != nil {
		return nil, err
	}
	if archive != nil {
		// The following line is nosec because
		// we check for nil pointer & not close it in the code
		defer archive.Close() // #nosec
	}

	tr := tar.NewReader(archive)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("metadata.json not found")
		}
		if err != nil {
			return nil, err
		}
		if hdr.Name == "metadata.json" {
			break
		}
	}
	contents, err := ioutil.ReadAll(tr)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(contents, &md)
	if err != nil {
		return nil, err
	}
	return &md, nil
}

func (b *Server) readRunMetadata(metadataPath string) (*runMetadata, error) {
	if containsDotDot(metadataPath) {
		return nil, fmt.Errorf("invalid metadata path: %s", metadataPath)
	}
	metadataPath = filepath.Join(b.SharedPath, metadataPath)
	contents, err := ioutil.ReadFile(filepath.Clean(metadataPath))
	if err != nil {
		return nil, err
	}

	var md runMetadata
	err = json.Unmarshal(contents, &md)
	if err != nil {
		return nil, err
	}
	return &md, nil
}

func (b *Server) createCryptoSecret(m *runMetadata, uuid string) error {
	secret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "certs-" + uuid,
		},
		StringData: m.toSecretData(),
	}

	if _, err := b.SecretClient.Create(context.TODO(), secret, metav1.CreateOptions{}); err != nil {
		return err
	}

	return nil
}

func (b *Server) deleteCryptoSecret(uuid string, logger logger) error {
	name := "certs-" + uuid
	secret, err := b.SecretClient.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		// Do nothing if secret has already been deleted
		return nil
	}

	if secret.OwnerReferences != nil && len(secret.OwnerReferences) > 0 {
		// Do nothing if secret has a controller references as it will be garbage collected
		// when the owner is deleted
		return nil
	}

	logger.Log("crypto secret does not have owner references, deleting secret\n")
	return b.SecretClient.Delete(context.TODO(), name, metav1.DeleteOptions{})
}

var SchemeGroupVersion = schema.GroupVersion{Group: "Pod", Version: "v1"}

func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&v1.Pod{},
	)
	metav1.AddToGroupVersion(scheme, SchemeGroupVersion)
	return nil
}

func (b *Server) addControllerReferenceToCryptoSecret(owner metav1.Object, uuid string) error {
	name := "certs-" + uuid
	secret, err := b.SecretClient.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get %s secret: %s", name, err)
	}

	boolTrue := true
	ref := metav1.OwnerReference{
		APIVersion:         "v1",
		Kind:               "Pod",
		Name:               owner.GetName(),
		UID:                owner.GetUID(),
		BlockOwnerDeletion: &boolTrue,
		Controller:         &boolTrue,
	}
	secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{ref}

	if _, err := b.SecretClient.Update(context.TODO(), secret, metav1.UpdateOptions{}); err != nil {
		return err
	}

	return nil
}

// UUID returns a UUID based on RFC 4122.
func UUID() string {
	uuid := make([]byte, 16)
	if _, err := rand.Read(uuid); err != nil {
		panic(fmt.Errorf("failed to generate UUID: %s", err))
	}
	uuid[8] = uuid[8]&^0xc0 | 0x80 // variant bits; section 4.1.1
	uuid[6] = uuid[6]&^0xf0 | 0x40 // version 4 (pseudo-random); section 4.1.3

	return fmt.Sprintf("%x-%x-%x-%x-%x", uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:])
}

func urlJoin(rawurl, pathSegment string) string {
	u, err := url.Parse(rawurl)
	if err != nil {
		panic(err)
	}
	u.Path = path.Join(u.Path, pathSegment)
	return u.String()
}

func genPullSecretList(secrets []string) (pullSecrets []v1.LocalObjectReference) {
	for _, secret := range secrets {
		pullSecret := v1.LocalObjectReference{
			Name: secret,
		}
		pullSecrets = append(pullSecrets, pullSecret)
	}
	return
}
