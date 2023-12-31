//go:build !ignore_autogenerated
// +build !ignore_autogenerated

/*
Copyright 2023.

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

// Code generated by controller-gen. DO NOT EDIT.

package v1beta1

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDB) DeepCopyInto(out *QuestDB) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDB.
func (in *QuestDB) DeepCopy() *QuestDB {
	if in == nil {
		return nil
	}
	out := new(QuestDB)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *QuestDB) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBConfigSpec) DeepCopyInto(out *QuestDBConfigSpec) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBConfigSpec.
func (in *QuestDBConfigSpec) DeepCopy() *QuestDBConfigSpec {
	if in == nil {
		return nil
	}
	out := new(QuestDBConfigSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBEndpointStatus) DeepCopyInto(out *QuestDBEndpointStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBEndpointStatus.
func (in *QuestDBEndpointStatus) DeepCopy() *QuestDBEndpointStatus {
	if in == nil {
		return nil
	}
	out := new(QuestDBEndpointStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBList) DeepCopyInto(out *QuestDBList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]QuestDB, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBList.
func (in *QuestDBList) DeepCopy() *QuestDBList {
	if in == nil {
		return nil
	}
	out := new(QuestDBList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *QuestDBList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBSnapshot) DeepCopyInto(out *QuestDBSnapshot) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	out.Status = in.Status
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBSnapshot.
func (in *QuestDBSnapshot) DeepCopy() *QuestDBSnapshot {
	if in == nil {
		return nil
	}
	out := new(QuestDBSnapshot)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *QuestDBSnapshot) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBSnapshotList) DeepCopyInto(out *QuestDBSnapshotList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]QuestDBSnapshot, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBSnapshotList.
func (in *QuestDBSnapshotList) DeepCopy() *QuestDBSnapshotList {
	if in == nil {
		return nil
	}
	out := new(QuestDBSnapshotList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *QuestDBSnapshotList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBSnapshotSchedule) DeepCopyInto(out *QuestDBSnapshotSchedule) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBSnapshotSchedule.
func (in *QuestDBSnapshotSchedule) DeepCopy() *QuestDBSnapshotSchedule {
	if in == nil {
		return nil
	}
	out := new(QuestDBSnapshotSchedule)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *QuestDBSnapshotSchedule) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBSnapshotScheduleList) DeepCopyInto(out *QuestDBSnapshotScheduleList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]QuestDBSnapshotSchedule, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBSnapshotScheduleList.
func (in *QuestDBSnapshotScheduleList) DeepCopy() *QuestDBSnapshotScheduleList {
	if in == nil {
		return nil
	}
	out := new(QuestDBSnapshotScheduleList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyObject is an autogenerated deepcopy function, copying the receiver, creating a new runtime.Object.
func (in *QuestDBSnapshotScheduleList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBSnapshotScheduleSpec) DeepCopyInto(out *QuestDBSnapshotScheduleSpec) {
	*out = *in
	in.Snapshot.DeepCopyInto(&out.Snapshot)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBSnapshotScheduleSpec.
func (in *QuestDBSnapshotScheduleSpec) DeepCopy() *QuestDBSnapshotScheduleSpec {
	if in == nil {
		return nil
	}
	out := new(QuestDBSnapshotScheduleSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBSnapshotScheduleStatus) DeepCopyInto(out *QuestDBSnapshotScheduleStatus) {
	*out = *in
	in.LastSnapshot.DeepCopyInto(&out.LastSnapshot)
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBSnapshotScheduleStatus.
func (in *QuestDBSnapshotScheduleStatus) DeepCopy() *QuestDBSnapshotScheduleStatus {
	if in == nil {
		return nil
	}
	out := new(QuestDBSnapshotScheduleStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBSnapshotSpec) DeepCopyInto(out *QuestDBSnapshotSpec) {
	*out = *in
	if in.VolumeSnapshotClassName != nil {
		in, out := &in.VolumeSnapshotClassName, &out.VolumeSnapshotClassName
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBSnapshotSpec.
func (in *QuestDBSnapshotSpec) DeepCopy() *QuestDBSnapshotSpec {
	if in == nil {
		return nil
	}
	out := new(QuestDBSnapshotSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBSnapshotStatus) DeepCopyInto(out *QuestDBSnapshotStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBSnapshotStatus.
func (in *QuestDBSnapshotStatus) DeepCopy() *QuestDBSnapshotStatus {
	if in == nil {
		return nil
	}
	out := new(QuestDBSnapshotStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBSpec) DeepCopyInto(out *QuestDBSpec) {
	*out = *in
	in.Volume.DeepCopyInto(&out.Volume)
	out.Config = in.Config
	if in.Affinity != nil {
		in, out := &in.Affinity, &out.Affinity
		*out = new(corev1.Affinity)
		(*in).DeepCopyInto(*out)
	}
	if in.ExtraEnv != nil {
		in, out := &in.ExtraEnv, &out.ExtraEnv
		*out = make([]corev1.EnvVar, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ExtraVolumeMounts != nil {
		in, out := &in.ExtraVolumeMounts, &out.ExtraVolumeMounts
		*out = make([]corev1.VolumeMount, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ExtraVolumes != nil {
		in, out := &in.ExtraVolumes, &out.ExtraVolumes
		*out = make([]corev1.Volume, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.ImagePullSecrets != nil {
		in, out := &in.ImagePullSecrets, &out.ImagePullSecrets
		*out = make([]corev1.LocalObjectReference, len(*in))
		copy(*out, *in)
	}
	if in.NodeSelector != nil {
		in, out := &in.NodeSelector, &out.NodeSelector
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.PodAnnotations != nil {
		in, out := &in.PodAnnotations, &out.PodAnnotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	in.PodSecurityContext.DeepCopyInto(&out.PodSecurityContext)
	in.Resources.DeepCopyInto(&out.Resources)
	if in.StatefulSetAnnotations != nil {
		in, out := &in.StatefulSetAnnotations, &out.StatefulSetAnnotations
		*out = make(map[string]string, len(*in))
		for key, val := range *in {
			(*out)[key] = val
		}
	}
	if in.Tolerations != nil {
		in, out := &in.Tolerations, &out.Tolerations
		*out = make([]corev1.Toleration, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBSpec.
func (in *QuestDBSpec) DeepCopy() *QuestDBSpec {
	if in == nil {
		return nil
	}
	out := new(QuestDBSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBStatus) DeepCopyInto(out *QuestDBStatus) {
	*out = *in
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBStatus.
func (in *QuestDBStatus) DeepCopy() *QuestDBStatus {
	if in == nil {
		return nil
	}
	out := new(QuestDBStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto is an autogenerated deepcopy function, copying the receiver, writing into out. in must be non-nil.
func (in *QuestDBVolumeSpec) DeepCopyInto(out *QuestDBVolumeSpec) {
	*out = *in
	if in.Selector != nil {
		in, out := &in.Selector, &out.Selector
		*out = new(v1.LabelSelector)
		(*in).DeepCopyInto(*out)
	}
	out.Size = in.Size.DeepCopy()
	if in.StorageClassName != nil {
		in, out := &in.StorageClassName, &out.StorageClassName
		*out = new(string)
		**out = **in
	}
}

// DeepCopy is an autogenerated deepcopy function, copying the receiver, creating a new QuestDBVolumeSpec.
func (in *QuestDBVolumeSpec) DeepCopy() *QuestDBVolumeSpec {
	if in == nil {
		return nil
	}
	out := new(QuestDBVolumeSpec)
	in.DeepCopyInto(out)
	return out
}
