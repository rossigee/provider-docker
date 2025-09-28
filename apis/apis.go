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

// Package apis contains the API groups for Docker resources.
package apis

import (
	"k8s.io/apimachinery/pkg/runtime"

	composev1alpha1 "github.com/rossigee/provider-docker/apis/compose/v1alpha1"
	composev1beta1 "github.com/rossigee/provider-docker/apis/compose/v1beta1"
	containerv1alpha1 "github.com/rossigee/provider-docker/apis/container/v1alpha1"
	containerv1beta1 "github.com/rossigee/provider-docker/apis/container/v1beta1"
	networkv1alpha1 "github.com/rossigee/provider-docker/apis/network/v1alpha1"
	networkv1beta1 "github.com/rossigee/provider-docker/apis/network/v1beta1"
	"github.com/rossigee/provider-docker/apis/v1beta1"
	volumev1alpha1 "github.com/rossigee/provider-docker/apis/volume/v1alpha1"
	volumev1beta1 "github.com/rossigee/provider-docker/apis/volume/v1beta1"
)

func init() {
	// Register the types with the Scheme so the components can map objects to GroupVersionKinds and back
	AddToSchemes = append(AddToSchemes,
		v1beta1.SchemeBuilder.AddToScheme,
		containerv1alpha1.SchemeBuilder.AddToScheme,
		containerv1beta1.SchemeBuilder.AddToScheme,
		composev1alpha1.SchemeBuilder.AddToScheme,
		composev1beta1.SchemeBuilder.AddToScheme,
		volumev1alpha1.SchemeBuilder.AddToScheme,
		volumev1beta1.SchemeBuilder.AddToScheme,
		networkv1alpha1.SchemeBuilder.AddToScheme,
		networkv1beta1.SchemeBuilder.AddToScheme,
	)
}

// AddToSchemes may be used to add all resources defined in the project to a Scheme
var AddToSchemes runtime.SchemeBuilder

// AddToScheme adds all Resources to the Scheme
func AddToScheme(s *runtime.Scheme) error {
	return AddToSchemes.AddToScheme(s)
}
