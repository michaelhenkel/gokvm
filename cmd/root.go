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
	rootCmd.PersistentFlags().StringVarP(&name, "name", "n", "", "")

	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(listCmd)
}

func initConfig() {

}
