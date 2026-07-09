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

package v1alpha1

import (
	"reflect"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Volume type metadata.
var (
	VolumeKind             = reflect.TypeOf(Volume{}).Name()
	VolumeGroupKind        = schema.GroupKind{Group: Group, Kind: VolumeKind}
	VolumeKindAPIVersion   = VolumeKind + "." + SchemeGroupVersion.String()
	VolumeGroupVersionKind = SchemeGroupVersion.WithKind(VolumeKind)
)

// VolumeUsageData type metadata.
var (
	VolumeUsageDataKind             = reflect.TypeOf(VolumeUsageData{}).Name()
	VolumeUsageDataGroupKind        = schema.GroupKind{Group: Group, Kind: VolumeUsageDataKind}
	VolumeUsageDataKindAPIVersion   = VolumeUsageDataKind + "." + SchemeGroupVersion.String()
	VolumeUsageDataGroupVersionKind = SchemeGroupVersion.WithKind(VolumeUsageDataKind)
)
