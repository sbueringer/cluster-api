//go:build !ignore_autogenerated_kubeadm_bootstrap_v1alpha3
// +build !ignore_autogenerated_kubeadm_bootstrap_v1alpha3

/*
Copyright The Kubernetes Authors.

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

// Code generated by conversion-gen. DO NOT EDIT.

package v1alpha3

import (
	unsafe "unsafe"

	conversion "k8s.io/apimachinery/pkg/conversion"
	runtime "k8s.io/apimachinery/pkg/runtime"
	apiv1alpha3 "sigs.k8s.io/cluster-api/api/v1alpha3"
	apiv1alpha4 "sigs.k8s.io/cluster-api/api/v1alpha4"
	v1alpha4 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/api/v1alpha4"
	v1beta1 "sigs.k8s.io/cluster-api/bootstrap/kubeadm/types/v1beta1"
)

func init() {
	localSchemeBuilder.Register(RegisterConversions)
}

// RegisterConversions adds conversion functions to the given scheme.
// Public to allow building arbitrary schemes.
func RegisterConversions(s *runtime.Scheme) error {
	if err := s.AddGeneratedConversionFunc((*DiskSetup)(nil), (*v1alpha4.DiskSetup)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_DiskSetup_To_v1alpha4_DiskSetup(a.(*DiskSetup), b.(*v1alpha4.DiskSetup), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.DiskSetup)(nil), (*DiskSetup)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_DiskSetup_To_v1alpha3_DiskSetup(a.(*v1alpha4.DiskSetup), b.(*DiskSetup), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*File)(nil), (*v1alpha4.File)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_File_To_v1alpha4_File(a.(*File), b.(*v1alpha4.File), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.File)(nil), (*File)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_File_To_v1alpha3_File(a.(*v1alpha4.File), b.(*File), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*FileSource)(nil), (*v1alpha4.FileSource)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_FileSource_To_v1alpha4_FileSource(a.(*FileSource), b.(*v1alpha4.FileSource), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.FileSource)(nil), (*FileSource)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_FileSource_To_v1alpha3_FileSource(a.(*v1alpha4.FileSource), b.(*FileSource), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*Filesystem)(nil), (*v1alpha4.Filesystem)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_Filesystem_To_v1alpha4_Filesystem(a.(*Filesystem), b.(*v1alpha4.Filesystem), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.Filesystem)(nil), (*Filesystem)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_Filesystem_To_v1alpha3_Filesystem(a.(*v1alpha4.Filesystem), b.(*Filesystem), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*KubeadmConfig)(nil), (*v1alpha4.KubeadmConfig)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_KubeadmConfig_To_v1alpha4_KubeadmConfig(a.(*KubeadmConfig), b.(*v1alpha4.KubeadmConfig), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.KubeadmConfig)(nil), (*KubeadmConfig)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_KubeadmConfig_To_v1alpha3_KubeadmConfig(a.(*v1alpha4.KubeadmConfig), b.(*KubeadmConfig), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*KubeadmConfigList)(nil), (*v1alpha4.KubeadmConfigList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_KubeadmConfigList_To_v1alpha4_KubeadmConfigList(a.(*KubeadmConfigList), b.(*v1alpha4.KubeadmConfigList), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.KubeadmConfigList)(nil), (*KubeadmConfigList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_KubeadmConfigList_To_v1alpha3_KubeadmConfigList(a.(*v1alpha4.KubeadmConfigList), b.(*KubeadmConfigList), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*KubeadmConfigSpec)(nil), (*v1alpha4.KubeadmConfigSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_KubeadmConfigSpec_To_v1alpha4_KubeadmConfigSpec(a.(*KubeadmConfigSpec), b.(*v1alpha4.KubeadmConfigSpec), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.KubeadmConfigSpec)(nil), (*KubeadmConfigSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_KubeadmConfigSpec_To_v1alpha3_KubeadmConfigSpec(a.(*v1alpha4.KubeadmConfigSpec), b.(*KubeadmConfigSpec), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.KubeadmConfigStatus)(nil), (*KubeadmConfigStatus)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_KubeadmConfigStatus_To_v1alpha3_KubeadmConfigStatus(a.(*v1alpha4.KubeadmConfigStatus), b.(*KubeadmConfigStatus), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*KubeadmConfigTemplate)(nil), (*v1alpha4.KubeadmConfigTemplate)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_KubeadmConfigTemplate_To_v1alpha4_KubeadmConfigTemplate(a.(*KubeadmConfigTemplate), b.(*v1alpha4.KubeadmConfigTemplate), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.KubeadmConfigTemplate)(nil), (*KubeadmConfigTemplate)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_KubeadmConfigTemplate_To_v1alpha3_KubeadmConfigTemplate(a.(*v1alpha4.KubeadmConfigTemplate), b.(*KubeadmConfigTemplate), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*KubeadmConfigTemplateList)(nil), (*v1alpha4.KubeadmConfigTemplateList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_KubeadmConfigTemplateList_To_v1alpha4_KubeadmConfigTemplateList(a.(*KubeadmConfigTemplateList), b.(*v1alpha4.KubeadmConfigTemplateList), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.KubeadmConfigTemplateList)(nil), (*KubeadmConfigTemplateList)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_KubeadmConfigTemplateList_To_v1alpha3_KubeadmConfigTemplateList(a.(*v1alpha4.KubeadmConfigTemplateList), b.(*KubeadmConfigTemplateList), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*KubeadmConfigTemplateResource)(nil), (*v1alpha4.KubeadmConfigTemplateResource)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_KubeadmConfigTemplateResource_To_v1alpha4_KubeadmConfigTemplateResource(a.(*KubeadmConfigTemplateResource), b.(*v1alpha4.KubeadmConfigTemplateResource), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.KubeadmConfigTemplateResource)(nil), (*KubeadmConfigTemplateResource)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_KubeadmConfigTemplateResource_To_v1alpha3_KubeadmConfigTemplateResource(a.(*v1alpha4.KubeadmConfigTemplateResource), b.(*KubeadmConfigTemplateResource), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*KubeadmConfigTemplateSpec)(nil), (*v1alpha4.KubeadmConfigTemplateSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_KubeadmConfigTemplateSpec_To_v1alpha4_KubeadmConfigTemplateSpec(a.(*KubeadmConfigTemplateSpec), b.(*v1alpha4.KubeadmConfigTemplateSpec), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.KubeadmConfigTemplateSpec)(nil), (*KubeadmConfigTemplateSpec)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_KubeadmConfigTemplateSpec_To_v1alpha3_KubeadmConfigTemplateSpec(a.(*v1alpha4.KubeadmConfigTemplateSpec), b.(*KubeadmConfigTemplateSpec), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*NTP)(nil), (*v1alpha4.NTP)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_NTP_To_v1alpha4_NTP(a.(*NTP), b.(*v1alpha4.NTP), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.NTP)(nil), (*NTP)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_NTP_To_v1alpha3_NTP(a.(*v1alpha4.NTP), b.(*NTP), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*Partition)(nil), (*v1alpha4.Partition)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_Partition_To_v1alpha4_Partition(a.(*Partition), b.(*v1alpha4.Partition), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.Partition)(nil), (*Partition)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_Partition_To_v1alpha3_Partition(a.(*v1alpha4.Partition), b.(*Partition), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*SecretFileSource)(nil), (*v1alpha4.SecretFileSource)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_SecretFileSource_To_v1alpha4_SecretFileSource(a.(*SecretFileSource), b.(*v1alpha4.SecretFileSource), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.SecretFileSource)(nil), (*SecretFileSource)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_SecretFileSource_To_v1alpha3_SecretFileSource(a.(*v1alpha4.SecretFileSource), b.(*SecretFileSource), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*User)(nil), (*v1alpha4.User)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_User_To_v1alpha4_User(a.(*User), b.(*v1alpha4.User), scope)
	}); err != nil {
		return err
	}
	if err := s.AddGeneratedConversionFunc((*v1alpha4.User)(nil), (*User)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_User_To_v1alpha3_User(a.(*v1alpha4.User), b.(*User), scope)
	}); err != nil {
		return err
	}
	if err := s.AddConversionFunc((*KubeadmConfigStatus)(nil), (*v1alpha4.KubeadmConfigStatus)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha3_KubeadmConfigStatus_To_v1alpha4_KubeadmConfigStatus(a.(*KubeadmConfigStatus), b.(*v1alpha4.KubeadmConfigStatus), scope)
	}); err != nil {
		return err
	}
	if err := s.AddConversionFunc((*v1alpha4.ClusterConfiguration)(nil), (*v1beta1.ClusterConfiguration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1alpha4_ClusterConfiguration_To_v1beta1_ClusterConfiguration(a.(*v1alpha4.ClusterConfiguration), b.(*v1beta1.ClusterConfiguration), scope)
	}); err != nil {
		return err
	}
	if err := s.AddConversionFunc((*v1beta1.ClusterConfiguration)(nil), (*v1alpha4.ClusterConfiguration)(nil), func(a, b interface{}, scope conversion.Scope) error {
		return Convert_v1beta1_ClusterConfiguration_To_v1alpha4_ClusterConfiguration(a.(*v1beta1.ClusterConfiguration), b.(*v1alpha4.ClusterConfiguration), scope)
	}); err != nil {
		return err
	}
	return nil
}

func autoConvert_v1alpha3_DiskSetup_To_v1alpha4_DiskSetup(in *DiskSetup, out *v1alpha4.DiskSetup, s conversion.Scope) error {
	out.Partitions = *(*[]v1alpha4.Partition)(unsafe.Pointer(&in.Partitions))
	out.Filesystems = *(*[]v1alpha4.Filesystem)(unsafe.Pointer(&in.Filesystems))
	return nil
}

// Convert_v1alpha3_DiskSetup_To_v1alpha4_DiskSetup is an autogenerated conversion function.
func Convert_v1alpha3_DiskSetup_To_v1alpha4_DiskSetup(in *DiskSetup, out *v1alpha4.DiskSetup, s conversion.Scope) error {
	return autoConvert_v1alpha3_DiskSetup_To_v1alpha4_DiskSetup(in, out, s)
}

func autoConvert_v1alpha4_DiskSetup_To_v1alpha3_DiskSetup(in *v1alpha4.DiskSetup, out *DiskSetup, s conversion.Scope) error {
	out.Partitions = *(*[]Partition)(unsafe.Pointer(&in.Partitions))
	out.Filesystems = *(*[]Filesystem)(unsafe.Pointer(&in.Filesystems))
	return nil
}

// Convert_v1alpha4_DiskSetup_To_v1alpha3_DiskSetup is an autogenerated conversion function.
func Convert_v1alpha4_DiskSetup_To_v1alpha3_DiskSetup(in *v1alpha4.DiskSetup, out *DiskSetup, s conversion.Scope) error {
	return autoConvert_v1alpha4_DiskSetup_To_v1alpha3_DiskSetup(in, out, s)
}

func autoConvert_v1alpha3_File_To_v1alpha4_File(in *File, out *v1alpha4.File, s conversion.Scope) error {
	out.Path = in.Path
	out.Owner = in.Owner
	out.Permissions = in.Permissions
	out.Encoding = v1alpha4.Encoding(in.Encoding)
	out.Content = in.Content
	out.ContentFrom = (*v1alpha4.FileSource)(unsafe.Pointer(in.ContentFrom))
	return nil
}

// Convert_v1alpha3_File_To_v1alpha4_File is an autogenerated conversion function.
func Convert_v1alpha3_File_To_v1alpha4_File(in *File, out *v1alpha4.File, s conversion.Scope) error {
	return autoConvert_v1alpha3_File_To_v1alpha4_File(in, out, s)
}

func autoConvert_v1alpha4_File_To_v1alpha3_File(in *v1alpha4.File, out *File, s conversion.Scope) error {
	out.Path = in.Path
	out.Owner = in.Owner
	out.Permissions = in.Permissions
	out.Encoding = Encoding(in.Encoding)
	out.Content = in.Content
	out.ContentFrom = (*FileSource)(unsafe.Pointer(in.ContentFrom))
	return nil
}

// Convert_v1alpha4_File_To_v1alpha3_File is an autogenerated conversion function.
func Convert_v1alpha4_File_To_v1alpha3_File(in *v1alpha4.File, out *File, s conversion.Scope) error {
	return autoConvert_v1alpha4_File_To_v1alpha3_File(in, out, s)
}

func autoConvert_v1alpha3_FileSource_To_v1alpha4_FileSource(in *FileSource, out *v1alpha4.FileSource, s conversion.Scope) error {
	if err := Convert_v1alpha3_SecretFileSource_To_v1alpha4_SecretFileSource(&in.Secret, &out.Secret, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha3_FileSource_To_v1alpha4_FileSource is an autogenerated conversion function.
func Convert_v1alpha3_FileSource_To_v1alpha4_FileSource(in *FileSource, out *v1alpha4.FileSource, s conversion.Scope) error {
	return autoConvert_v1alpha3_FileSource_To_v1alpha4_FileSource(in, out, s)
}

func autoConvert_v1alpha4_FileSource_To_v1alpha3_FileSource(in *v1alpha4.FileSource, out *FileSource, s conversion.Scope) error {
	if err := Convert_v1alpha4_SecretFileSource_To_v1alpha3_SecretFileSource(&in.Secret, &out.Secret, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha4_FileSource_To_v1alpha3_FileSource is an autogenerated conversion function.
func Convert_v1alpha4_FileSource_To_v1alpha3_FileSource(in *v1alpha4.FileSource, out *FileSource, s conversion.Scope) error {
	return autoConvert_v1alpha4_FileSource_To_v1alpha3_FileSource(in, out, s)
}

func autoConvert_v1alpha3_Filesystem_To_v1alpha4_Filesystem(in *Filesystem, out *v1alpha4.Filesystem, s conversion.Scope) error {
	out.Device = in.Device
	out.Filesystem = in.Filesystem
	out.Label = in.Label
	out.Partition = (*string)(unsafe.Pointer(in.Partition))
	out.Overwrite = (*bool)(unsafe.Pointer(in.Overwrite))
	out.ReplaceFS = (*string)(unsafe.Pointer(in.ReplaceFS))
	out.ExtraOpts = *(*[]string)(unsafe.Pointer(&in.ExtraOpts))
	return nil
}

// Convert_v1alpha3_Filesystem_To_v1alpha4_Filesystem is an autogenerated conversion function.
func Convert_v1alpha3_Filesystem_To_v1alpha4_Filesystem(in *Filesystem, out *v1alpha4.Filesystem, s conversion.Scope) error {
	return autoConvert_v1alpha3_Filesystem_To_v1alpha4_Filesystem(in, out, s)
}

func autoConvert_v1alpha4_Filesystem_To_v1alpha3_Filesystem(in *v1alpha4.Filesystem, out *Filesystem, s conversion.Scope) error {
	out.Device = in.Device
	out.Filesystem = in.Filesystem
	out.Label = in.Label
	out.Partition = (*string)(unsafe.Pointer(in.Partition))
	out.Overwrite = (*bool)(unsafe.Pointer(in.Overwrite))
	out.ReplaceFS = (*string)(unsafe.Pointer(in.ReplaceFS))
	out.ExtraOpts = *(*[]string)(unsafe.Pointer(&in.ExtraOpts))
	return nil
}

// Convert_v1alpha4_Filesystem_To_v1alpha3_Filesystem is an autogenerated conversion function.
func Convert_v1alpha4_Filesystem_To_v1alpha3_Filesystem(in *v1alpha4.Filesystem, out *Filesystem, s conversion.Scope) error {
	return autoConvert_v1alpha4_Filesystem_To_v1alpha3_Filesystem(in, out, s)
}

func autoConvert_v1alpha3_KubeadmConfig_To_v1alpha4_KubeadmConfig(in *KubeadmConfig, out *v1alpha4.KubeadmConfig, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_v1alpha3_KubeadmConfigSpec_To_v1alpha4_KubeadmConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1alpha3_KubeadmConfigStatus_To_v1alpha4_KubeadmConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha3_KubeadmConfig_To_v1alpha4_KubeadmConfig is an autogenerated conversion function.
func Convert_v1alpha3_KubeadmConfig_To_v1alpha4_KubeadmConfig(in *KubeadmConfig, out *v1alpha4.KubeadmConfig, s conversion.Scope) error {
	return autoConvert_v1alpha3_KubeadmConfig_To_v1alpha4_KubeadmConfig(in, out, s)
}

func autoConvert_v1alpha4_KubeadmConfig_To_v1alpha3_KubeadmConfig(in *v1alpha4.KubeadmConfig, out *KubeadmConfig, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_v1alpha4_KubeadmConfigSpec_To_v1alpha3_KubeadmConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	if err := Convert_v1alpha4_KubeadmConfigStatus_To_v1alpha3_KubeadmConfigStatus(&in.Status, &out.Status, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha4_KubeadmConfig_To_v1alpha3_KubeadmConfig is an autogenerated conversion function.
func Convert_v1alpha4_KubeadmConfig_To_v1alpha3_KubeadmConfig(in *v1alpha4.KubeadmConfig, out *KubeadmConfig, s conversion.Scope) error {
	return autoConvert_v1alpha4_KubeadmConfig_To_v1alpha3_KubeadmConfig(in, out, s)
}

func autoConvert_v1alpha3_KubeadmConfigList_To_v1alpha4_KubeadmConfigList(in *KubeadmConfigList, out *v1alpha4.KubeadmConfigList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]v1alpha4.KubeadmConfig, len(*in))
		for i := range *in {
			if err := Convert_v1alpha3_KubeadmConfig_To_v1alpha4_KubeadmConfig(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

// Convert_v1alpha3_KubeadmConfigList_To_v1alpha4_KubeadmConfigList is an autogenerated conversion function.
func Convert_v1alpha3_KubeadmConfigList_To_v1alpha4_KubeadmConfigList(in *KubeadmConfigList, out *v1alpha4.KubeadmConfigList, s conversion.Scope) error {
	return autoConvert_v1alpha3_KubeadmConfigList_To_v1alpha4_KubeadmConfigList(in, out, s)
}

func autoConvert_v1alpha4_KubeadmConfigList_To_v1alpha3_KubeadmConfigList(in *v1alpha4.KubeadmConfigList, out *KubeadmConfigList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]KubeadmConfig, len(*in))
		for i := range *in {
			if err := Convert_v1alpha4_KubeadmConfig_To_v1alpha3_KubeadmConfig(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

// Convert_v1alpha4_KubeadmConfigList_To_v1alpha3_KubeadmConfigList is an autogenerated conversion function.
func Convert_v1alpha4_KubeadmConfigList_To_v1alpha3_KubeadmConfigList(in *v1alpha4.KubeadmConfigList, out *KubeadmConfigList, s conversion.Scope) error {
	return autoConvert_v1alpha4_KubeadmConfigList_To_v1alpha3_KubeadmConfigList(in, out, s)
}

func autoConvert_v1alpha3_KubeadmConfigSpec_To_v1alpha4_KubeadmConfigSpec(in *KubeadmConfigSpec, out *v1alpha4.KubeadmConfigSpec, s conversion.Scope) error {
	if in.ClusterConfiguration != nil {
		in, out := &in.ClusterConfiguration, &out.ClusterConfiguration
		*out = new(v1alpha4.ClusterConfiguration)
		if err := Convert_v1beta1_ClusterConfiguration_To_v1alpha4_ClusterConfiguration(*in, *out, s); err != nil {
			return err
		}
	} else {
		out.ClusterConfiguration = nil
	}
	out.InitConfiguration = (*v1alpha4.InitConfiguration)(unsafe.Pointer(in.InitConfiguration))
	out.JoinConfiguration = (*v1alpha4.JoinConfiguration)(unsafe.Pointer(in.JoinConfiguration))
	out.Files = *(*[]v1alpha4.File)(unsafe.Pointer(&in.Files))
	out.DiskSetup = (*v1alpha4.DiskSetup)(unsafe.Pointer(in.DiskSetup))
	out.Mounts = *(*[]v1alpha4.MountPoints)(unsafe.Pointer(&in.Mounts))
	out.PreKubeadmCommands = *(*[]string)(unsafe.Pointer(&in.PreKubeadmCommands))
	out.PostKubeadmCommands = *(*[]string)(unsafe.Pointer(&in.PostKubeadmCommands))
	out.Users = *(*[]v1alpha4.User)(unsafe.Pointer(&in.Users))
	out.NTP = (*v1alpha4.NTP)(unsafe.Pointer(in.NTP))
	out.Format = v1alpha4.Format(in.Format)
	out.Verbosity = (*int32)(unsafe.Pointer(in.Verbosity))
	out.UseExperimentalRetryJoin = in.UseExperimentalRetryJoin
	return nil
}

// Convert_v1alpha3_KubeadmConfigSpec_To_v1alpha4_KubeadmConfigSpec is an autogenerated conversion function.
func Convert_v1alpha3_KubeadmConfigSpec_To_v1alpha4_KubeadmConfigSpec(in *KubeadmConfigSpec, out *v1alpha4.KubeadmConfigSpec, s conversion.Scope) error {
	return autoConvert_v1alpha3_KubeadmConfigSpec_To_v1alpha4_KubeadmConfigSpec(in, out, s)
}

func autoConvert_v1alpha4_KubeadmConfigSpec_To_v1alpha3_KubeadmConfigSpec(in *v1alpha4.KubeadmConfigSpec, out *KubeadmConfigSpec, s conversion.Scope) error {
	if in.ClusterConfiguration != nil {
		in, out := &in.ClusterConfiguration, &out.ClusterConfiguration
		*out = new(v1beta1.ClusterConfiguration)
		if err := Convert_v1alpha4_ClusterConfiguration_To_v1beta1_ClusterConfiguration(*in, *out, s); err != nil {
			return err
		}
	} else {
		out.ClusterConfiguration = nil
	}
	out.InitConfiguration = (*v1beta1.InitConfiguration)(unsafe.Pointer(in.InitConfiguration))
	out.JoinConfiguration = (*v1beta1.JoinConfiguration)(unsafe.Pointer(in.JoinConfiguration))
	out.Files = *(*[]File)(unsafe.Pointer(&in.Files))
	out.DiskSetup = (*DiskSetup)(unsafe.Pointer(in.DiskSetup))
	out.Mounts = *(*[]MountPoints)(unsafe.Pointer(&in.Mounts))
	out.PreKubeadmCommands = *(*[]string)(unsafe.Pointer(&in.PreKubeadmCommands))
	out.PostKubeadmCommands = *(*[]string)(unsafe.Pointer(&in.PostKubeadmCommands))
	out.Users = *(*[]User)(unsafe.Pointer(&in.Users))
	out.NTP = (*NTP)(unsafe.Pointer(in.NTP))
	out.Format = Format(in.Format)
	out.Verbosity = (*int32)(unsafe.Pointer(in.Verbosity))
	out.UseExperimentalRetryJoin = in.UseExperimentalRetryJoin
	return nil
}

// Convert_v1alpha4_KubeadmConfigSpec_To_v1alpha3_KubeadmConfigSpec is an autogenerated conversion function.
func Convert_v1alpha4_KubeadmConfigSpec_To_v1alpha3_KubeadmConfigSpec(in *v1alpha4.KubeadmConfigSpec, out *KubeadmConfigSpec, s conversion.Scope) error {
	return autoConvert_v1alpha4_KubeadmConfigSpec_To_v1alpha3_KubeadmConfigSpec(in, out, s)
}

func autoConvert_v1alpha3_KubeadmConfigStatus_To_v1alpha4_KubeadmConfigStatus(in *KubeadmConfigStatus, out *v1alpha4.KubeadmConfigStatus, s conversion.Scope) error {
	out.Ready = in.Ready
	out.DataSecretName = (*string)(unsafe.Pointer(in.DataSecretName))
	// WARNING: in.BootstrapData requires manual conversion: does not exist in peer-type
	out.FailureReason = in.FailureReason
	out.FailureMessage = in.FailureMessage
	out.ObservedGeneration = in.ObservedGeneration
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(apiv1alpha4.Conditions, len(*in))
		for i := range *in {
			if err := apiv1alpha3.Convert_v1alpha3_Condition_To_v1alpha4_Condition(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Conditions = nil
	}
	return nil
}

func autoConvert_v1alpha4_KubeadmConfigStatus_To_v1alpha3_KubeadmConfigStatus(in *v1alpha4.KubeadmConfigStatus, out *KubeadmConfigStatus, s conversion.Scope) error {
	out.Ready = in.Ready
	out.DataSecretName = (*string)(unsafe.Pointer(in.DataSecretName))
	out.FailureReason = in.FailureReason
	out.FailureMessage = in.FailureMessage
	out.ObservedGeneration = in.ObservedGeneration
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make(apiv1alpha3.Conditions, len(*in))
		for i := range *in {
			if err := apiv1alpha3.Convert_v1alpha4_Condition_To_v1alpha3_Condition(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Conditions = nil
	}
	return nil
}

// Convert_v1alpha4_KubeadmConfigStatus_To_v1alpha3_KubeadmConfigStatus is an autogenerated conversion function.
func Convert_v1alpha4_KubeadmConfigStatus_To_v1alpha3_KubeadmConfigStatus(in *v1alpha4.KubeadmConfigStatus, out *KubeadmConfigStatus, s conversion.Scope) error {
	return autoConvert_v1alpha4_KubeadmConfigStatus_To_v1alpha3_KubeadmConfigStatus(in, out, s)
}

func autoConvert_v1alpha3_KubeadmConfigTemplate_To_v1alpha4_KubeadmConfigTemplate(in *KubeadmConfigTemplate, out *v1alpha4.KubeadmConfigTemplate, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_v1alpha3_KubeadmConfigTemplateSpec_To_v1alpha4_KubeadmConfigTemplateSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha3_KubeadmConfigTemplate_To_v1alpha4_KubeadmConfigTemplate is an autogenerated conversion function.
func Convert_v1alpha3_KubeadmConfigTemplate_To_v1alpha4_KubeadmConfigTemplate(in *KubeadmConfigTemplate, out *v1alpha4.KubeadmConfigTemplate, s conversion.Scope) error {
	return autoConvert_v1alpha3_KubeadmConfigTemplate_To_v1alpha4_KubeadmConfigTemplate(in, out, s)
}

func autoConvert_v1alpha4_KubeadmConfigTemplate_To_v1alpha3_KubeadmConfigTemplate(in *v1alpha4.KubeadmConfigTemplate, out *KubeadmConfigTemplate, s conversion.Scope) error {
	out.ObjectMeta = in.ObjectMeta
	if err := Convert_v1alpha4_KubeadmConfigTemplateSpec_To_v1alpha3_KubeadmConfigTemplateSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha4_KubeadmConfigTemplate_To_v1alpha3_KubeadmConfigTemplate is an autogenerated conversion function.
func Convert_v1alpha4_KubeadmConfigTemplate_To_v1alpha3_KubeadmConfigTemplate(in *v1alpha4.KubeadmConfigTemplate, out *KubeadmConfigTemplate, s conversion.Scope) error {
	return autoConvert_v1alpha4_KubeadmConfigTemplate_To_v1alpha3_KubeadmConfigTemplate(in, out, s)
}

func autoConvert_v1alpha3_KubeadmConfigTemplateList_To_v1alpha4_KubeadmConfigTemplateList(in *KubeadmConfigTemplateList, out *v1alpha4.KubeadmConfigTemplateList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]v1alpha4.KubeadmConfigTemplate, len(*in))
		for i := range *in {
			if err := Convert_v1alpha3_KubeadmConfigTemplate_To_v1alpha4_KubeadmConfigTemplate(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

// Convert_v1alpha3_KubeadmConfigTemplateList_To_v1alpha4_KubeadmConfigTemplateList is an autogenerated conversion function.
func Convert_v1alpha3_KubeadmConfigTemplateList_To_v1alpha4_KubeadmConfigTemplateList(in *KubeadmConfigTemplateList, out *v1alpha4.KubeadmConfigTemplateList, s conversion.Scope) error {
	return autoConvert_v1alpha3_KubeadmConfigTemplateList_To_v1alpha4_KubeadmConfigTemplateList(in, out, s)
}

func autoConvert_v1alpha4_KubeadmConfigTemplateList_To_v1alpha3_KubeadmConfigTemplateList(in *v1alpha4.KubeadmConfigTemplateList, out *KubeadmConfigTemplateList, s conversion.Scope) error {
	out.ListMeta = in.ListMeta
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]KubeadmConfigTemplate, len(*in))
		for i := range *in {
			if err := Convert_v1alpha4_KubeadmConfigTemplate_To_v1alpha3_KubeadmConfigTemplate(&(*in)[i], &(*out)[i], s); err != nil {
				return err
			}
		}
	} else {
		out.Items = nil
	}
	return nil
}

// Convert_v1alpha4_KubeadmConfigTemplateList_To_v1alpha3_KubeadmConfigTemplateList is an autogenerated conversion function.
func Convert_v1alpha4_KubeadmConfigTemplateList_To_v1alpha3_KubeadmConfigTemplateList(in *v1alpha4.KubeadmConfigTemplateList, out *KubeadmConfigTemplateList, s conversion.Scope) error {
	return autoConvert_v1alpha4_KubeadmConfigTemplateList_To_v1alpha3_KubeadmConfigTemplateList(in, out, s)
}

func autoConvert_v1alpha3_KubeadmConfigTemplateResource_To_v1alpha4_KubeadmConfigTemplateResource(in *KubeadmConfigTemplateResource, out *v1alpha4.KubeadmConfigTemplateResource, s conversion.Scope) error {
	if err := Convert_v1alpha3_KubeadmConfigSpec_To_v1alpha4_KubeadmConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha3_KubeadmConfigTemplateResource_To_v1alpha4_KubeadmConfigTemplateResource is an autogenerated conversion function.
func Convert_v1alpha3_KubeadmConfigTemplateResource_To_v1alpha4_KubeadmConfigTemplateResource(in *KubeadmConfigTemplateResource, out *v1alpha4.KubeadmConfigTemplateResource, s conversion.Scope) error {
	return autoConvert_v1alpha3_KubeadmConfigTemplateResource_To_v1alpha4_KubeadmConfigTemplateResource(in, out, s)
}

func autoConvert_v1alpha4_KubeadmConfigTemplateResource_To_v1alpha3_KubeadmConfigTemplateResource(in *v1alpha4.KubeadmConfigTemplateResource, out *KubeadmConfigTemplateResource, s conversion.Scope) error {
	if err := Convert_v1alpha4_KubeadmConfigSpec_To_v1alpha3_KubeadmConfigSpec(&in.Spec, &out.Spec, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha4_KubeadmConfigTemplateResource_To_v1alpha3_KubeadmConfigTemplateResource is an autogenerated conversion function.
func Convert_v1alpha4_KubeadmConfigTemplateResource_To_v1alpha3_KubeadmConfigTemplateResource(in *v1alpha4.KubeadmConfigTemplateResource, out *KubeadmConfigTemplateResource, s conversion.Scope) error {
	return autoConvert_v1alpha4_KubeadmConfigTemplateResource_To_v1alpha3_KubeadmConfigTemplateResource(in, out, s)
}

func autoConvert_v1alpha3_KubeadmConfigTemplateSpec_To_v1alpha4_KubeadmConfigTemplateSpec(in *KubeadmConfigTemplateSpec, out *v1alpha4.KubeadmConfigTemplateSpec, s conversion.Scope) error {
	if err := Convert_v1alpha3_KubeadmConfigTemplateResource_To_v1alpha4_KubeadmConfigTemplateResource(&in.Template, &out.Template, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha3_KubeadmConfigTemplateSpec_To_v1alpha4_KubeadmConfigTemplateSpec is an autogenerated conversion function.
func Convert_v1alpha3_KubeadmConfigTemplateSpec_To_v1alpha4_KubeadmConfigTemplateSpec(in *KubeadmConfigTemplateSpec, out *v1alpha4.KubeadmConfigTemplateSpec, s conversion.Scope) error {
	return autoConvert_v1alpha3_KubeadmConfigTemplateSpec_To_v1alpha4_KubeadmConfigTemplateSpec(in, out, s)
}

func autoConvert_v1alpha4_KubeadmConfigTemplateSpec_To_v1alpha3_KubeadmConfigTemplateSpec(in *v1alpha4.KubeadmConfigTemplateSpec, out *KubeadmConfigTemplateSpec, s conversion.Scope) error {
	if err := Convert_v1alpha4_KubeadmConfigTemplateResource_To_v1alpha3_KubeadmConfigTemplateResource(&in.Template, &out.Template, s); err != nil {
		return err
	}
	return nil
}

// Convert_v1alpha4_KubeadmConfigTemplateSpec_To_v1alpha3_KubeadmConfigTemplateSpec is an autogenerated conversion function.
func Convert_v1alpha4_KubeadmConfigTemplateSpec_To_v1alpha3_KubeadmConfigTemplateSpec(in *v1alpha4.KubeadmConfigTemplateSpec, out *KubeadmConfigTemplateSpec, s conversion.Scope) error {
	return autoConvert_v1alpha4_KubeadmConfigTemplateSpec_To_v1alpha3_KubeadmConfigTemplateSpec(in, out, s)
}

func autoConvert_v1alpha3_NTP_To_v1alpha4_NTP(in *NTP, out *v1alpha4.NTP, s conversion.Scope) error {
	out.Servers = *(*[]string)(unsafe.Pointer(&in.Servers))
	out.Enabled = (*bool)(unsafe.Pointer(in.Enabled))
	return nil
}

// Convert_v1alpha3_NTP_To_v1alpha4_NTP is an autogenerated conversion function.
func Convert_v1alpha3_NTP_To_v1alpha4_NTP(in *NTP, out *v1alpha4.NTP, s conversion.Scope) error {
	return autoConvert_v1alpha3_NTP_To_v1alpha4_NTP(in, out, s)
}

func autoConvert_v1alpha4_NTP_To_v1alpha3_NTP(in *v1alpha4.NTP, out *NTP, s conversion.Scope) error {
	out.Servers = *(*[]string)(unsafe.Pointer(&in.Servers))
	out.Enabled = (*bool)(unsafe.Pointer(in.Enabled))
	return nil
}

// Convert_v1alpha4_NTP_To_v1alpha3_NTP is an autogenerated conversion function.
func Convert_v1alpha4_NTP_To_v1alpha3_NTP(in *v1alpha4.NTP, out *NTP, s conversion.Scope) error {
	return autoConvert_v1alpha4_NTP_To_v1alpha3_NTP(in, out, s)
}

func autoConvert_v1alpha3_Partition_To_v1alpha4_Partition(in *Partition, out *v1alpha4.Partition, s conversion.Scope) error {
	out.Device = in.Device
	out.Layout = in.Layout
	out.Overwrite = (*bool)(unsafe.Pointer(in.Overwrite))
	out.TableType = (*string)(unsafe.Pointer(in.TableType))
	return nil
}

// Convert_v1alpha3_Partition_To_v1alpha4_Partition is an autogenerated conversion function.
func Convert_v1alpha3_Partition_To_v1alpha4_Partition(in *Partition, out *v1alpha4.Partition, s conversion.Scope) error {
	return autoConvert_v1alpha3_Partition_To_v1alpha4_Partition(in, out, s)
}

func autoConvert_v1alpha4_Partition_To_v1alpha3_Partition(in *v1alpha4.Partition, out *Partition, s conversion.Scope) error {
	out.Device = in.Device
	out.Layout = in.Layout
	out.Overwrite = (*bool)(unsafe.Pointer(in.Overwrite))
	out.TableType = (*string)(unsafe.Pointer(in.TableType))
	return nil
}

// Convert_v1alpha4_Partition_To_v1alpha3_Partition is an autogenerated conversion function.
func Convert_v1alpha4_Partition_To_v1alpha3_Partition(in *v1alpha4.Partition, out *Partition, s conversion.Scope) error {
	return autoConvert_v1alpha4_Partition_To_v1alpha3_Partition(in, out, s)
}

func autoConvert_v1alpha3_SecretFileSource_To_v1alpha4_SecretFileSource(in *SecretFileSource, out *v1alpha4.SecretFileSource, s conversion.Scope) error {
	out.Name = in.Name
	out.Key = in.Key
	return nil
}

// Convert_v1alpha3_SecretFileSource_To_v1alpha4_SecretFileSource is an autogenerated conversion function.
func Convert_v1alpha3_SecretFileSource_To_v1alpha4_SecretFileSource(in *SecretFileSource, out *v1alpha4.SecretFileSource, s conversion.Scope) error {
	return autoConvert_v1alpha3_SecretFileSource_To_v1alpha4_SecretFileSource(in, out, s)
}

func autoConvert_v1alpha4_SecretFileSource_To_v1alpha3_SecretFileSource(in *v1alpha4.SecretFileSource, out *SecretFileSource, s conversion.Scope) error {
	out.Name = in.Name
	out.Key = in.Key
	return nil
}

// Convert_v1alpha4_SecretFileSource_To_v1alpha3_SecretFileSource is an autogenerated conversion function.
func Convert_v1alpha4_SecretFileSource_To_v1alpha3_SecretFileSource(in *v1alpha4.SecretFileSource, out *SecretFileSource, s conversion.Scope) error {
	return autoConvert_v1alpha4_SecretFileSource_To_v1alpha3_SecretFileSource(in, out, s)
}

func autoConvert_v1alpha3_User_To_v1alpha4_User(in *User, out *v1alpha4.User, s conversion.Scope) error {
	out.Name = in.Name
	out.Gecos = (*string)(unsafe.Pointer(in.Gecos))
	out.Groups = (*string)(unsafe.Pointer(in.Groups))
	out.HomeDir = (*string)(unsafe.Pointer(in.HomeDir))
	out.Inactive = (*bool)(unsafe.Pointer(in.Inactive))
	out.Shell = (*string)(unsafe.Pointer(in.Shell))
	out.Passwd = (*string)(unsafe.Pointer(in.Passwd))
	out.PrimaryGroup = (*string)(unsafe.Pointer(in.PrimaryGroup))
	out.LockPassword = (*bool)(unsafe.Pointer(in.LockPassword))
	out.Sudo = (*string)(unsafe.Pointer(in.Sudo))
	out.SSHAuthorizedKeys = *(*[]string)(unsafe.Pointer(&in.SSHAuthorizedKeys))
	return nil
}

// Convert_v1alpha3_User_To_v1alpha4_User is an autogenerated conversion function.
func Convert_v1alpha3_User_To_v1alpha4_User(in *User, out *v1alpha4.User, s conversion.Scope) error {
	return autoConvert_v1alpha3_User_To_v1alpha4_User(in, out, s)
}

func autoConvert_v1alpha4_User_To_v1alpha3_User(in *v1alpha4.User, out *User, s conversion.Scope) error {
	out.Name = in.Name
	out.Gecos = (*string)(unsafe.Pointer(in.Gecos))
	out.Groups = (*string)(unsafe.Pointer(in.Groups))
	out.HomeDir = (*string)(unsafe.Pointer(in.HomeDir))
	out.Inactive = (*bool)(unsafe.Pointer(in.Inactive))
	out.Shell = (*string)(unsafe.Pointer(in.Shell))
	out.Passwd = (*string)(unsafe.Pointer(in.Passwd))
	out.PrimaryGroup = (*string)(unsafe.Pointer(in.PrimaryGroup))
	out.LockPassword = (*bool)(unsafe.Pointer(in.LockPassword))
	out.Sudo = (*string)(unsafe.Pointer(in.Sudo))
	out.SSHAuthorizedKeys = *(*[]string)(unsafe.Pointer(&in.SSHAuthorizedKeys))
	return nil
}

// Convert_v1alpha4_User_To_v1alpha3_User is an autogenerated conversion function.
func Convert_v1alpha4_User_To_v1alpha3_User(in *v1alpha4.User, out *User, s conversion.Scope) error {
	return autoConvert_v1alpha4_User_To_v1alpha3_User(in, out, s)
}
