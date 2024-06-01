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
	"fmt"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	monkalev1alpha1 "github.com/monkale.io/coredns-manager-operator/api/v1alpha1"
)

// generateCorefile is used to generate Corefile based on originalCorefile(string) and DNSZone's zonefile configMaps.
// receives original corednsConfCM and zoneConfigMaps as args.
func generateCorefileCM(dnsConnector *monkalev1alpha1.DNSConnector, corednsConfCM *corev1.ConfigMap, zoneConfigMaps *corev1.ConfigMapList) (corev1.ConfigMap, error) {
	corefileConfigBlockStartPrefix := "# COREDNS CONTROLLER MANAGED BLOCK BEGINNING -- "
	corefileConfigBlockEndPrefix := "# COREDNS CONTROLLER MANAGED BLOCK END -- "
	corefileBlocks := make(map[string]string)
	configMapDomains := make(map[string]bool)
	var newCorefileBuilder strings.Builder

	cmDataKey := dnsConnector.Spec.CorednsCM.CorefileKey
	newCorednsConfCM := corednsConfCM.DeepCopy()

	corednsCorefileContent, ok := corednsConfCM.Data[cmDataKey]
	if !ok {
		return corev1.ConfigMap{}, fmt.Errorf("key %s not found in CoreDNS ConfigMap", cmDataKey)
	}

	for _, configMap := range zoneConfigMaps.Items {
		// extract domain
		domainName, ok := configMap.Annotations["DomainName"]
		if !ok {
			return corev1.ConfigMap{}, fmt.Errorf("configMap %s does not have a domain annotation", configMap.Name)
		}

		// get enabled plugins
		pluginString := ""
		for _, plugin := range dnsConnector.Spec.CorednsZoneEnaledPlugins {
			pluginString += fmt.Sprintf("\n\t%s", plugin)
		}

		// Ensure that zonefile contains zonefile in the cm.data
		zonefileName := domainName + ".zone"
		if _, ok := configMap.Data[zonefileName]; !ok {
			return corev1.ConfigMap{}, fmt.Errorf("configMap %s does not contain zonefile data", configMap.Name)
		}

		// generate zone config block
		configBlock := fmt.Sprintf(`
%s %s
%s:53 {
	file %s/%s%s
}
%s %s`, corefileConfigBlockStartPrefix, domainName, domainName, dnsConnector.Spec.CorednsDeployment.ZoneFileMountDir, zonefileName, pluginString, corefileConfigBlockEndPrefix, domainName)

		corefileBlocks[domainName] = configBlock
		configMapDomains[domainName] = true
	}

	// iterate over corefile
	lines := strings.Split(corednsCorefileContent, "\n")
	inBlock := false

	for _, line := range lines {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, corefileConfigBlockStartPrefix) {
			inBlock = true
			currentDomain := strings.TrimPrefix(trimmedLine, corefileConfigBlockStartPrefix)
			if _, exists := corefileBlocks[currentDomain]; exists {
				newCorefileBuilder.WriteString(corefileBlocks[currentDomain])
				delete(corefileBlocks, currentDomain)
			}
		} else if strings.HasPrefix(trimmedLine, corefileConfigBlockEndPrefix) {
			inBlock = false
		} else if !inBlock {
			newCorefileBuilder.WriteString(line + "\n")
		}
	}

	for _, block := range corefileBlocks {
		newCorefileBuilder.WriteString(block)
	}

	corefileContent := newCorefileBuilder.String()
	// modify configmap
	newCorednsConfCM.Data[cmDataKey] = corefileContent

	return *newCorednsConfCM, nil
}

// getDesiredVolumes iterates over zone configmaps list and returns a map where the keys are volume names volume names based on the
// domain name annotation, and values are names of the zone Config Maps
func getDesiredVolumes(configMaps *corev1.ConfigMapList) (map[string]string, error) {
	desiredVolumes := make(map[string]string)
	for _, configMap := range configMaps.Items {
		domainName, ok := configMap.Annotations["DomainName"]
		if !ok {
			return nil, fmt.Errorf("configMap %s does not have a domain annotation", configMap.Name)
		}
		volumeName := fmt.Sprintf("dnszone-%s", strings.ReplaceAll(domainName, ".", "-"))
		desiredVolumes[volumeName] = configMap.Name
	}
	return desiredVolumes, nil
}

