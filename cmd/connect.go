package cmd

import (
	"os"

	"github.com/JPM1118/slua/internal/sprites"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect <sprite-name>",
	Short: "Connect to a Sprite console session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cli := &sprites.CLI{Org: org}
		c := cli.ConsoleCmd(args[0])
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		return c.Run()
	},
}

func init() {
	rootCmd.AddCommand(connectCmd)
}
