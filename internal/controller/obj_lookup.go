/*
Copyright 2024 monkale.io.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// Look up for object by resource name + name + namespace. Updates context.
func getObjFromK8s(ctx context.Context, cl client.Client, obj types.NamespacedName, resource client.Object) error {
	if err := cl.Get(ctx, obj, resource); err != nil {
		return fmt.Errorf("failed to get resource %s/%s: %v", obj.Name, obj.Namespace, err)
	}
	return nil
}

// isStatefulSetReady checks if the StatefulSet is ready
func isStatefulSetReady(sts *appsv1.StatefulSet) bool {
	return sts.Status.ReadyReplicas == *sts.Spec.Replicas
}

// isDeploymentReady checks if the Deployment is ready
func isDeploymentReady(deploy *appsv1.Deployment) bool {
	return deploy.Status.ReadyReplicas == *deploy.Spec.Replicas
}

// isDaemonSetReady checks if the DaemonSet is ready
func isDaemonSetReady(ds *appsv1.DaemonSet) bool {
	return ds.Status.NumberReady == ds.Status.DesiredNumberScheduled
}
