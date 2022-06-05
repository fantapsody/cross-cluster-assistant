package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
)

var rootCmd = &cobra.Command{
	Use: "cca",
}

func Execute() {
	rootCmd.AddCommand()
	AddSyncSvcCmd(rootCmd)
	AddDelSyncedSvcCmd(rootCmd)
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
