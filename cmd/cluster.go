package cmd

import (
	"fmt"
	"os"

	"code.cloudfoundry.org/bytefmt"
	"github.com/michaelhenkel/gokvm/ansible"
	"github.com/michaelhenkel/gokvm/cluster"
	"github.com/michaelhenkel/gokvm/git"
	"github.com/michaelhenkel/gokvm/image"
	"github.com/michaelhenkel/gokvm/instance"
	"github.com/michaelhenkel/gokvm/ks"
	"github.com/michaelhenkel/gokvm/network"
	"github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"k8s.io/client-go/tools/clientcmd"

	log "github.com/sirupsen/logrus"
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
)

func init() {
	cobra.OnInitialize(initImageConfig)
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
}

func initClusterConfig() {

}

var createClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "creates a cluster",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := createCluster(); err != nil {
			panic(err)
		}
	},
}

var deleteClusterCmd = &cobra.Command{
	Use:   "cluster",
	Short: "deletes a cluster",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := deleteCluster(); err != nil {
			panic(err)
		}
	},
}

var listClusterCmd = &cobra.Command{
	Use:   "cluster",
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
	}

	if err := mergeKubeconfig(cl.Name); err != nil {
		return err
	}

	return nil
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
	if name == "" {
		log.Fatal("Name is required")
	}
	cl := cluster.Cluster{
		Name: name,
	}
	return cl.Delete()
}

func mergeKubeconfig(clusterName string) error {
	kubeConfigPath, err := findKubeConfig()
	if err != nil {
		return err
	}
	loadingRules := clientcmd.ClientConfigLoadingRules{
		Precedence: []string{"/tmp/" + clusterName + "/admin.conf", kubeConfigPath},
	}

	fmt.Println(kubeConfigPath)

	newKubeConfig, err := clientcmd.LoadFromFile("/tmp/" + clusterName + "/admin.conf")
	if err != nil {
		return err
	}
	mergedConfig, err := loadingRules.Load()
	if err != nil {
		return err
	}
	mergedConfig.CurrentContext = newKubeConfig.CurrentContext
	if err := clientcmd.WriteToFile(*mergedConfig, kubeConfigPath); err != nil {
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
