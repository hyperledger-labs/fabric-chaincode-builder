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
package integration_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	pb "github.com/hyperledger/fabric-protos-go/peer"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

var (
	clientSet *kubernetes.Clientset
)

var _ = SynchronizedBeforeSuite(func() []byte {
	kubeconfig := k8sConfigPath()

	clientConfig, err := restClientConfig(kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to establish connection to kubernetes: %s", err)
		os.Exit(10)
	}

	clientSet, err = kubernetes.NewForConfig(clientConfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to create client set: %s", err)
		os.Exit(11)
	}

	b, err := base64.StdEncoding.DecodeString(os.Getenv("DOCKERCONFIGJSON"))
	Expect(err).NotTo(HaveOccurred())

	_, err = clientSet.CoreV1().Secrets("default").Create(context.TODO(), &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "regcred",
			Namespace: "default",
		},
		Data: map[string][]byte{
			".dockerconfigjson": b,
		},
		Type: corev1.SecretTypeDockerConfigJson,
	}, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	vendor := "testdata/go-chaincode/source/src/vendor"
	_, err = os.Stat(vendor)
	if err != nil && os.IsNotExist(err) {
		vendorCmd := exec.Command("go", "mod", "vendor")
		vendorCmd.Dir = "testdata/go-chaincode/source/src"
		sess, err := gexec.Start(vendorCmd, GinkgoWriter, GinkgoWriter)
		Expect(err).NotTo(HaveOccurred())
		Eventually(sess, 2*time.Minute).Should(gexec.Exit(0))
	}
	return nil
}, func(data []byte) {
})

var _ = AfterSuite(func() {
	err := clientSet.CoreV1().Secrets("default").Delete(context.TODO(), "regcred", metav1.DeleteOptions{})
	Expect(err).NotTo(HaveOccurred())
})

//go:generate counterfeiter -o fakes/chaincode_server.go --fake-name ChaincodeSupportServer . chaincodeSupportServer
type chaincodeSupportServer interface{ pb.ChaincodeSupportServer }

func restClientConfig(configPath string) (*rest.Config, error) {
	if configPath == "" {
		return rest.InClusterConfig()
	}
	return clientcmd.BuildConfigFromFlags("", configPath)
}

func k8sConfigPath() string {
	// check the environment variable
	if kubeconfig, ok := os.LookupEnv("KUBECONFIG"); ok {
		return kubeconfig
	}

	// check the home directory
	if home, err := os.UserHomeDir(); err == nil {
		config := filepath.Join(home, ".kube", "config")
		if _, err := os.Stat(config); err == nil {
			return config
		}
	}

	return ""
}
