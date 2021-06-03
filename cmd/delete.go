package cmd

import (
	"errors"

	"github.com/spf13/cobra"
)

func init() {
	deleteCmd.AddCommand(deleteNetworkCmd)
	deleteCmd.AddCommand(deleteImageCmd)
	deleteCmd.AddCommand(deleteClusterCmd)
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "deletes network/cluster/image",
	Long:  `All software has versions. This is Hugo's`,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			return errors.New("requires a color argument")
		}
		if args[0] != string(NETWORK) && args[0] != string(IMAGE) && args[0] != string(CLUSTER) {
			return errors.New("wrong command")
		}

		return nil
	},
}
