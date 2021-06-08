package ks

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/michaelhenkel/gokvm/cluster"
	"github.com/michaelhenkel/gokvm/instance"
)

type Host struct {
	AnsibleHost   string `yaml:"ansible_host"`
	AnsibleBecome bool   `yaml:"ansible_become"`
}

type All struct {
	Hosts map[string]Host   `yaml:"hosts"`
	Vars  map[string]string `yaml:"vars"`
}

type KubeMaster struct {
	Hosts map[string]interface{}
}

type KubeNode struct {
	Hosts map[string]interface{}
}

type Etcd struct {
	Hosts map[string]interface{}
}

type K8SCluster struct {
	Children map[string]string `yaml:"children"`
}

type Inventory struct {
	All        All        `yaml:"all"`
	KubeMaster KubeMaster `yaml:"kube-master"`
	KubeNode   KubeNode   `yaml:"kube-node"`
	Etcd       Etcd       `yaml:"etcd"`
	K8SCluster K8SCluster `yaml:"k8s-cluster"`
}

func Build(cluster *cluster.Cluster) error {
	var allHosts = make(map[string]Host)
	var kubeMasterHosts = make(map[string]interface{})
	var kubeNodeHosts = make(map[string]interface{})
	var etcdHosts = make(map[string]interface{})

	for _, inst := range cluster.Instances {
		allHosts[inst.Name] = Host{
			AnsibleHost:   inst.IPAddresses[0],
			AnsibleBecome: true,
		}
		switch inst.Role {
		case instance.Controller:
			kubeMasterHosts[inst.Name] = ""
			etcdHosts[inst.Name] = ""

		case instance.Worker:
			kubeNodeHosts[inst.Name] = ""
		}

	}
	i := Inventory{
		All: All{
			Hosts: allHosts,
			Vars: map[string]string{
				"docker_image_repo":            "svl-artifactory.juniper.net/atom-docker-remote",
				"cluster_name":                 fmt.Sprintf("%s.%s", cluster.Name, cluster.Suffix),
				"artifacts_dir":                "/tmp/cluster1",
				"kubectl.cluster.localhost":    "true",
				"kubeconfig.cluster.localhost": "true",
			},
		},
		KubeMaster: KubeMaster{
			Hosts: kubeMasterHosts,
		},
		KubeNode: KubeNode{
			Hosts: kubeNodeHosts,
		},
		Etcd: Etcd{
			Hosts: etcdHosts,
		},
		K8SCluster: K8SCluster{
			Children: map[string]string{
				"kube-master": "",
				"kube-node":   "",
			},
		},
	}
	inventoryByte, err := yaml.Marshal(&i)
	if err != nil {
		return err
	}
	fmt.Print(string(inventoryByte))
	return nil

}
