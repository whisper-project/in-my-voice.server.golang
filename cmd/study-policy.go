/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"fmt"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"log"

	"github.com/spf13/cobra"
)

var studyPolicyCmd = &cobra.Command{
	Use:   "policy",
	Short: "Set data collection policies",
	Long: `Determine from whom and how data is collected from users of the app.
Defaults are to collect from all users, to pool all data from users
who are not participating in a study, and to segregate data collected from study
participants about repeated phrases from similar data collected from other users.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		env, _ := cmd.Flags().GetString("env")
		if err := platform.PushConfig(env); err != nil {
			log.Fatalf("Can't load %q configuration: %v", env, err)
		}
		useDefaults, _ := cmd.Flags().GetCount("defaults")
		collectAll, _ := cmd.Flags().GetCount("collect-all")
		noCollectAll, _ := cmd.Flags().GetCount("no-collect-all")
		poolNonStudyUsers, _ := cmd.Flags().GetCount("pool-non-study-users")
		noPoolNonStudyUsers, _ := cmd.Flags().GetCount("no-pool-non-study-users")
		separateNonStudyPhrases, _ := cmd.Flags().GetCount("separate-non-study-phrases")
		noSeparateNonStudyPhrases, _ := cmd.Flags().GetCount("no-separate-non-study-phrases")
		if useDefaults > 0 {
			storage.SetStudyPolicies(storage.DefaultStudyPolicies)
		} else {
			setPolicy(
				collectAll-noCollectAll,
				poolNonStudyUsers-noPoolNonStudyUsers,
				separateNonStudyPhrases-noSeparateNonStudyPhrases,
			)
		}
		showPolicy()
	},
}

func init() {
	studyCmd.AddCommand(studyPolicyCmd)
	studyPolicyCmd.Args = cobra.NoArgs
	studyPolicyCmd.Flags().StringP("env", "e", "development", "The environment to run in")
	studyPolicyCmd.Flags().CountP("defaults", "d", "Use the default policies")
	studyPolicyCmd.Flags().CountP("collect-all", "a", "Collect from all users")
	studyPolicyCmd.Flags().CountP("no-collect-all", "A", "Collect from study participants only")
	studyPolicyCmd.Flags().CountP("pool-non-study-users", "p", "Pool user data from non-study participants")
	studyPolicyCmd.Flags().CountP("no-pool-non-study-users", "P", "Individually track user data from all users")
	studyPolicyCmd.Flags().CountP("separate-non-study-phrases", "s", "Separate phrase data from non-study participants")
	studyPolicyCmd.Flags().CountP("no-separate-non-study-phrases", "S", "Combine phrase data from all users")
	studyPolicyCmd.MarkFlagsMutuallyExclusive("defaults", "collect-all", "no-collect-all")
	studyPolicyCmd.MarkFlagsMutuallyExclusive("defaults", "pool-non-study-users", "no-pool-non-study-users")
	studyPolicyCmd.MarkFlagsMutuallyExclusive("defaults", "separate-non-study-phrases", "no-separate-non-study-phrases")
}

func setPolicy(collectAll, poolNonStudyUsers, separateNonStudyPhrases int) {
	policy := storage.GetStudyPolicies()
	if collectAll != 0 {
		policy.CollectNonStudyStats = collectAll > 0
	}
	if poolNonStudyUsers != 0 {
		policy.AnonymizeNonStudyLineStats = poolNonStudyUsers > 0
	}
	if separateNonStudyPhrases != 0 {
		policy.SeparateNonStudyRepeatStats = separateNonStudyPhrases > 0
	}
	storage.SetStudyPolicies(policy)
}

func showPolicy() {
	policy := storage.GetStudyPolicies()
	if policy.CollectNonStudyStats {
		fmt.Println("Data is being collected from all users of the app.")
		if policy.AnonymizeNonStudyLineStats {
			fmt.Println("Data collected from non-study participants is pooled.")
		} else {
			fmt.Println("Data collected from non-study participants is per-user.")
		}
		if policy.SeparateNonStudyRepeatStats {
			fmt.Println("Phrase data for study participants is separated from that for non-study participants.")
		} else {
			fmt.Println("Phrase data is not separated between study participants and non-study participants.")
		}
	} else {
		fmt.Println("Data is only being collected from study participants.")
	}
}
