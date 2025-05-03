/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package cmd

import (
	"fmt"
	"github.com/tealeg/xlsx/v3"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"github.com/whisper-project/in-my-voice.server.golang/storage"
	"log"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var studyReportCmd = &cobra.Command{
	Use:   "report [args] [output-file-path]",
	Short: "Report on study usage data",
	Long: `This command produces CSV reports on study usage data.
By default, all typed-line data is reported, but you can use flags to restrict
the dates being reported and the users covered in the report. You can also
use a flag to switch to a repeated-phrase report. If no output file path is
specified, or the given file path names a directory, a report filename
is generated from the report type and date range.`,
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
			reportPhrases(pathname, studyOnly > 0)
		} else {
			var start int64
			if firstDate != "" {
				d, err := time.ParseInLocation("1/2/06", firstDate, storage.AdminTZ)
				if err != nil {
					log.Fatalf("The date specification %q wasn't understood. Please use m/d/yy format.", firstDate)
				}
				start = d.UnixMilli()
			}
			end := time.Now().In(storage.AdminTZ)
			if lastDate != "" {
				d, err := time.ParseInLocation("1/2/06", lastDate, storage.AdminTZ)
				if err != nil {
					log.Fatalf("The date specification %q wasn't understood. Please use m/d/yy format.", lastDate)
				}
				end = d
			}
			// set the end value at the very end of the day
			end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, storage.AdminTZ)
			var upns []string
			if upn != "" {
				upns = strings.Split(upn, ",")
			}
			reportLines(pathname, start, end.UnixMilli(), studyOnly > 0, upns)
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

func reportLines(pathname string, start int64, end int64, studyOnly bool, upns []string) {
	name := generateFilename(pathname, "typed-lines", studyOnly, start, end)
	var stats [][]storage.TypedLineStat
	if len(upns) > 0 {
		stats = make([][]storage.TypedLineStat, 0, len(upns))
		for _, upn := range upns {
			stat, err := storage.FetchTypedLineStats(upn, start, end)
			if err != nil {
				log.Fatalf("Failed to fetch line stats for %q: %v", upn, err)
			}
			if len(stat) > 0 {
				stats = append(stats, stat)
			}
		}
	} else {
		var err error
		stats, err = storage.FetchAllTypedLineStats(start, end, studyOnly)
		if err != nil {
			log.Fatalf("Failed to fetch line stats: %v", err)
		}
	}
	userCount, statCount := 0, 0
	for _, s := range stats {
		userCount++
		statCount += len(s)
	}
	if statCount == 0 {
		log.Printf("There are no statistics for the selected period/users. No report will be generated.")
		return
	}
	log.Printf("Generating a report on %d users (%d lines) to path %q...", userCount, statCount, name)
	generateLinesReport(name, stats)
	log.Printf("Report saved to file: %s", name)
}

func reportPhrases(pathname string, studyOnly bool) {
	name := generateFilename(pathname, "repeated-phrases", studyOnly, 0, 0)
	stats, err := storage.FetchAllCannedLineStats(studyOnly)
	if err != nil {
		log.Fatalf("Failed to fetch phrase stats: %v", err)
	}
	if len(stats) == 0 {
		log.Printf("There are no repeated phrases for the selected users. No report will be generated.")
		return
	}
	log.Printf("Generating a report on %d repeated phrases to path %q...", len(stats), name)
	generatePhraseReport(name, stats)
	log.Printf("Report saved to file: %s", name)
}

func generateFilename(pathname, reportType string, studyOnly bool, start, end int64) string {
	prefix := "all-"
	if studyOnly {
		prefix = "study-"
	}
	if pathname == "" {
		pathname = "."
	}
	stat, err := os.Stat(pathname)
	if err == nil && stat.IsDir() {
		// generate a filename based on the report type and date range
		var startPart string
		if start > 0 {
			startDate := time.UnixMilli(start).In(storage.AdminTZ)
			startPart = fmt.Sprintf("from-%s-", startDate.Format("01-02-06"))
		}
		if end == 0 {
			end = time.Now().In(storage.AdminTZ).UnixMilli()
		}
		endDate := time.UnixMilli(end).In(storage.AdminTZ)
		endPart := fmt.Sprintf("thru-%s", endDate.Format("01-02-06"))
		if strings.HasSuffix(pathname, "/") {
			pathname = pathname[:len(pathname)-1]
		}
		pathname = pathname + "/" + prefix + reportType + "-" + startPart + endPart
	}
	if !strings.HasSuffix(strings.ToLower(pathname), ".xlsx") {
		pathname += ".xlsx"
	}
	return pathname
}

func generateLinesReport(name string, stats [][]storage.TypedLineStat) {
	xlsx.SetDefaultFont(12, "Arial")
	xf := xlsx.NewFile()
	xs, err := xf.AddSheet("Lines Report")
	if err != nil {
		log.Fatalf("Failed to create the report worksheet: %v", err)
	}
	headings := []string{"UPN", "When", "Key Count", "Char Count", "Time (ms)", "Platform"}
	headingsRow := xs.AddRow()
	headingStyle := xlsx.NewStyle()
	headingStyle.Alignment.Horizontal = "center"
	headingStyle.Font.Bold = true
	for _, h := range headings {
		cell := headingsRow.AddCell()
		cell.SetString(h)
		cell.SetStyle(headingStyle)
	}
	textStyle := xlsx.NewStyle()
	textStyle.Font.Bold = false
	textStyle.Alignment.Horizontal = "center"
	cols1to2 := xlsx.NewColForRange(1, 2)
	cols1to2.SetWidth(22)
	cols1to2.SetStyle(textStyle)
	xs.SetColParameters(cols1to2)
	cols3to5 := xlsx.NewColForRange(3, 5)
	cols3to5.SetWidth(11)
	xs.SetColParameters(cols3to5)
	col6 := xlsx.NewColForRange(6, 6)
	col6.SetWidth(15)
	col6.SetStyle(textStyle)
	xs.SetColParameters(col6)
	xlDateFormat := `yyyy/mm/dd hh:mm:ss.000`
	for _, user := range stats {
		for _, stat := range user {
			row := xs.AddRow()
			row.AddCell().SetString(stat.Upn)
			date := time.UnixMilli(stat.Completed).In(storage.AdminTZ)
			xlDate := xlsx.TimeToExcelTime(date, xf.Date1904)
			row.AddCell().SetDateTimeWithFormat(xlDate, xlDateFormat)
			row.AddCell().SetInt64(stat.Changes)
			row.AddCell().SetInt64(stat.Length)
			row.AddCell().SetInt64(stat.Duration)
			row.AddCell().SetString(storage.PlatformNames[stat.From])
		}
	}
	if err := xf.Save(name); err != nil {
		log.Fatalf("Failed to save the report to %q: %v", name, err)
	}
}

func generatePhraseReport(name string, stats []storage.CannedLineStat) {
	// the report is sorted by default: descending by total usage
	slices.SortFunc(stats, func(a, b storage.CannedLineStat) int {
		if diff := (a.FavoriteCount + a.RepeatCount) - (b.FavoriteCount + b.RepeatCount); diff > 0 {
			return -1
		} else if diff < 0 {
			return 1
		} else {
			return 0
		}
	})
	xlsx.SetDefaultFont(12, "Arial")
	xf := xlsx.NewFile()
	xs, err := xf.AddSheet("Phrases Report")
	if err != nil {
		log.Fatalf("Failed to create the report worksheet: %v", err)
	}
	headings := []string{"Total Count", "Favorite Count", "Repeat Count", "Content"}
	headingsRow := xs.AddRow()
	headingStyle := xlsx.NewStyle()
	headingStyle.Alignment.Horizontal = "center"
	headingStyle.Font.Bold = true
	for _, h := range headings {
		cell := headingsRow.AddCell()
		cell.SetString(h)
		cell.SetStyle(headingStyle)
	}
	xs.SetColWidth(1, 3, 15)
	xs.SetColWidth(4, 4, 80)
	for _, phrase := range stats {
		row := xs.AddRow()
		row.AddCell().SetInt64(phrase.FavoriteCount + phrase.RepeatCount)
		row.AddCell().SetInt64(phrase.FavoriteCount)
		row.AddCell().SetInt64(phrase.RepeatCount)
		row.AddCell().SetString(phrase.Content)
	}
	if err := xf.Save(name); err != nil {
		log.Fatalf("Failed to save the report to %q: %v", name, err)
	}
}