// setZoneFileConifgMaps attaches ConfigMaps to a CoreDNS deployment by adding them as volumes
// and volume mounts to the PodTemplateSpec of the provided StatefulSet, Deployment, or DaemonSet.
// It returns the modified deployment object.
func setZoneFileConifgMaps(dnsConnector monkalev1alpha1.DNSConnector, corednsDeployment client.Object, configMaps *corev1.ConfigMapList) (client.Object, error) {
	var podTemplateSpec *corev1.PodTemplateSpec
	switch res := corednsDeployment.(type) {
	case *appsv1.StatefulSet:
		podTemplateSpec = &res.Spec.Template
	case *appsv1.Deployment:
		podTemplateSpec = &res.Spec.Template
	case *appsv1.DaemonSet:
		podTemplateSpec = &res.Spec.Template
	default:
		return nil, fmt.Errorf("unsupported resource type: %T", res)
	}

	// trigger coredns deployment reconciliation
	if podTemplateSpec.Annotations == nil {
		podTemplateSpec.Annotations = make(map[string]string)
	}
	podTemplateSpec.Annotations["reconcilation-request"] = fmt.Sprintf("%d", metav1.Now().Unix())

	// get desired volumes from the provided configmaps
	desiredVolumes, err := getDesiredVolumes(configMaps)
	if err != nil {
		return nil, err
	}

	// filter exisitng volumes and volume mounts to remove old DNS zone volume
	newVolumes := make([]corev1.Volume, 0)
	newVolumeMounts := make([]corev1.VolumeMount, 0)
	for _, volume := range podTemplateSpec.Spec.Volumes {
		if strings.HasPrefix(volume.Name, "dnszone-") {
			if _, exists := desiredVolumes[volume.Name]; exists {
				newVolumes = append(newVolumes, volume) // keep desired volume
			}
		} else {
			newVolumes = append(newVolumes, volume) // keep other volume
		}
	}

	for _, volumeMount := range podTemplateSpec.Spec.Containers[0].VolumeMounts {
		if strings.HasPrefix(volumeMount.Name, "dnszone-") {
			if _, exists := desiredVolumes[volumeMount.Name]; exists {
				newVolumeMounts = append(newVolumeMounts, volumeMount) // keep desired volumemount
			}
		} else {
			newVolumeMounts = append(newVolumeMounts, volumeMount) // keep other volume mounts
		}
	}

	// init volume and volumemounts
	for volumeName, configMapName := range desiredVolumes {
		// build volume name
		domainName := strings.TrimPrefix(volumeName, "dnszone-")
		domainName = strings.ReplaceAll(domainName, "-", ".")

		volume := corev1.Volume{
			Name: volumeName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  fmt.Sprintf("%s.zone", domainName),
							Path: fmt.Sprintf("%s.zone", domainName),
						},
					},
				},
			},
		}
		volumeMount := corev1.VolumeMount{
			Name:      volumeName,
			MountPath: fmt.Sprintf("%s/%s.zone", dnsConnector.Spec.CorednsDeployment.ZoneFileMountDir, domainName),
			SubPath:   fmt.Sprintf("%s.zone", domainName),
			ReadOnly:  true,
		}
		// Check if the volume already exists
		volumeExists := false
		for _, v := range newVolumes {
			if v.Name == volumeName {
				volumeExists = true
				break
			}
		}

		// Check if the volume mount already exists
		volumeMountExists := false
		for _, vm := range newVolumeMounts {
			if vm.MountPath == volumeMount.MountPath {
				volumeMountExists = true
				break
			}
		}
		// Add the volume and volume mount if they don't already exist
		if !volumeExists {
			newVolumes = append(newVolumes, volume)
		}
		if !volumeMountExists {
			newVolumeMounts = append(newVolumeMounts, volumeMount)
		}
	}

	// update podtemplate with new volumes and volumemount
	podTemplateSpec.Spec.Volumes = newVolumes
	for i := range podTemplateSpec.Spec.Containers {
		podTemplateSpec.Spec.Containers[i].VolumeMounts = newVolumeMounts
	}

	return corednsDeployment, nil
}
