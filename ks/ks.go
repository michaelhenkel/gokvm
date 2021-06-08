package ks

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/michaelhenkel/gokvm/cluster"
	"github.com/michaelhenkel/gokvm/instance"
)

type Host struct {
	AnsibleHost string `yaml:"ansible_host"`
}

type All struct {
	Hosts map[string]Host   `yaml:"hosts"`
	Vars  map[string]string `yaml:"vars"`
}

type KubeMaster struct {
	Hosts map[string]struct{}
}

type KubeNode struct {
	Hosts map[string]struct{}
}

type Etcd struct {
	Hosts map[string]struct{}
}

type K8SCluster struct {
	Children map[string]struct{} `yaml:"children"`
}

type Inventory struct {
	All        All        `yaml:"all"`
	KubeMaster KubeMaster `yaml:"kube-master"`
	KubeNode   KubeNode   `yaml:"kube-node"`
	Etcd       Etcd       `yaml:"etcd"`
	K8SCluster K8SCluster `yaml:"k8s-cluster"`
}

func Build(cluster *cluster.Cluster, inventoryLocation string) error {
	var allHosts = make(map[string]Host)
	var kubeMasterHosts = make(map[string]struct{})
	var kubeNodeHosts = make(map[string]struct{})
	var etcdHosts = make(map[string]struct{})

	for _, inst := range cluster.Instances {
		allHosts[inst.Name] = Host{
			AnsibleHost: inst.IPAddresses[0],
		}
		switch inst.Role {
		case instance.Controller:
			kubeMasterHosts[inst.Name] = struct{}{}
			etcdHosts[inst.Name] = struct{}{}

		case instance.Worker:
			kubeNodeHosts[inst.Name] = struct{}{}
		}

	}
	i := Inventory{
		All: All{
			Hosts: allHosts,
			Vars: map[string]string{
				"docker_image_repo":          "svl-artifactory.juniper.net/atom-docker-remote",
				"cluster_name":               fmt.Sprintf("%s.%s", cluster.Name, cluster.Suffix),
				"artifacts_dir":              "/tmp/cluster1",
				"kube_network_plugin":        "cni",
				"kube_network_plugin_multus": "false",
				"kubectl_localhost":          "true",
				"kubeconfig_localhost":       "true",
				"override_system_hostname":   "true",
				"container_manager":          "crio",
				"kubelet_deployment_type":    "host",
				"download_container":         "false",
				"etcd_deployment_type":       "host",
				"host_key_checking":          "false",
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
			Children: map[string]struct{}{
				"kube-master": struct{}{},
				"kube-node":   struct{}{},
			},
		},
	}
	inventoryByte, err := yaml.Marshal(&i)
	if err != nil {
		return err
	}
	inventoryString := strings.Replace(string(inventoryByte), "{}", "", -1)
	//fmt.Print(string(inventoryString))
	if err := os.WriteFile(inventoryLocation, []byte(inventoryString), 0600); err != nil {
		return err
	}
	return nil

}
