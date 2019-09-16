package main

import (
	"github.com/spf13/cobra"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Helps you configure/debug your machine to use Bifrost",
	Run: func(cmd *cobra.Command, args []string) {
		var cmdArgs string
		cmdArgs = "./bifrost-doctor"
		dir := "bifrost"
		execrunner(cmdArgs, nil, dir)
	},
}