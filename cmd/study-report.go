/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"github.com/spf13/cobra"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"log"
	"os"
	"strings"
)

var studyReportCmd = &cobra.Command{
	Use:   "report [args] [output-file-path]",
	Short: "Report on study usage data",
	Long: `This command produces CSV reports on study usage data.
By default, all typed-line data is reported, but you can use flags to restrict
the dates being reported and the users covered in the report. You can also
use a flag to switch to a repeated-phrase report. If the given file path
names a directory, a report filename is generated from the report type and date range.
If no filepath is given, the report will be saved to the system temp file directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetFlags(0)
		env, _ := cmd.Flags().GetString("env")
		if err := platform.PushConfig(env); err != nil {
			log.Fatalf("Can't load %q configuration: %v", env, err)
		}
		phrases, _ := cmd.Flags().GetCount("phrases")
		firstDate, _ := cmd.Flags().GetString("first-date")
		lastDate, _ := cmd.Flags().GetString("last-date")
		studyOnly, _ := cmd.Flags().GetCount("study-only")
		upn, _ := cmd.Flags().GetString("upn")
		pathname := ""
		if len(args) > 0 {
			pathname = args[0]
		}
		if phrases > 0 {
			ReportPhrases(pathname, studyOnly > 0)
		} else {
			start, end, err := storage.ComputeReportDates(firstDate, lastDate, "1/2/06")
			if err != nil {
				log.Fatalf("Invalid first or last date (must be in m/d/yy format)")
			}
			var upns []string
			if upn != "" {
				upns = strings.Split(upn, ",")
			}
			ReportLines(pathname, start, end, studyOnly > 0, upns)
		}
	},
}

func init() {
	studyCmd.AddCommand(studyReportCmd)
	studyReportCmd.Args = cobra.MaximumNArgs(1)
	studyReportCmd.Flags().StringP("env", "e", "development", "Environment to report from")
	studyReportCmd.Flags().CountP("phrases", "p", "Report on phrases instead of lines")
	studyReportCmd.Flags().StringP("first-date", "f", "", "First date to report on (m/d/y)")
	studyReportCmd.Flags().StringP("last-date", "l", "", "Last date to report on (m/d/y)")
	studyReportCmd.Flags().CountP("study-only", "s", "Restrict report to study participants")
	studyReportCmd.Flags().StringP("upn", "u", "", "Restrict report to specific UPNs (comma-separated list)")
	studyReportCmd.MarkFlagsMutuallyExclusive("phrases", "first-date", "last-date", "upn")
}

func ReportLines(pathname string, start int64, end int64, studyOnly bool, upns []string) {
	report := storage.NewStudyReport(storage.ReportTypeLines, start, end, studyOnly, upns)
	name := generateName(pathname, report)
	if err := report.Generate(name); err != nil {
		log.Fatalf("Error: %v", err)
	}
	log.Printf("Report saved to file: %s", name)
}

func ReportPhrases(pathname string, studyOnly bool) {
	report := storage.NewStudyReport(storage.ReportTypeLines, 0, 0, studyOnly, nil)
	name := generateName(pathname, report)
	if err := report.Generate(name); err != nil {
		log.Fatalf("Error: %v", err)
	}
	log.Printf("Report saved to file: %s", name)
}

func generateName(pathname string, report *storage.StudyReport) string {
	if pathname == "" {
		pathname = os.TempDir()
	}
	stat, err := os.Stat(pathname)
	if err == nil && stat.IsDir() {
		pathname = pathname + "/" + report.Filename
	}
	if !strings.HasSuffix(strings.ToLower(pathname), ".xlsx") {
		pathname += ".xlsx"
	}
	return pathname
}
