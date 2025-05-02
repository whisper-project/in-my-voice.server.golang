/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"log"

	"github.com/spf13/cobra"
)

var studyCmd = &cobra.Command{
	Use:   "study",
	Short: "Manage study policy and reports.",
	Long: `Determine policies covering data collection,
and get reports on data collected.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		log.Fatal("You must specify a subcommand.")
	},
}

func init() {
	rootCmd.AddCommand(studyCmd)
}
