package cmd

import (
	"github.com/michaelhenkel/gokvm/cluster"
	"github.com/spf13/cobra"

	log "github.com/sirupsen/logrus"
)

func init() {
	cobra.OnInitialize(initSnapshotConfig)
	snapshotCmd.AddCommand(createSnapshotCmd)
	snapshotCmd.AddCommand(listSnapshotCmd)
	snapshotCmd.AddCommand(revertSnapshotCmd)
}

func initSnapshotConfig() {

}

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "manages snapshots",
	Long:  `All software has versions. This is Hugo's`,
}

var createSnapshotCmd = &cobra.Command{
	Use:   "create",
	Short: "creates an snapshot",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Error("Name is needed")
		} else {
			name = args[0]
		}
		clusterList, err := cluster.List()
		if err != nil {
			panic(err)
		}
		for _, cl := range clusterList {
			if cl.Name == name {
				if err := cl.CreateSnapshot(); err != nil {
					panic(err)
				}
			}
		}
	},
}

var listSnapshotCmd = &cobra.Command{
	Use:   "list",
	Short: "list a snapshot",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Error("Name is needed")
		} else {
			name = args[0]
		}
		clusterList, err := cluster.List()
		if err != nil {
			panic(err)
		}
		for _, cl := range clusterList {
			if cl.Name == name {
				if err := cl.ListSnapshot(); err != nil {
					panic(err)
				}
			}
		}
	},
}

var revertSnapshotCmd = &cobra.Command{
	Use:   "revert",
	Short: "reverts an snapshot",
	Long:  `All software has versions. This is Hugo's`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			log.Error("Name is needed")
		} else {
			name = args[0]
		}
		clusterList, err := cluster.List()
		if err != nil {
			panic(err)
		}
		for _, cl := range clusterList {
			if cl.Name == name {
				if err := cl.RevertSnapshot(); err != nil {
					panic(err)
				}
			}
		}
	},
}
