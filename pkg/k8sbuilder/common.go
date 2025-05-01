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

import v1 "k8s.io/api/core/v1"

func getSecContext() v1.SecurityContext {

	user := int64(7051)
	boolfalse := false
	capabilities := v1.Capabilities{
		Add: []v1.Capability{
			v1.Capability("NET_BIND_SERVICE"),
		},
		Drop: []v1.Capability{
			v1.Capability("ALL"),
		},
	}
	return v1.SecurityContext{
		RunAsGroup:               &user,
		RunAsUser:                &user,
		Privileged:               &boolfalse,
		AllowPrivilegeEscalation: &boolfalse,
		Capabilities:             &capabilities,
		SeccompProfile:           &v1.SeccompProfile{Type: v1.SeccompProfileTypeRuntimeDefault},
	}
}
