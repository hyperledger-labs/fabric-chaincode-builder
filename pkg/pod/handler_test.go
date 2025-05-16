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

package pod_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/pod"
	"github.com/hyperledger-labs/fabric-chaincode-builder/pkg/pod/mocks"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("Handler", func() {
	var (
		mockPodClient *mocks.PodInterface
		handler       *pod.Handler
	)

	BeforeEach(func() {
		mockPodClient = &mocks.PodInterface{}
		handler = pod.NewHandler(mockPodClient)
	})

	Context("List completed pods", func() {
		BeforeEach(func() {
			mockPodClient.ListReturns(&v1.PodList{
				Items: []v1.Pod{
					v1.Pod{
						Status: v1.PodStatus{
							ContainerStatuses: []v1.ContainerStatus{
								v1.ContainerStatus{
									State: v1.ContainerState{
										Terminated: &v1.ContainerStateTerminated{
											Reason: "Completed",
										},
									},
								},
							},
						},
					}, v1.Pod{},
				},
			}, nil)
		})

		It("returns error if unable to get pods", func() {
			mockPodClient.ListReturns(nil, errors.New("list pods error"))
			_, err := handler.ListCompletedExecutionPodsForPeer("Peer1")
			Expect(err).To(HaveOccurred())
			Expect(err).Should(MatchError("failed to get pods: list pods error"))
		})

		It("returns a list of completed pods", func() {
			pods, err := handler.ListCompletedExecutionPodsForPeer("Peer1")
			Expect(err).NotTo(HaveOccurred())
			Expect(len(pods)).To(Equal(1))
		})
	})

	Context("Delete pods", func() {
		It("returns error if unable to delete pods", func() {
			mockPodClient.DeleteReturns(errors.New("delete pod error"))

			err := handler.DeletePods([]v1.Pod{
				v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod1",
					},
				},
			}, metav1.DeleteOptions{})

			Expect(err).To(HaveOccurred())
			Expect(err).Should(MatchError("failed to delete pod 'pod1': delete pod error"))
		})

		It("returns no error on success", func() {
			err := handler.DeletePods([]v1.Pod{
				v1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name: "pod1",
					},
				},
			}, metav1.DeleteOptions{})

			Expect(err).NotTo(HaveOccurred())
		})
	})
})
