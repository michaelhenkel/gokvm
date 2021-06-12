package cmd

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"code.cloudfoundry.org/bytefmt"
	"github.com/michaelhenkel/gokvm/ansible"
	"github.com/michaelhenkel/gokvm/cluster"
	"github.com/michaelhenkel/gokvm/git"
	"github.com/michaelhenkel/gokvm/image"
	"github.com/michaelhenkel/gokvm/instance"
	"github.com/michaelhenkel/gokvm/ks"
	"github.com/michaelhenkel/gokvm/network"
	"github.com/michaelhenkel/gokvm/remote"
	"github.com/mitchellh/go-homedir"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	img          string
	nw           string
	suffix       string
	worker       int
	controller   int
	pubKeyPath   string
	cpu          int
	memory       string
	disk         string
	k8sinventory string
	gitLocation  string
	runAnsible   bool
	runCommand   bool
)

func init() {
	cobra.OnInitialize(initImageConfig)
	clusterCmd.AddCommand(createClusterCmd)
	clusterCmd.AddCommand(deleteClusterCmd)
	clusterCmd.AddCommand(listClusterCmd)
	clusterCmd.AddCommand(snapshotClusterCmd)
	clusterCmd.AddCommand(revertClusterCmd)
	createClusterCmd.PersistentFlags().StringVarP(&img, "image", "i", "default", "")
	createClusterCmd.PersistentFlags().StringVarP(&nw, "network", "l", "gokvm", "")
	createClusterCmd.PersistentFlags().StringVarP(&suffix, "suffix", "s", "local", "")
	createClusterCmd.PersistentFlags().IntVarP(&worker, "worker", "w", 0, "")
	createClusterCmd.PersistentFlags().IntVarP(&controller, "controller", "c", 1, "")
	createClusterCmd.PersistentFlags().StringVarP(&memory, "memory", "m", "12G", "")
	createClusterCmd.PersistentFlags().IntVarP(&cpu, "cpu", "v", 4, "")
	createClusterCmd.PersistentFlags().StringVarP(&disk, "disk", "d", "10G", "")
	createClusterCmd.PersistentFlags().StringVarP(&distribution, "distribution", "p", "ubuntu", "")
	createClusterCmd.PersistentFlags().StringVarP(&pubKeyPath, "publickey", "k", "", "")
	createClusterCmd.PersistentFlags().StringVarP(&k8sinventory, "inventory", "y", "", "")
	createClusterCmd.PersistentFlags().StringVarP(&gitLocation, "gitlocation", "g", "", "")
	createClusterCmd.PersistentFlags().BoolVarP(&runAnsible, "run", "r", false, "")
	createClusterCmd.PersistentFlags().BoolVarP(&runCommand, "exec", "x", false, "")
}

func initClusterConfig() {

}

var clusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "manages a cluster",
	Long:  `All software has versions. This is Hugo's`,
}

var createClusterCmd = &cobra.Command{
	Use:   "create",
	Short: "creates a cluster",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Error("Name is needed")
		} else {
			name = args[0]
		}
		if err := createCluster(); err != nil {
			panic(err)
		}
	},
}

var snapshotClusterCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "snapshots a cluster",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Error("Name is needed")
		} else {
			name = args[0]
		}
		cl := cluster.Cluster{
			Name: name,
		}
		if err := cl.CreateSnapshot(); err != nil {
			panic(err)
		}
	},
}

var revertClusterCmd = &cobra.Command{
	Use:   "revert",
	Short: "reverts a cluster snapshot",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Error("Name is needed")
		} else {
			name = args[0]
		}
		cl := cluster.Cluster{
			Name: name,
		}
		if err := cl.RevertSnapshot(); err != nil {
			panic(err)
		}
	},
}

var deleteClusterCmd = &cobra.Command{
	Use:   "delete",
	Short: "deletes a cluster",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Error("Name is needed")
		} else {
			name = args[0]
		}
		if err := deleteCluster(); err != nil {
			panic(err)
		}
	},
}

var listClusterCmd = &cobra.Command{
	Use:   "list",
	Short: "lists cluster",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := listCluster(); err != nil {
			panic(err)
		}
	},
}

