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
	xpcontroller "github.com/crossplane/crossplane-runtime/v2/pkg/controller"
	"github.com/rossigee/provider-docker/internal/controller/compose"
	"github.com/rossigee/provider-docker/internal/controller/container"
	"github.com/rossigee/provider-docker/internal/controller/network"
	"github.com/rossigee/provider-docker/internal/controller/providerconfig"
	"github.com/rossigee/provider-docker/internal/controller/volume"
	ctrl "sigs.k8s.io/controller-runtime"
)

// Setup Docker controllers with the manager.
func Setup(mgr ctrl.Manager, o xpcontroller.Options) error {
	if err := providerconfig.Setup(mgr); err != nil {
		return err
	}

	// Setup container controller (v2 namespaced)
	if err := container.Setup(mgr, o); err != nil {
		return err
	}

	// Setup compose controllers (v2 namespaced)
	if err := compose.Setup(mgr, o); err != nil {
		return err
	}

	// Setup volume controllers (v2 namespaced)
	if err := volume.SetupVolume(mgr, o); err != nil {
		return err
	}

	// Setup network controllers (v2 namespaced)
	if err := network.SetupNetwork(mgr, o); err != nil {
		return err
	}

	return nil
}
