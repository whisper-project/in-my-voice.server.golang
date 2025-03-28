/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"github.com/spf13/cobra"
	"log"
)

// participantsCmd represents the participants command
var participantsCmd = &cobra.Command{
	Use:   "participants",
	Short: "Manage study participants",
	Long: `Report on, add, and remove study participants.
Specify what to do with subcommands.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		log.Fatal("You must specify a subcommand.")
	},
}

func init() {
	rootCmd.AddCommand(participantsCmd)
}
