/*
 * Copyright 2025 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package storage

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"github.com/tealeg/xlsx/v3"
	"github.com/whisper-project/in-my-voice.server.golang/platform"
	"go.uber.org/zap"
	"io"
	"os"
	"path"
	"slices"
	"time"
)

const (
	ReportTypeLines   = "typed-lines"
	ReportTypePhrases = "repeated-phrases"
)

type StudyReport struct {
	Id        string
	Type      string
	Start     int64
	End       int64
	StudyOnly bool
	Upns      []string
	Filename  string
	Generated int64
	Stored    bool
}

func (s *StudyReport) StoragePrefix() string {
	return "study-report:"
}
func (s *StudyReport) StorageId() string {
	return s.Id
}
func (s *StudyReport) ToRedis() ([]byte, error) {
	var b bytes.Buffer
	if err := gob.NewEncoder(&b).Encode(s); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
func (s *StudyReport) FromRedis(b []byte) error {
	*s = StudyReport{} // dump old data
	return gob.NewDecoder(bytes.NewReader(b)).Decode(s)
}

func (s *StudyReport) Generate(dest string) (err error) {
	switch s.Type {
	case ReportTypeLines:
		var stats [][]TypedLineStat
		stats, err = FetchAllTypedLineStats(s.Start, s.End, s.StudyOnly, s.Upns)
		if err == nil {
			err = generateLinesReport(dest, stats)
		}
	case ReportTypePhrases:
		var stats []CannedLineStat
		stats, err = FetchAllCannedLineStats(s.StudyOnly)
		if err == nil {
			err = generatePhraseReport(dest, stats)
		}
	default:
		err = fmt.Errorf("unknown report type: %s", s.Type)
	}
	if err == nil {
		s.Generated = time.Now().UnixMilli()
	}
	return
}

func (s *StudyReport) DisplayName() string {
	prefix := "all-"
	if len(s.Upns) > 0 {
		prefix = "upns-"
	} else if s.StudyOnly {
		prefix = "study-"
	}
	var startPart string
	if s.Start > 0 {
		startDate := time.UnixMilli(s.Start).In(AdminTZ)
		startPart = fmt.Sprintf("from-%s-", startDate.Format("01-02-06"))
	}
	if s.End == 0 {
		s.End = time.Now().In(AdminTZ).UnixMilli()
	}
	endDate := time.UnixMilli(s.End).In(AdminTZ)
	endPart := fmt.Sprintf("thru-%s", endDate.Format("01-02-06"))
	return prefix + s.Type + "-" + startPart + endPart
}

func (s *StudyReport) GenerateAndStore() error {
	localPath := path.Join(os.TempDir(), s.Id)
	if err := s.Generate(localPath); err != nil {
		sLog().Error("failed to generate a report",
			zap.Any("report", s), zap.Error(err))
		return err
	}
	f, err := os.Open(localPath)
	if err != nil {
		sLog().Error("failed to open the generated report file",
			zap.Any("report", s), zap.Error(err))
	}
	defer f.Close()
	if err = platform.S3PutEncryptedBlob(sCtx(), s.Id, f); err != nil {
		sLog().Error("failed to store the generated report",
			zap.Any("report", s), zap.Error(err))
		return err
	}
	s.Stored = true
	if err = platform.SaveObject(sCtx(), s); err != nil {
		sLog().Error("failed to save the report object",
			zap.Any("report", s), zap.Error(err))
		_ = os.Remove(localPath)
		_ = platform.S3DeleteBlob(sCtx(), s.Id)
		return err
	}
	return nil
}

func (s *StudyReport) Retrieve() (io.ReadCloser, error) {
	localPath := path.Join(os.TempDir(), s.Id)
	if _, err := os.Stat(localPath); err == nil {
		return os.Open(localPath)
	}
	f, err := os.Create(localPath)
	if err != nil {
		sLog().Error("failed to create the local report file",
			zap.Any("report", s), zap.Error(err))
		return nil, err
	}
	if err = platform.S3GetEncryptedBlob(sCtx(), s.Id, f); err != nil {
		f.Close()
		_ = os.Remove(localPath)
		sLog().Error("failed to retrieve the report from S3",
			zap.Any("report", s), zap.Error(err))
		return nil, err
	}
	if _, err := f.Seek(0, 0); err != nil {
		f.Close()
		_ = os.Remove(localPath)
		sLog().Error("failed to seek to the beginning of the report file",
			zap.Any("report", s), zap.Error(err))
		return nil, err
	}
	return f, nil
}

func (s *StudyReport) Delete() error {
	localPath := path.Join(os.TempDir(), s.Id)
	if _, err := os.Stat(localPath); err == nil {
		if err := os.Remove(localPath); err != nil {
			sLog().Error("failed to delete the local report file",
				zap.Any("report", s), zap.Error(err))
		}
	}
	if s.Stored {
		if err := platform.S3DeleteBlob(sCtx(), s.Id); err != nil {
			sLog().Error("failed to delete the report from S3",
				zap.Any("report", s), zap.Error(err))
		}
	}
	err := platform.DeleteStorage(sCtx(), s)
	if err != nil {
		sLog().Error("failed to delete the report from storage",
			zap.Any("report", s), zap.Error(err))
	}
	return err
}

func NewStudyReport(reportType string, start, end int64, studyOnly bool, upns []string) *StudyReport {
	s := &StudyReport{
		Id:        uuid.NewString(),
		Type:      reportType,
		Start:     start,
		End:       end,
		StudyOnly: studyOnly,
		Upns:      upns,
	}
	s.Filename = s.DisplayName() + ".xlsx"
	return s
}

func GetStudyReport(id string) (*StudyReport, error) {
	s := &StudyReport{Id: id}
	if err := platform.LoadObject(sCtx(), s); err != nil {
		if errors.Is(err, platform.NotFoundError) {
			return nil, nil
		}
		sLog().Error("db failure on report fetch", zap.String("id", id), zap.Error(err))
		return nil, err
	}
	return s, nil
}

func FetchAllStudyReports() ([]*StudyReport, error) {
	var results []*StudyReport
	s := &StudyReport{}
	mapper := func() error {
		n := *s
		results = append(results, &n)
		return nil
	}
	if err := platform.MapObjects(sCtx(), mapper, s); err != nil {
		sLog().Error("failure mapping over study reports", zap.Error(err))
		return nil, err
	}
	return results, nil
}

func ComputeReportDates(startString, endString, dateFormat string) (start, end int64, err error) {
	var d time.Time
	if startString != "" {
		d, err = time.ParseInLocation(dateFormat, startString, AdminTZ)
		if err != nil {
			return
		}
		start = d.UnixMilli()
	}
	if endString != "" {
		d, err = time.ParseInLocation(dateFormat, endString, AdminTZ)
		if err != nil {
			return
		}
	} else {
		d = time.Now().In(AdminTZ)
	}
	end = time.Date(d.Year(), d.Month(), d.Day(), 23, 59, 59, 999999999, AdminTZ).UnixMilli()
	return
}

func generateLinesReport(name string, stats [][]TypedLineStat) error {
	// the report is not sorted
	xlsx.SetDefaultFont(12, "Arial")
	xf := xlsx.NewFile()
	xs, err := xf.AddSheet("Lines Report")
	if err != nil {
		return fmt.Errorf("failed to create the report worksheet: %w", err)
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
			date := time.UnixMilli(stat.Completed).In(AdminTZ)
			xlDate := xlsx.TimeToExcelTime(date, xf.Date1904)
			row.AddCell().SetDateTimeWithFormat(xlDate, xlDateFormat)
			row.AddCell().SetInt64(stat.Changes)
			row.AddCell().SetInt64(stat.Length)
			row.AddCell().SetInt64(stat.Duration)
			row.AddCell().SetString(PlatformNames[stat.From])
		}
	}
	if err := xf.Save(name); err != nil {
		return fmt.Errorf("failed to save the report to %q: %w", name, err)
	}
	return nil
}

func generatePhraseReport(name string, stats []CannedLineStat) error {
	// the report is sorted by default: descending by total usage
	slices.SortFunc(stats, func(a, b CannedLineStat) int {
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
		return fmt.Errorf("failed to create the report worksheet: %v", err)
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
	if err = xf.Save(name); err != nil {
		return fmt.Errorf("failed to save the report to %q: %w", name, err)
	}
	return nil
}
