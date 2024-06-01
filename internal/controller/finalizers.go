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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Look up for k8s object, and add finalizer.
func addFinalizer(ctx context.Context, cl client.Client, obj types.NamespacedName, resource client.Object, finalizerName string) error {
	// update resource by returned object from GetObjFromK8s
	if err := getObjFromK8s(ctx, cl, obj, resource); err != nil {
		return fmt.Errorf("failed to get resource %s/%s: %v", obj.Name, obj.Namespace, err)
	}
	if !controllerutil.ContainsFinalizer(resource, finalizerName) {
		// Add finalizer to object
		controllerutil.AddFinalizer(resource, finalizerName)
		if err := cl.Update(ctx, resource); err != nil {
			return fmt.Errorf("failed to get resource %s/%s with finalizer '%s': %v", obj.Name, obj.Namespace, finalizerName, err)
		}
	}
	return nil
}

// Look up for k8s object, and add finalizer.
func removeFinalizer(ctx context.Context, cl client.Client, obj types.NamespacedName, resource client.Object, finalizerName string) error {
	// update resource by returned object from GetObjFromK8s
	if err := getObjFromK8s(ctx, cl, obj, resource); err != nil {
		return fmt.Errorf("failed to get resource %s/%s: %v", obj.Name, obj.Namespace, err)
	}

	if controllerutil.ContainsFinalizer(resource, finalizerName) {
		// Add finalizer to object
		controllerutil.RemoveFinalizer(resource, finalizerName)
		if err := cl.Update(ctx, resource); err != nil {
			return fmt.Errorf("failed to get resource %s/%s with finalizer '%s': %v", obj.Name, obj.Namespace, finalizerName, err)
		}
	}
	return nil
}
