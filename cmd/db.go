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

// dbCmd represents the db command
var dbCmd = &cobra.Command{
	Use:   "db",
	Short: "Database utilities",
	Long: `This is the parent command for managing the database.
It must be invoked with a subcommand.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		log.Fatal("You must specify a subcommand.")
	},
}

func init() {
	rootCmd.AddCommand(dbCmd)
}
