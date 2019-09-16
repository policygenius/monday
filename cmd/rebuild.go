package main

import (
	"github.com/spf13/cobra"
)

var rebuildCmd = &cobra.Command{
	Use:   "rebuild",
	Short: "Cleans and rebuilds a doctor image",
	Run: func(cmd *cobra.Command, args []string) {
		var cmdArgs []string
		var service string
		service = args[0]
		cmdArgs = []string{"build", "--no-cache", service}
		execrunner("docker-compose", cmdArgs, service)
	},
}
