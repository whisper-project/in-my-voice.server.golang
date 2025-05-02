/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/whisper-project/in-my-voice.server.golang/handlers"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"log"
	"os"
	"slices"
	"text/template"
)

var participantsReportCmd = &cobra.Command{
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
	participantsCmd.AddCommand(participantsReportCmd)
	participantsCmd.Args = cobra.NoArgs
	participantsCmd.Flags().StringP("env", "e", "development", "The environment to run in")
}

var report = template.Must(template.New("report").Parse(`
{{ range . }}
	{{- .UPN}}
	{{- if .Assigned -}}
		, Assigned: {{ .Assigned }}
		{{- if .Configured -}}
			, Configured: {{ .Configured }}
			{{- if .Started -}}
				, Started: {{ .Started }}
				{{- if .Finished -}}
					, Finished: {{ .Finished }}
				{{- end -}}
			{{- end -}}
		{{- end -}}
	{{- else }} (not assigned)
	{{- end }}
{{ end }}`))

func reportParticipants() {
	participants, err := storage.GetAllStudyParticipants()
	if err != nil {
		log.Fatalf("Failed to get all study participants: %v", err)
	}
	slices.SortFunc(participants, handlers.CompareParticipantsFunc("assign"))
	var lines []map[string]string
	for _, p := range participants {
		lines = append(lines, handlers.MakeParticipantMap(p))
	}
	if len(lines) == 0 {
		fmt.Println("No participants.")
	} else {
		if err := report.Execute(os.Stdout, lines); err != nil {
			log.Fatalf("error executing report template: %v", err)
		}
	}
}
