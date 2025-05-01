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

package pod

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
)

//go:generate counterfeiter -o mocks/pod.go -fake-name PodInterface k8s.io/client-go/kubernetes/typed/core/v1.PodInterface

type Handler struct {
	PodClient corev1.PodInterface
}

func NewHandler(podClient corev1.PodInterface) *Handler {
	return &Handler{
		PodClient: podClient,
	}
}

func (h *Handler) ListCompletedExecutionPodsForPeer(peerID string) ([]v1.Pod, error) {
	listOpts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("peer-id=%s", peerID),
	}

	podList, err := h.PodClient.List(context.TODO(), listOpts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get pods")
	}
	pods := podList.Items

	completedPods := []v1.Pod{}
	for _, pod := range pods {
		podStatus := pod.Status

		// If no containers found, this is not a valid pod, skip
		// and continue to next pod
		if len(podStatus.ContainerStatuses) == 0 {
			continue
		}

		// Execution chaincode pod should only have a single container, which is
		// the chaincode run container
		runContainerStatus := podStatus.ContainerStatuses[0]
		if runContainerStatus.State.Terminated != nil {
			terminatedState := runContainerStatus.State.Terminated
			if terminatedState.Reason == "Completed" {
				completedPods = append(completedPods, pod)
			}
		}
	}

	return completedPods, nil
}

func (h *Handler) DeletePods(pods []v1.Pod, opts metav1.DeleteOptions) error {
	for _, pod := range pods {
		err := h.DeletePod(pod.Name, opts)
		if err != nil {
			return errors.Wrapf(err, "failed to delete pod '%s'", pod.Name)
		}
	}
	return nil
}

func (h *Handler) DeletePod(name string, opts metav1.DeleteOptions) error {
	return h.PodClient.Delete(context.TODO(), name, opts)
}
