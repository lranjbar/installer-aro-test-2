package agentconfig

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/installer/pkg/asset"
	"github.com/openshift/installer/pkg/types/agent"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"
)

var (
	agentConfigFilename = "agent-config.yaml"
)

// AgentConfig reads the agent-config.yaml file.
type AgentConfig struct {
	File   *asset.File
	Config *agent.Config
}

// Name returns a human friendly name for the asset.
func (*AgentConfig) Name() string {
	return "Agent Config"
}

// Dependencies returns all of the dependencies directly needed to generate
// the asset.
func (*AgentConfig) Dependencies() []asset.Asset {
	return []asset.Asset{}
}

// Generate generates the Agent manifest.
func (a *AgentConfig) Generate(dependencies asset.Parents) error {
	return nil
}

// Files returns the files generated by the asset.
func (a *AgentConfig) Files() []*asset.File {
	if a.File != nil {
		return []*asset.File{a.File}
	}
	return []*asset.File{}
}

// Load returns agent config asset from the disk.
func (a *AgentConfig) Load(f asset.FileFetcher) (bool, error) {

	file, err := f.FetchByName(agentConfigFilename)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, errors.Wrap(err, fmt.Sprintf("failed to load %s file", agentConfigFilename))
	}

	config := &agent.Config{}
	if err := yaml.UnmarshalStrict(file.Data, config); err != nil {
		return false, errors.Wrapf(err, "failed to unmarshal %s", agentConfigFilename)
	}

	a.File, a.Config = file, config
	if err = a.finish(); err != nil {
		return false, err
	}

	return true, nil
}

func (a *AgentConfig) finish() error {
	if err := a.validateAgent().ToAggregate(); err != nil {
		return errors.Wrapf(err, "invalid Agent Config configuration")
	}

	return nil
}

func (a *AgentConfig) validateAgent() field.ErrorList {
	allErrs := field.ErrorList{}

	if err := a.validateNodesHaveAtLeastOneMacAddressDefined(); err != nil {
		allErrs = append(allErrs, err...)
	}

	if err := a.validateRootDeviceHints(); err != nil {
		allErrs = append(allErrs, err...)
	}

	if err := a.validateRoles(); err != nil {
		allErrs = append(allErrs, err...)
	}

	return allErrs
}

func (a *AgentConfig) validateNodesHaveAtLeastOneMacAddressDefined() field.ErrorList {
	var allErrs field.ErrorList

	if len(a.Config.Spec.Hosts) == 0 {
		return allErrs
	}

	rootPath := field.NewPath("Spec", "Hosts")

	for i := range a.Config.Spec.Hosts {
		node := a.Config.Spec.Hosts[i]
		interfacePath := rootPath.Index(i).Child("Interfaces")
		if len(node.Interfaces) == 0 {
			allErrs = append(allErrs, field.Required(interfacePath, "at least one interface must be defined for each node"))
		}

		for j := range node.Interfaces {
			if node.Interfaces[j].MacAddress == "" {
				macAddressPath := interfacePath.Index(j).Child("macAddress")
				allErrs = append(allErrs, field.Required(macAddressPath, "each interface must have a MAC address defined"))
			}
		}
	}
	return allErrs
}

func (a *AgentConfig) validateRootDeviceHints() field.ErrorList {
	var allErrs field.ErrorList
	rootPath := field.NewPath("Spec", "Hosts")

	for i, host := range a.Config.Spec.Hosts {
		hostPath := rootPath.Index(i)
		if host.RootDeviceHints.WWNWithExtension != "" {
			allErrs = append(allErrs, field.Forbidden(
				hostPath.Child("RootDeviceHints", "WWNWithExtension"),
				"WWN extensions are not supported in root device hints"))
		}
		if host.RootDeviceHints.WWNVendorExtension != "" {
			allErrs = append(allErrs, field.Forbidden(
				hostPath.Child("RootDeviceHints", "WWNVendorExtension"),
				"WWN vendor extensions are not supported in root device hints"))
		}
	}

	return allErrs
}

func (a *AgentConfig) validateRoles() field.ErrorList {
	var allErrs field.ErrorList
	rootPath := field.NewPath("Spec", "Hosts")

	for i, host := range a.Config.Spec.Hosts {
		hostPath := rootPath.Index(i)
		if len(host.Role) > 0 && host.Role != "master" && host.Role != "worker" {
			allErrs = append(allErrs, field.Forbidden(
				hostPath.Child("Host"),
				"host role has incorrect value. Role must either be 'master' or 'worker'"))
		}
	}

	return allErrs
}

// HostConfigFileMap is a map from a filepath ("<host>/<file>") to file content
// for hostconfig files.
type HostConfigFileMap map[string][]byte

// HostConfigFiles returns a map from filename to contents of the files used for
// host-specific configuration by the agent installer client
func (a *AgentConfig) HostConfigFiles() (HostConfigFileMap, error) {
	if a == nil || a.Config == nil {
		return nil, nil
	}

	files := HostConfigFileMap{}
	for i, host := range a.Config.Spec.Hosts {
		name := fmt.Sprintf("host-%d", i)
		if host.Hostname != "" {
			name = host.Hostname
		}

		macs := []string{}
		for _, iface := range host.Interfaces {
			macs = append(macs, strings.ToLower(iface.MacAddress)+"\n")
		}

		if len(macs) > 0 {
			files[filepath.Join(name, "mac_addresses")] = []byte(strings.Join(macs, ""))
		}

		rdh, err := yaml.Marshal(host.RootDeviceHints)
		if err != nil {
			return nil, err
		}
		if len(rdh) > 0 && string(rdh) != "{}\n" {
			files[filepath.Join(name, "root-device-hints.yaml")] = rdh
		}

		if len(host.Role) > 0 {
			files[filepath.Join(name, "role")] = []byte(host.Role)
		}
	}
	return files, nil
}