func createCluster() error {
	if name == "" {
		log.Fatal("Name is required")
	}
	if nw == "" {
		networks, err := network.List()
		if err != nil {
			return err
		}
		if len(networks) > 0 {
			nw = networks[0].Name
		}
	}
	if pubKeyPath == "" {
		dirname, err := os.UserHomeDir()
		if err != nil {
			return err
		}
		pubKeyPath = fmt.Sprintf("%s/.ssh/id_rsa.pub", dirname)
	}
	f, err := os.ReadFile(pubKeyPath)
	if err != nil {
		return err
	}

	memBytes, err := bytefmt.ToBytes(memory)
	if err != nil {
		return err
	}

	cl := cluster.Cluster{
		Name: name,
		Network: network.Network{
			Name: nw,
		},
		Image: image.Image{
			Name:             img,
			Distribution:     distribution,
			ImageType:        image.DISTRIBUTION,
			LibvirtImagePath: "/var/lib/libvirt/images",
		},
		Suffix:     suffix,
		Worker:     worker,
		Controller: controller,
		PublicKey:  string(f),
		Resources: instance.Resources{
			Memory: memBytes,
			CPU:    cpu,
			Disk:   disk,
		},
	}

	if err := cl.Create(); err != nil {
		return err
	}

	clusterList, err := cluster.List()
	if err != nil {
		return err
	}
	if k8sinventory != "" {
		for _, newCL := range clusterList {
			if newCL.Name == cl.Name {
				if err := ks.Build(newCL, k8sinventory); err != nil {
					return err
				}
				break

			}
		}
	}

	if gitLocation != "" {
		if err := git.Clone(gitLocation); err != nil {
			return err
		}
	}

	if runAnsible {
		if err := ansible.Run(k8sinventory, gitLocation+"/cluster.yml", cl.Name); err != nil {
			return err
		}
		if err := mergeKubeconfig(cl.Name, cl.Suffix); err != nil {
			return err
		}
	}

	if runCommand {
		for idx, newCL := range clusterList {
			if newCL.Name == cl.Name {
				var wg sync.WaitGroup
				for _, inst := range newCL.Instances {
					wg.Add(1)
					go func(inst *instance.Instance) {
						hosts := buildHosts(newCL.Instances)
						conn, err := remote.Connect(inst.IPAddresses[0]+":22", "root")
						if err != nil {
							log.Info(err)
						}
						cmds := remote.GetUbuntuCMDS()
						cmds = append(cmds, hosts...)
						if inst.Role == instance.Controller {
							cmds = append(cmds, remote.KubeadmImagePull()...)
						}
						_, err = conn.SendCommands(inst.Image.Distribution, cmds, &wg)
						if err != nil {
							log.Info(err)
						}
					}(inst)
				}
				wg.Wait()
				var joinCmd string
				for _, inst := range newCL.Instances {
					if inst.Role == instance.Controller {
						conn, err := remote.Connect(inst.IPAddresses[0]+":22", "root")
						if err != nil {
							log.Info(err)
						}
						podCidr := fmt.Sprintf("10.244.%d.0/24", idx)
						serviceCidr := fmt.Sprintf("10.96.%d.0/24", idx)
						_, err = conn.SendCommands(inst.Image.Distribution, remote.KubeadmInit(podCidr, serviceCidr, inst.IPAddresses[0]), nil)
						if err != nil {
							log.Info(err)
						}
						joinCmdByte, err := conn.SendCommands(inst.Image.Distribution, []string{"kubeadm token create --print-join-command"}, nil)
						if err != nil {
							log.Info(err)
						}
						joinCmd = string(joinCmdByte)
						kubeConfigByte, err := conn.SendCommands(inst.Image.Distribution, []string{"cat /etc/kubernetes/admin.conf"}, nil)
						if err != nil {
							log.Info(err)
						}
						if _, err := os.Stat(fmt.Sprintf("/tmp/%s", inst.ClusterName)); os.IsNotExist(err) {
							if err := os.Mkdir(fmt.Sprintf("/tmp/%s", inst.ClusterName), 0700); err != nil {
								log.Info(err)
							}
						}
						if err := os.WriteFile(fmt.Sprintf("/tmp/%s/admin.conf", inst.ClusterName), kubeConfigByte, 0600); err != nil {
							log.Info(err)
						}
						break
					}
				}
				wg.Wait()
				for _, inst := range newCL.Instances {
					if inst.Role == instance.Worker {
						fmt.Println("adding worker")
						wg.Add(1)
						go func(inst *instance.Instance) {
							conn, err := remote.Connect(inst.IPAddresses[0]+":22", "root")
							if err != nil {
								log.Info(err)
							}
							joinCmd = strings.TrimRight(joinCmd, "\r\n")
							_, err = conn.SendCommands(inst.Image.Distribution, []string{joinCmd}, &wg)
							if err != nil {
								log.Info("ERROR", err)
							}
						}(inst)
					}
				}
				wg.Wait()
			}
		}
		if err := mergeKubeconfig(cl.Name, cl.Suffix); err != nil {
			return err
		}
	}
	return nil
}

func buildHosts(instanceList []*instance.Instance) []string {
	var hostsCmd []string
	for _, inst := range instanceList {
		nameList := strings.Split(inst.Name, ".")
		hostsLine := fmt.Sprintf("echo %s %s %s >> /etc/hosts", inst.IPAddresses[0], inst.Name, nameList[0])
		hostsCmd = append(hostsCmd, hostsLine)
	}
	return hostsCmd
}

func listCluster() error {
	clusters, err := cluster.List()
	if err != nil {
		return err
	}
	cluster.Render(clusters)
	return nil
}

func deleteCluster() error {
	cl := cluster.Cluster{
		Name: name,
	}
	return cl.Delete()
}

func mergeKubeconfig(clusterName string, suffix string) error {
	kubeConfigPath, err := findKubeConfig()
	if err != nil {
		return err
	}

	newKubeConfig, err := clientcmd.LoadFromFile("/tmp/" + clusterName + "/admin.conf")
	if err != nil {
		return err
	}
	existingKubeConfig, err := clientcmd.LoadFromFile(kubeConfigPath)
	if err != nil {
		return err
	}

	for k, v := range newKubeConfig.Clusters {
		existingKubeConfig.Clusters[k] = v
	}

	for k, v := range newKubeConfig.Contexts {
		existingKubeConfig.Contexts[k] = v
	}
	for k, v := range newKubeConfig.AuthInfos {
		existingKubeConfig.AuthInfos[k] = v
	}
	newKubeConfig.CurrentContext = existingKubeConfig.CurrentContext

	if err := clientcmd.WriteToFile(*existingKubeConfig, kubeConfigPath); err != nil {
		return err
	}
	return nil
}

func findKubeConfig() (string, error) {
	path, err := homedir.Expand("~/.kube/config")
	if err != nil {
		return "", err
	}
	return path, nil
}
