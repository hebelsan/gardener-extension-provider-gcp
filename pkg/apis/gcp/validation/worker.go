// SPDX-FileCopyrightText: 2024 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package validation

import (
	"fmt"
	"strings"

	"github.com/gardener/gardener/pkg/apis/core"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/gardener/gardener-extension-provider-gcp/pkg/apis/gcp"
)

// VolumeTypeScratch is the gcp SCRATCH volume type
const VolumeTypeScratch = "SCRATCH"

var (
	validVolumeLocalSSDInterfacesTypes = sets.New("NVME", "SCSI")

	providerFldPath = field.NewPath("providerConfig")
	volumeFldPath   = providerFldPath.Child("volume")
)

// ValidateWorkerConfig validates a WorkerConfig object.
func ValidateWorkerConfig(workerConfig *gcp.WorkerConfig, dataVolumes []core.DataVolume) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, dataVolume := range dataVolumes {
		dataVolumeFldPath := field.NewPath("dataVolumes").Index(i)
		allErrs = append(allErrs, validateDataVolume(workerConfig, dataVolume, dataVolumeFldPath)...)
	}

	if workerConfig != nil {
		allErrs = append(allErrs, validateGPU(workerConfig.GPU, providerFldPath.Child("gpu"))...)
		allErrs = append(allErrs, validateServiceAccount(workerConfig.ServiceAccount, providerFldPath.Child("serviceAccount"))...)
		if workerConfig.Volume != nil {
			allErrs = append(allErrs, validateDiskEncryption(workerConfig.Volume.Encryption, volumeFldPath.Child("encryption"))...)
		}
	}

	return allErrs
}

func validateGPU(gpu *gcp.GPU, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if gpu == nil {
		return allErrs
	}

	if gpu.AcceleratorType == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("acceleratorType"), "must be set when providing gpu"))
	}

	if gpu.Count <= 0 {
		allErrs = append(allErrs, field.Forbidden(fldPath.Child("count"), "must be > 0 when providing gpu"))
	}

	return allErrs
}

func validateServiceAccount(sa *gcp.ServiceAccount, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if sa == nil {
		return allErrs
	}

	if sa.Email == "" {
		allErrs = append(allErrs, field.Required(fldPath.Child("email"), "must be set when providing service account"))
	}

	if len(sa.Scopes) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("scopes"), "must have at least one scope"))
	} else {
		existingScopes := sets.NewString()

		for i, scope := range sa.Scopes {
			switch {
			case scope == "":
				allErrs = append(allErrs, field.Required(fldPath.Child("scopes").Index(i), "must not be empty"))
			case existingScopes.Has(scope):
				allErrs = append(allErrs, field.Duplicate(fldPath.Child("scopes").Index(i), scope))
			default:
				existingScopes.Insert(scope)
			}
		}
	}

	return allErrs
}

// validateDiskEncryption validates the provider specific disk encryption configuration for a volume
func validateDiskEncryption(encryption *gcp.DiskEncryption, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if encryption == nil {
		return allErrs
	}

	if encryption.KmsKeyName == nil || strings.TrimSpace(*encryption.KmsKeyName) == "" {
		// Currently DiskEncryption only contains CMEK fields. Hence if not nil, then kmsKeyName is a must
		// Validation logic will need to be modified when CSEK fields are possibly added to gcp.DiskEncryption in the future.
		allErrs = append(allErrs, field.Required(fldPath.Child("kmsKeyName"), "must be specified when configuring disk encryption"))
	}

	return allErrs
}

func validateDataVolume(workerConfig *gcp.WorkerConfig, volume core.DataVolume, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if volume.Type == nil {
		allErrs = append(allErrs, field.Required(fldPath.Child("type"), "must not be empty"))
		return allErrs
	}
	if *volume.Type == VolumeTypeScratch {
		if workerConfig == nil || workerConfig.Volume == nil || workerConfig.Volume.LocalSSDInterface == nil {
			allErrs = append(allErrs, field.Required(volumeFldPath.Child("interface"), fmt.Sprintf("must be set when using %s volumes", VolumeTypeScratch)))
		} else {
			if !validVolumeLocalSSDInterfacesTypes.Has(*workerConfig.Volume.LocalSSDInterface) {
				allErrs = append(allErrs, field.NotSupported(volumeFldPath.Child("interface"), *workerConfig.Volume.LocalSSDInterface, validVolumeLocalSSDInterfacesTypes.UnsortedList()))
			}
		}
		// DiskEncryption not allowed for type SCRATCH
		if workerConfig != nil && workerConfig.Volume != nil && workerConfig.Volume.Encryption != nil {
			allErrs = append(allErrs, field.Invalid(volumeFldPath.Child("encryption"), *workerConfig.Volume.Encryption, fmt.Sprintf("must not be set in combination with %s volumes", VolumeTypeScratch)))
		}
	} else {
		// LocalSSDInterface only allowed for type SCRATCH
		if workerConfig != nil && workerConfig.Volume != nil && workerConfig.Volume.LocalSSDInterface != nil {
			allErrs = append(allErrs, field.Invalid(volumeFldPath.Child("interface"), *workerConfig.Volume.LocalSSDInterface, fmt.Sprintf("is only allowed for type %s", VolumeTypeScratch)))
		}
	}

	return allErrs
}
