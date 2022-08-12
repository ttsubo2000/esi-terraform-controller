package provider

import (
	"github.com/pkg/errors"
	"gopkg.in/yaml.v2"
	"k8s.io/klog/v2"
)

const (
	envHashicupsUser     = "HASHICUPS_USERNAME"
	envHashicupsPassword = "HASHICUPS_PASSWORD"
	envHashicupsHost     = "HASHICUPS_HOST"
)

// VSphereCredentials are credentials for VSphere
type HashicupsCredentials struct {
	HashicupsUser     string `yaml:"HashicupsUser"`
	HashicupsPassword string `yaml:"HashicupsPassword"`
	HashicupsHost     string `yaml:"HashicupsHost"`
}

func getHashicupsCredentials(secretData []byte, name, namespace string) (map[string]string, error) {
	var cred HashicupsCredentials
	if err := yaml.Unmarshal(secretData, &cred); err != nil {
		klog.ErrorS(err, errConvertCredentials, "Name", name, "Namespace", namespace)
		return nil, errors.Wrap(err, errConvertCredentials)
	}
	return map[string]string{
		envHashicupsUser:     cred.HashicupsUser,
		envHashicupsPassword: cred.HashicupsPassword,
		envHashicupsHost:     cred.HashicupsHost,
	}, nil
}
