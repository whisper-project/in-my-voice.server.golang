/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"log"
	"slices"
)

// reportCmd represents the report command
var reportCmd = &cobra.Command{
	Use:   "report",
	Short: "Report on study participants",
	Long: `Provides a report on all study participants.
Uses separate sections for those not yet assigned to a profile,
those assigned to a profile, and those who have left the study.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		env, _ := cmd.Flags().GetString("env")
		err := platform.PushConfig(env)
		if err != nil {
			log.Fatalf("Can't load configuration: %v", err)
		}
		defer platform.PopConfig()
		reportParticipants()
	},
}

func init() {
	participantsCmd.AddCommand(reportCmd)
	participantsCmd.Args = cobra.NoArgs
	participantsCmd.Flags().StringP("env", "e", "development", "The environment to run in")
}

func reportParticipants() {
	reportAvailable()
	fmt.Println()
	reportAssigned()
	fmt.Println()
	reportUnassigned()
}

func reportAvailable() {
	fmt.Printf("These participant IDs have not yet been assigned:\n")
	ids, err := storage.AvailableParticipantIdsUtility()
	if err != nil {
		log.Fatalf("Failed to get available participants: %v", err)
	}
	if len(ids) == 0 {
		fmt.Printf("\t(none)\n")
	}
	for _, p := range ids {
		fmt.Printf("\t%s\n", p)
	}
}

func reportAssigned() {
	fmt.Printf("Assigned profiles and participant IDs:\n")
	forward, err := storage.AssignedProfilesParticipantIdsUtility()
	if err != nil {
		log.Fatalf("Failed to get profile to participant ID mapping: %v", err)
	}
	used, err := storage.UsedParticipantIdsUtility()
	if err != nil {
		log.Fatalf("Failed to get used participant IDs: %v", err)
	}
	if len(forward) == 0 {
		fmt.Printf("\t(none)\n")
	}
	reverse := make(map[string]string, len(forward))
	var inUse []string
	for profileId, studyId := range forward {
		reverse[studyId] = profileId
		inUse = append(inUse, studyId)
		fmt.Printf("\t%s\t%s\n", profileId, studyId)
	}
	slices.Sort(inUse)
	slices.Sort(used)
	var assignedButNotUsed []string
	var usedButNotAssigned []string
	for i, j := 0, 0; i < len(used) || j < len(inUse); {
		if i == len(used) {
			assignedButNotUsed = append(assignedButNotUsed, inUse[j])
			j += 1
		} else if j == len(inUse) {
			usedButNotAssigned = append(usedButNotAssigned, used[i])
			i += 1
		} else if used[i] == inUse[j] {
			i += 1
			j += 1
		} else if used[i] < inUse[j] {
			usedButNotAssigned = append(usedButNotAssigned, used[i])
			i += 1
		} else {
			assignedButNotUsed = append(assignedButNotUsed, inUse[j])
			j += 1
		}
	}
	if len(assignedButNotUsed) > 0 {
		fmt.Printf("\nThese participant IDs are assigned to profiles, but not marked used:\n")
		for _, p := range assignedButNotUsed {
			fmt.Printf("\t%s\n", p)
		}
	}
	if len(usedButNotAssigned) > 0 {
		fmt.Printf("\nThese participant IDs are marked used, but not assigned to a profile:\n")
		for _, p := range usedButNotAssigned {
			fmt.Printf("\t%s\n", p)
		}
	}
}

func reportUnassigned() {
	fmt.Printf("These participant IDs were assigned, but their users have left the study:\n")
	ids, err := storage.UnassignedParticipantIdsUtility()
	if err != nil {
		log.Fatalf("Failed to get unassigned participants: %v", err)
	}
	if len(ids) == 0 {
		fmt.Printf("\t(none)\n")
	}
	for _, p := range ids {
		fmt.Printf("\t%s\n", p)
	}
}
