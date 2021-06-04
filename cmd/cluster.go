package cmd

import (
	"github.com/spf13/cobra"

	"github.com/michaelhenkel/gokvm/cluster"
	"github.com/michaelhenkel/gokvm/image"
	"github.com/michaelhenkel/gokvm/network"

	log "github.com/sirupsen/logrus"
)

var (
	img        string
	nw         string
	suffix     string
	worker     int
	controller int
	pubKey     string
)

func init() {
	cobra.OnInitialize(initImageConfig)
	createClusterCmd.PersistentFlags().StringVarP(&img, "image", "i", "default", "")
	createClusterCmd.PersistentFlags().StringVarP(&nw, "network", "l", "gokvm", "")
	createClusterCmd.PersistentFlags().StringVarP(&suffix, "suffix", "s", "local", "")
	createClusterCmd.PersistentFlags().IntVarP(&worker, "worker", "w", 0, "")
	createClusterCmd.PersistentFlags().IntVarP(&controller, "controller", "c", 1, "")
	createClusterCmd.PersistentFlags().StringVarP(&pubKey, "publickey", "k", "~/.ssh/id_rsa.pub", "")

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
	cl := cluster.Cluster{
		Name: name,
		Network: network.Network{
			Name: nw,
		},
		Image: image.Image{
			Name: img,
		},
		Suffix:     suffix,
		Worker:     worker,
		Controller: controller,
		PublicKey:  pubKey,
	}
	return cl.Create()
}

func listCluster() error {
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
