/*
Copyright 2021 The Kubernetes Authors.

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

package v1beta1

import (
	"fmt"

	apiconversion "k8s.io/apimachinery/pkg/conversion"
	"sigs.k8s.io/controller-runtime/pkg/conversion"

	utilconversion "sigs.k8s.io/cluster-api/util/conversion"

	infrav1 "sigs.k8s.io/cluster-api/test/infrastructure/docker/api/v1beta2"
)

func (src *DockerCluster) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.DockerCluster)
	if err := Convert_v1beta1_DockerCluster_To_v1beta2_DockerCluster(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &infrav1.DockerCluster{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	dst.Spec.LoadBalancer.ImageMeta.NewField = restored.Spec.LoadBalancer.ImageMeta.NewField

	return nil
}

func (dst *DockerCluster) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.DockerCluster)
	if err := Convert_v1beta2_DockerCluster_To_v1beta1_DockerCluster(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion except for metadata
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

func (src *DockerClusterList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.DockerClusterList)

	return Convert_v1beta1_DockerClusterList_To_v1beta2_DockerClusterList(src, dst, nil)
}

func (dst *DockerClusterList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.DockerClusterList)

	return Convert_v1beta2_DockerClusterList_To_v1beta1_DockerClusterList(src, dst, nil)
}

func (src *DockerClusterTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.DockerClusterTemplate)

	return Convert_v1beta1_DockerClusterTemplate_To_v1beta2_DockerClusterTemplate(src, dst, nil)
}

func (dst *DockerClusterTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.DockerClusterTemplate)

	return Convert_v1beta2_DockerClusterTemplate_To_v1beta1_DockerClusterTemplate(src, dst, nil)
}

func (src *DockerClusterTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.DockerClusterTemplateList)

	return Convert_v1beta1_DockerClusterTemplateList_To_v1beta2_DockerClusterTemplateList(src, dst, nil)
}

func (dst *DockerClusterTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.DockerClusterTemplateList)

	return Convert_v1beta2_DockerClusterTemplateList_To_v1beta1_DockerClusterTemplateList(src, dst, nil)
}

func (src *DockerMachine) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.DockerMachine)

	return Convert_v1beta1_DockerMachine_To_v1beta2_DockerMachine(src, dst, nil)
}

func (dst *DockerMachine) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.DockerMachine)

	return Convert_v1beta2_DockerMachine_To_v1beta1_DockerMachine(src, dst, nil)
}

func (src *DockerMachineList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.DockerMachineList)

	return Convert_v1beta1_DockerMachineList_To_v1beta2_DockerMachineList(src, dst, nil)
}

func (dst *DockerMachineList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.DockerMachineList)

	return Convert_v1beta2_DockerMachineList_To_v1beta1_DockerMachineList(src, dst, nil)
}

func (src *DockerMachineTemplate) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.DockerMachineTemplate)
	fmt.Println("DEBUG: Convert DockerMachineTemplate v1beta1 => v1beta2")
	if err := Convert_v1beta1_DockerMachineTemplate_To_v1beta2_DockerMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Manually restore data.
	restored := &infrav1.DockerMachineTemplate{}
	if ok, err := utilconversion.UnmarshalData(src, restored); err != nil || !ok {
		return err
	}

	// Just for testing, not actually restoring anything

	return nil
}

func (dst *DockerMachineTemplate) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.DockerMachineTemplate)
	fmt.Println("DEBUG: Convert DockerMachineTemplate v1beta2 => v1beta1")
	if err := Convert_v1beta2_DockerMachineTemplate_To_v1beta1_DockerMachineTemplate(src, dst, nil); err != nil {
		return err
	}

	// Preserve Hub data on down-conversion except for metadata
	if err := utilconversion.MarshalData(src, dst); err != nil {
		return err
	}

	return nil
}

func (src *DockerMachineTemplateList) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*infrav1.DockerMachineTemplateList)

	return Convert_v1beta1_DockerMachineTemplateList_To_v1beta2_DockerMachineTemplateList(src, dst, nil)
}

func (dst *DockerMachineTemplateList) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*infrav1.DockerMachineTemplateList)

	return Convert_v1beta2_DockerMachineTemplateList_To_v1beta1_DockerMachineTemplateList(src, dst, nil)
}

// Convert_v1beta1_DockerMachineSpec_To_v1beta2_DockerMachineSpec is an autogenerated conversion function.
func Convert_v1beta1_DockerMachineSpec_To_v1beta2_DockerMachineSpec(in *DockerMachineSpec, out *infrav1.DockerMachineSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_DockerMachineSpec_To_v1beta2_DockerMachineSpec(in, out, s); err != nil {
		return err
	}
	out.NewCustomImage = in.CustomImage
	return nil
}

// Convert_v1beta2_DockerMachineSpec_To_v1beta1_DockerMachineSpec is an autogenerated conversion function.
func Convert_v1beta2_DockerMachineSpec_To_v1beta1_DockerMachineSpec(in *infrav1.DockerMachineSpec, out *DockerMachineSpec, s apiconversion.Scope) error {
	if err := autoConvert_v1beta2_DockerMachineSpec_To_v1beta1_DockerMachineSpec(in, out, s); err != nil {
		return err
	}
	out.CustomImage = in.NewCustomImage
	return nil
}

// Convert_v1beta1_ImageMeta_To_v1beta2_ImageMeta is an autogenerated conversion function.
func Convert_v1beta1_ImageMeta_To_v1beta2_ImageMeta(in *ImageMeta, out *infrav1.ImageMeta, s apiconversion.Scope) error {
	if err := autoConvert_v1beta1_ImageMeta_To_v1beta2_ImageMeta(in, out, s); err != nil {
		return err
	}
	out.NewImageRepository = in.ImageRepository
	return nil
}

// Convert_v1beta2_ImageMeta_To_v1beta1_ImageMeta is an autogenerated conversion function.
func Convert_v1beta2_ImageMeta_To_v1beta1_ImageMeta(in *infrav1.ImageMeta, out *ImageMeta, s apiconversion.Scope) error {
	if err := autoConvert_v1beta2_ImageMeta_To_v1beta1_ImageMeta(in, out, s); err != nil {
		return err
	}
	out.ImageRepository = in.NewImageRepository
	return nil
}
