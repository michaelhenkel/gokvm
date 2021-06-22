package cmd

import (
	"github.com/spf13/cobra"
)

type Commands string

const (
	NETWORK Commands = "network"
	CLUSTER Commands = "cluster"
	IMAGE   Commands = "image"
)

var (
	name    string
	rootCmd = &cobra.Command{
		Use:   "gokvm",
		Short: "",
		Long:  ``,
	}
)

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.AddCommand(clusterCmd)
	rootCmd.AddCommand(imageCmd)
	rootCmd.AddCommand(networkCmd)
	rootCmd.AddCommand(snapshotCmd)
}

func initConfig() {

}
