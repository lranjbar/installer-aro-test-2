package dnsmasq

import (
	"bytes"
	"encoding/json"
	"fmt"
	"text/template"

	ign2types "github.com/coreos/ignition/config/v2_2/types"
	ignutil "github.com/coreos/ignition/v2/config/util"
	ign3types "github.com/coreos/ignition/v2/config/v3_1/types"
	mcfgv1 "github.com/openshift/machine-config-operator/pkg/apis/machineconfiguration.openshift.io/v1"
	"github.com/vincent-petithory/dataurl"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	configTemplate = template.Must(template.New("").Parse(`resolv-file=/etc/resolv.conf.dnsmasq
strict-order
address=/api.{{ .ClusterDomain }}/{{ .APIIntIP }}
address=/api-int.{{ .ClusterDomain }}/{{ .APIIntIP }}
address=/.apps.{{ .ClusterDomain }}/{{ .IngressIP }}
{{- range $GatewayDomain := .GatewayDomains }}
address=/{{ $GatewayDomain }}/{{ $.GatewayPrivateEndpointIP }}
{{- end }}
user=dnsmasq
group=dnsmasq
no-hosts
cache-size=0
`))

	service = `[Unit]
Description=DNS caching server.
After=network-online.target
Before=bootkube.service

[Service]
ExecStartPre=/bin/bash -c '/bin/cp /etc/resolv.conf /etc/resolv.conf.dnsmasq; /bin/sed -ni -e "/^nameserver /!p; \\$$a nameserver $$(hostname -I)" /etc/resolv.conf; /usr/sbin/restorecon /etc/resolv.conf'
ExecStart=/usr/sbin/dnsmasq -k
ExecStop=/bin/bash -c '/bin/mv /etc/resolv.conf.dnsmasq /etc/resolv.conf; /usr/sbin/restorecon /etc/resolv.conf'
Restart=always

[Install]
WantedBy=multi-user.target
`
)

func config(clusterDomain, apiIntIP, ingressIP string, gatewayDomains []string, gatewayPrivateEndpointIP string) ([]byte, error) {
	buf := &bytes.Buffer{}

	err := configTemplate.ExecuteTemplate(buf, "", &struct {
		ClusterDomain            string
		APIIntIP                 string
		IngressIP                string
		GatewayDomains           []string
		GatewayPrivateEndpointIP string
	}{
		ClusterDomain:            clusterDomain,
		APIIntIP:                 apiIntIP,
		IngressIP:                ingressIP,
		GatewayDomains:           gatewayDomains,
		GatewayPrivateEndpointIP: gatewayPrivateEndpointIP,
	})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func Ignition2Config(clusterDomain, apiIntIP, ingressIP string, gatewayDomains []string, gatewayPrivateEndpointIPteEndpointIP string) (*ign2types.Config, error) {
	config, err := config(clusterDomain, apiIntIP, ingressIP, gatewayDomains, gatewayPrivateEndpointIPteEndpointIP)
	if err != nil {
		return nil, err
	}

	return &ign2types.Config{
		Ignition: ign2types.Ignition{
			Version: ign2types.MaxVersion.String(),
		},
		Storage: ign2types.Storage{
			Files: []ign2types.File{
				{
					Node: ign2types.Node{
						Filesystem: "root",
						Overwrite:  ignutil.BoolToPtr(true),
						Path:       "/etc/dnsmasq.conf",
						User: &ign2types.NodeUser{
							Name: "root",
						},
					},
					FileEmbedded1: ign2types.FileEmbedded1{
						Contents: ign2types.FileContents{
							Source: dataurl.EncodeBytes(config),
						},
						Mode: ignutil.IntToPtr(0644),
					},
				},
			},
		},
		Systemd: ign2types.Systemd{
			Units: []ign2types.Unit{
				{
					Contents: service,
					Enabled:  ignutil.BoolToPtr(true),
					Name:     "dnsmasq.service",
				},
			},
		},
	}, nil
}

func Ignition3Config(clusterDomain, apiIntIP, ingressIP string, gatewayDomains []string, gatewayPrivateEndpointIP string) (*ign3types.Config, error) {
	config, err := config(clusterDomain, apiIntIP, ingressIP, gatewayDomains, gatewayPrivateEndpointIP)
	if err != nil {
		return nil, err
	}

	return &ign3types.Config{
		Ignition: ign3types.Ignition{
			Version: ign3types.MaxVersion.String(),
		},
		Storage: ign3types.Storage{
			Files: []ign3types.File{
				{
					Node: ign3types.Node{
						Overwrite: ignutil.BoolToPtr(true),
						Path:      "/etc/dnsmasq.conf",
						User: ign3types.NodeUser{
							Name: ignutil.StrToPtr("root"),
						},
					},
					FileEmbedded1: ign3types.FileEmbedded1{
						Contents: ign3types.Resource{
							Source: ignutil.StrToPtr(dataurl.EncodeBytes(config)),
						},
						Mode: ignutil.IntToPtr(0644),
					},
				},
			},
		},
		Systemd: ign3types.Systemd{
			Units: []ign3types.Unit{
				{
					Contents: &service,
					Enabled:  ignutil.BoolToPtr(true),
					Name:     "dnsmasq.service",
				},
			},
		},
	}, nil
}

func MachineConfig(clusterDomain, apiIntIP, ingressIP, role string, gatewayDomains []string, gatewayPrivateEndpointIP string) (*mcfgv1.MachineConfig, error) {
	ignConfig, err := Ignition2Config(clusterDomain, apiIntIP, ingressIP, gatewayDomains, gatewayPrivateEndpointIP)
	if err != nil {
		return nil, err
	}

	b, err := json.Marshal(ignConfig)
	if err != nil {
		return nil, err
	}

	// canonicalise the machineconfig payload the same way as MCO
	var i interface{}
	err = json.Unmarshal(b, &i)
	if err != nil {
		return nil, err
	}

	rawExt := runtime.RawExtension{}
	rawExt.Raw, err = json.Marshal(i)
	if err != nil {
		return nil, err
	}

	return &mcfgv1.MachineConfig{
		TypeMeta: metav1.TypeMeta{
			APIVersion: mcfgv1.SchemeGroupVersion.String(),
			Kind:       "MachineConfig",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("99-%s-aro-dns", role),
			Labels: map[string]string{
				"machineconfiguration.openshift.io/role": role,
			},
		},
		Spec: mcfgv1.MachineConfigSpec{
			Config: rawExt,
		},
	}, nil
}
