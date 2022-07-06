package terraform

import (
	"context"
	"errors"
	"strings"

	"github.com/oam-dev/terraform-controller/controllers/client"
	"github.com/ttsubo2000/esi-terraform-worker/types"
	"k8s.io/klog/v2"
)

// GetTerraformStatus will get Terraform execution status
func GetTerraformStatus(ctx context.Context, namespace, jobName, containerName, initContainerName string) (types.ConfigurationState, error) {
	klog.InfoS("checking Terraform init and execution status", "Namespace", namespace, "Job", jobName)
	clientSet, err := client.Init()
	if err != nil {
		klog.ErrorS(err, "failed to init clientSet")
		return types.ConfigurationProvisioningAndChecking, err
	}

	// check the stage of the pod

	stage, logs, err := getPodLog(ctx, clientSet, namespace, jobName, containerName, initContainerName)
	if err != nil {
		klog.ErrorS(err, "failed to get pod logs")
		return types.ConfigurationProvisioningAndChecking, err
	}

	success, state, errMsg := analyzeTerraformLog(logs, stage)
	if success {
		return state, nil
	}

	return state, errors.New(errMsg)
}

func analyzeTerraformLog(logs string, stage types.Stage) (bool, types.ConfigurationState, string) {
	lines := strings.Split(logs, "\n")
	for i, line := range lines {
		if strings.Contains(line, "31mError:") {
			errMsg := strings.Join(lines[i:], "\n")
			if strings.Contains(errMsg, "Invalid Alibaba Cloud region") {
				return false, types.InvalidRegion, errMsg
			}
			switch stage {
			case types.TerraformInit:
				return false, types.TerraformInitError, errMsg
			case types.TerraformApply:
				return false, types.ConfigurationApplyFailed, errMsg
			}
		}
	}
	return true, types.ConfigurationProvisioningAndChecking, ""
}
