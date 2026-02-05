package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/JPM1118/slua/internal/sprites"
	"github.com/spf13/cobra"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Print Sprite status (non-interactive)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cli := &sprites.CLI{Org: org}
		spriteList, err := cli.List()
		if err != nil {
			return err
		}

		if len(spriteList) == 0 {
			fmt.Println("No Sprites running.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tUPTIME")
		fmt.Fprintln(w, "────\t──────\t──────")
		for _, s := range spriteList {
			fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Status, s.FormatUptime())
		}
		return w.Flush()
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
