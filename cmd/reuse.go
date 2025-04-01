/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"log"

	"github.com/spf13/cobra"
)

// reuseCmd represents the reuse command
var reuseCmd = &cobra.Command{
	Use:   "reuse",
	Short: "Reuse unassigned participant IDs.",
	Long: `When people leave a study, their IDs kept in a holding area
This command makes those IDs available for reuse with new participants.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		env, _ := cmd.Flags().GetString("env")
		err := platform.PushConfig(env)
		if err != nil {
			log.Fatalf("Can't load configuration: %v", err)
		}
		defer platform.PopConfig()
		if err = storage.ReuseUnassignedParticipantsUtility(); err != nil {
			log.Fatalf("Failed to reuse unassigned participants: %v", err)
		}
	},
}

func init() {
	participantsCmd.AddCommand(reuseCmd)
	reuseCmd.Flags().StringP("env", "e", "development", "The environment to run in")
}
