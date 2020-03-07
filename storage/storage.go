package storage

import (
	"fmt"
	"io"
	"izettle-daily-reports/izettle"
	"regexp"
	"strings"
	"time"

	"google.golang.org/api/drive/v3"
)

type MissingPDF struct {
	Report izettle.DayReport
	Dir    *drive.File
}

func Children(ds *drive.Service, directoryID string) (map[string]*drive.File, error) {
	folderList, err := ds.Files.List().
		Q(fmt.Sprintf("'%s' in parents", directoryID)).
		IncludeItemsFromAllDrives(true).
		SupportsTeamDrives(true).
		Do()
	if err != nil {
		return nil, err
	}
	folders := make(map[string]*drive.File)
	for _, folder := range folderList.Files {
		folders[folder.Name] = folder
	}
	return folders, nil
}

func UploadPDF(ds *drive.Service, report MissingPDF, data io.Reader) error {
	isYear := regexp.MustCompile("\\d+")
	dir := report.Dir
	if !isYear.Match([]byte(dir.Name)) {
		return fmt.Errorf("failed to find year directory to uppload PDF:s to. Please create the year directory for date %s and %s and try again", dir.Name, report.Report.User.Name)
	}
	f := &drive.File{
		MimeType: "application/pdf",
		Name:     ReportFileName(report.Report),
		Parents:  []string{report.Dir.Id},
	}
	_, err := ds.Files.
		Create(f).
		SupportsTeamDrives(true).
		Media(data).
		Do()
	return err
}

func MissingPDFs(ds *drive.Service, root *drive.File, reports []izettle.DayReport) ([]MissingPDF, error) {
	missing := make([]MissingPDF, 0)
	today := getToday()

	reportYears := make(map[string][]izettle.DayReport)
	for _, report := range reports {
		year := strings.Split(report.Date, "-")[0]
		reportYears[year] = append(reportYears[year], report)
	}
	yearDirList, err := ds.Files.List().
		Q(fmt.Sprintf("'%s' in parents", root.Id)).
		IncludeItemsFromAllDrives(true).
		SupportsTeamDrives(true).
		Do()
	if err != nil {
		return nil, err
	}
	dirYears := make(map[string]*drive.File)
	for _, file := range yearDirList.Files {
		dirYears[file.Name] = file
	}

	for year, reports := range reportYears {
		yearDir, ok := dirYears[year]
		if !ok {
			for _, report := range reports {
				if report.Date == today {
					continue
				}
				missing = append(missing, MissingPDF{Report: report, Dir: root})
			}
			continue
		}
		files, err := ds.Files.List().
			Q(fmt.Sprintf("'%s' in parents", yearDir.Id)).
			IncludeItemsFromAllDrives(true).
			SupportsTeamDrives(true).
			Do()
		if err != nil {
			return nil, err
		}
		filesByDate := make(map[string]*drive.File)
		for _, file := range files.Files {
			date, err := fileDate(file.Name)
			if err != nil {
				continue
			}
			filesByDate[date] = file
		}
		for _, report := range reports {
			if _, ok := filesByDate[report.Date]; !ok {
				if report.Date == today {
					continue
				}
				missing = append(missing, MissingPDF{Report: report, Dir: yearDir})
			}
		}
	}

	return missing, nil
}

func ReportFileName(report izettle.DayReport) string {
	return fmt.Sprintf("%s_%s.pdf", report.User.Name, report.Date)
}

func fileDate(name string) (string, error) {
	getDate := regexp.MustCompile(".*_(\\d+-\\d+-\\d+).pdf")
	date := getDate.FindStringSubmatch(name)
	if date == nil {
		return "", fmt.Errorf("failed to get date from file")
	}
	return date[1], nil
}

func getToday() string {
	year, month, day := time.Now().Date()
	return fmt.Sprintf("%d-%d-%d", year, month, day)
}
