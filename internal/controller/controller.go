/*
Copyright 2025 The Crossplane Authors.

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

// Package controller contains controller implementations for Docker resources.
package controller

import (
	ctrl "sigs.k8s.io/controller-runtime"

	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"

	"github.com/rossigee/provider-docker/internal/controller/compose"
	"github.com/rossigee/provider-docker/internal/controller/container"
	"github.com/rossigee/provider-docker/internal/controller/network"
	"github.com/rossigee/provider-docker/internal/controller/volume"
)

// Setup Docker controllers with the manager.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	// Setup v1alpha1 container controller (cluster-scoped for backwards compatibility)
	if err := container.Setup(mgr, o); err != nil {
		return err
	}

	// Setup v1beta1 container controller (namespaced for v2 compatibility)
	if err := container.SetupV1Beta1(mgr, o); err != nil {
		return err
	}

	// Setup compose controllers (v1alpha1 only for now)
	if err := compose.Setup(mgr, o); err != nil {
		return err
	}

	// Setup volume controllers (v1alpha1 cluster-scoped)
	if err := volume.SetupVolume(mgr, o); err != nil {
		return err
	}

	// Setup network controllers (v1alpha1 cluster-scoped)
	if err := network.SetupNetwork(mgr, o); err != nil {
		return err
	}

	return nil
}
