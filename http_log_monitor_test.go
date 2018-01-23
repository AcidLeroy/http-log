package monitor

import (
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/acidleroy/logparse"
)

func TestParseLogText(t *testing.T) {
	l, err := logparse.Common(`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:36 -0700] "GET /apache_pb.gif HTTP/1.0" 200 2326`)
	if err != nil {
		log.Fatal(err)
	}

	var expectedTime int64 = 971211336
	if expectedTime != l.Time.Unix() {
		t.Errorf("The time stamps don't match! Expected %d but go %d instead", expectedTime, l.Time.Unix())
	}

	expectedHost := `127.0.0.1`
	if expectedHost != l.Host.String() {
		t.Errorf("The host does not match! Expected %s, but got %s instead.", expectedHost, l.Host.String())
	}

	fmt.Println("The site is =", l.Request.URL)
	expectedURL := `/apache_pb.gif`
	if expectedURL != l.Request.URL.String() {
		t.Errorf("The URLs dont't match! Expected %s but got %s instead.", expectedURL, l.Request.URL)
	}
}

func TestGetSectionFromUrl(t *testing.T) {
	testSite := "http://my.site.com/pages/create"
	actual := GetSectionFromURL(testSite)

	expected := "my.site.com/pages"
	if expected != actual {
		t.Errorf("Expected %s but instead got %s", expected, actual)
	}
}

func TestLogStats(t *testing.T) {
	entries := []string{
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:36 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:37 -0700] "POST http://my.site.com/pets/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:38 -0700] "GET http://my.site.com/pages/view HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:39 -0700] "GET http://my.site.com/pets/find HTTP/1.0" 200 2326`}

	avg := NewOverallTimeAverage()
	rollingAverage := NewRollingTimeAverage(2)
	stats := NewLogStats("my.site.com", avg, rollingAverage, 1.0)
	for _, v := range entries {
		err := stats.ProcessEntry(&v)
		if err != nil {
			t.Errorf("Failed to process log entry! %s", err)
		}
	}

	if stats.TotalSiteRequests() != len(entries) {
		t.Errorf("Unexpected site requests! Expected %d but received %d instead", len(entries), stats.TotalSiteRequests())
	}

	expectedSites := 2
	if expectedSites != stats.UniqueSiteVisits() {
		t.Errorf("Expected to have %d sites visited, but instead got %d", expectedSites, stats.UniqueSiteVisits())
	}
}

func TestPopularSections(t *testing.T) {
	entries := []string{
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:37 -0700] "POST http://my.site.com/pets/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:36 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:38 -0700] "GET http://my.site.com/pages/view HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:39 -0700] "GET http://my.site.com/pages/find HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:40 -0700] "GET http://my.site.com/store/view HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:41 -0700] "GET http://my.site.com/store/buy HTTP/1.0" 200 2326`,
	}

	stats := NewLogStatsDefault("my.site.com")
	for _, v := range entries {
		err := stats.ProcessEntry(&v)
		if err != nil {
			t.Errorf("Failed to process log entry! %s", err)
		}
	}

	sections := stats.PopularSections()
	number1 := "my.site.com/pages"
	//number2 := "my.site.com/pets"

	if 3 != len(sections) {
		t.Errorf("Expected two sections but instead got %d.", len(sections))
		t.FailNow()
	}
	if number1 != sections[0].sectionName {
		t.Errorf("Expected the top ranking section to be %s but got %s instead.", number1, sections[0].sectionName)
	}
}

func TestUpdateSectionStats(t *testing.T) {
	roll := NewRollingTimeAverage(2)
	s := NewSectionStats("test", roll)
	fmt.Println("s = ", s)
	var ts int64 = 1
	UpdateSectionStats(s, ts)
	if s.accessesPerMinute != nil {
		t.Errorf("Insufficient data to calculate accesses per minute given only one statistic!")
	}
	ts++
	UpdateSectionStats(s, ts)
	ts++
	UpdateSectionStats(s, ts)
	if *s.firstAccess != 1 {
		t.Errorf("The first access should be zero and instead it is: %d.", s.firstAccess)
	}

}

func TestRollingTimeAverage(t *testing.T) {
	// Use one minute as convenience
	var rollingAvgTime int64 = 1
	avg := NewRollingTimeAverage(rollingAvgTime)
	if avg.avgMin != 0 {
		t.Errorf("Initial average should be zero!")
		t.FailNow()
	}
	var ts int64
	for i := 0; i < 8; i++ {
		avg.Update(ts)
		ts += 10
	}

	fmt.Println("The ts after frist loop = ", ts)

	var expectedAvg float32 = 6.0
	if avg.avgMin != expectedAvg {
		t.Errorf("The average should be %f, but got %f instead.", expectedAvg, avg.avgMin)
	}

	for i := 0; i < 60; i++ {
		avg.Update(ts)
		ts++
	}

	fmt.Println("The ts after second loop = ", ts)
	expectedAvg = 60.0
	if avg.avgMin != expectedAvg {
		t.Errorf("The average should be %f, but got %f instead.", expectedAvg, avg.avgMin)
	}
}

func TestUpdateSectionStatsOneMinuteAverage(t *testing.T) {
	// Use one minute as convenience
	var rollingAvgTime int64 = 1
	roll := NewRollingTimeAverage(rollingAvgTime)
	s := NewSectionStats("test", roll)
	fmt.Println("s = ", s)

	var ts int64 = 1 // start at time 0
	// for first minute do 1 access a second
	for i := 0; i < 7; i++ {
		UpdateSectionStats(s, ts)
		ts += 10
	}

	if *s.accessesPerMinute != s.rollingAverage.avgMin {
		t.Errorf("The overall average and the rolling average should be the same, but instead the overall average = %f  and the rolling average = %f", *s.accessesPerMinute, s.rollingAverage.avgMin)
	}

	// For second minute do 1 access every 10 seconds
	for i := 0; i < 8; i++ {
		UpdateSectionStats(s, ts)
		ts += 10
	}
	var expectedRollingAccesesMin float32 = 6.0
	if expectedRollingAccesesMin != s.rollingAverage.avgMin {
		t.Errorf("Expected %f accesses in the rolling average but got %f instead!", expectedRollingAccesesMin, s.rollingAverage.avgMin)
		t.FailNow()
	}

}

func TestAverages(t *testing.T) {
	entries := []string{ // 3 *60/(120)
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:56:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:57:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
	}
	stats := NewLogStatsDefault("my.site.com")
	for _, v := range entries {
		err := stats.ProcessEntry(&v)
		if err != nil {
			t.Errorf("Failed to process log entry! %s", err)
		}
	}

	var accessesPerMinute float32 = 1.0
	section := "my.site.com/pages"
	actual, err := stats.AccessesPerMinute(section)

	if err != nil {
		t.Errorf("There was a problem accessing section %s: %s", section, err)
		t.FailNow()
	}

	if accessesPerMinute != actual {
		t.Errorf("Expected to have an average of %f but instead got %f for section %s. ", accessesPerMinute, actual, section)
	}
}

func TestProcessEntryError(t *testing.T) {
	entry := "This is not a properly formatted string"
	stats := new(LogStats)
	err := stats.ProcessEntry(&entry)
	if err == nil {
		t.Errorf("We should get an error if we get an impropery formatted string!")
	}
}

func TestLogReader(t *testing.T) {

	// Write some data to a file.
	fName := "junk_log.txt"
	f, err := os.Create(fName)
	if err != nil {
		t.Errorf("There was an issure opening the file!")
		t.FailNow()
	}

	numWrites := 5
	lineToWrite := "this is my file!"
	for i := 0; i < numWrites; i++ {
		_, err2 := f.Write([]byte(lineToWrite + "\n"))
		if err2 != nil {
			t.Errorf("There was an issue writing to the file!")
			t.FailNow()
		}
	}
	defer f.Close()

	r := NewLogReader(fName)
	entries, err1 := r.GetNewLogEntries()
	if err1 != nil {
		t.Errorf("There was an error reading the file in Log reader! %s", err1)
		t.FailNow()
	}

	if numWrites != len(entries) {
		t.Errorf("Expected to have %d writes, but only got %d", numWrites, len(entries))
		t.FailNow()
	}

	if lineToWrite != entries[0] {
		t.Errorf("Expected the files to have %s but instead got %s", lineToWrite, entries[0])
	}
	entries, _ = r.GetNewLogEntries()
	if entries != nil {
		t.Errorf("Nothing in the file has changed, entries should be nil!")
		t.FailNow()
	}
	newEntry := "42"
	_, err2 := f.WriteString(newEntry + "\n")
	if err2 != nil {
		t.Errorf("There was an error with wring the new entry %s", err2)
	}
	entries, err3 := r.GetNewLogEntries()
	if err3 != nil {
		t.Errorf("There was a problem getting the new log entries %s", err3)
	}
	if len(entries) != 1 {
		t.Errorf("Got the wrong number of entries!")
	}

	if newEntry != entries[0] {
		t.Errorf("Expected %s but got %s instead!", newEntry, entries[0])
	}

}

func TestOverallAverage(t *testing.T) {
	ts := int64(0)
	min := 60
	o := NewOverallTimeAverage()
	for i := 0; i < min; i++ {
		o.Update(ts)
		ts++
	}

	expected := float32(60.0) // 60 calls a minute
	if expected != o.avgMin {
		t.Errorf("The average was expected to be %f but got %f instead.", expected, o.avgMin)
	}
}

func TestDifferentSite(t *testing.T) {
	entries := []string{ // 3 *60/(120)
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:55:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:56:00 -0700] "POST http://other-site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:57:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
	}
	stats := NewLogStatsDefault("my.site.com")
	for _, v := range entries {
		err := stats.ProcessEntry(&v)
		if err != nil {
			t.Errorf("Failed to process log entry! %s", err)
		}
	}

	if 2 != stats.totalSiteRequests {
		t.Errorf("main site was only accessed 2 times, not %d", stats.totalSiteRequests)
	}
}

func TestNoTwoMinuteAlert(t *testing.T) {

	// Access site once a minute for 5 minutes, validate that alarm wont' be set.
	entries := []string{
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:00:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:01:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:02:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:03:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:04:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
	}

	stats := NewLogStatsDefault("my.site.com")
	for _, v := range entries {
		err := stats.ProcessEntry(&v)
		if err != nil {
			t.Errorf("Failed to process log entry! %s", err)
		}
		if stats.highTrafficAlarm == true {
			t.Errorf("Threshold exceeded when it should not have been. Rolling average = %f", stats.rollingAvg.avgMin)
		}
	}
}

func TestTwoMinuteAlert(t *testing.T) {

	// Access site twice a minute for 2 minutes, validate that alarm will be set.
	entries := []string{
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:00:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:00:30 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:01:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:01:30 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:02:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
	}

	stats := NewLogStatsDefault("my.site.com")
	for _, v := range entries {
		err := stats.ProcessEntry(&v)
		if err != nil {
			t.Errorf("Failed to process log entry! %s", err)
		}
	}

	if stats.highTrafficAlarm == false {
		t.Errorf("Alarm should have been set!")
	}

}

func TestTwoMinuteAlertReturnToNormal(t *testing.T) {
	// Access site twice a minute for 2 minutes, then once a minute for two minuts after.
	// validate that alarm is unset
	entries := []string{
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:00:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:00:30 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:01:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:01:30 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:02:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:03:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:04:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
	}

	stats := NewLogStatsDefault("my.site.com")
	for _, v := range entries {
		err := stats.ProcessEntry(&v)
		if err != nil {
			t.Errorf("Failed to process log entry! %s", err)
		}
	}

	if stats.highTrafficAlarm == true {
		t.Errorf("Alarm should NOT be set!")
	}

}

func TestPrintStats(t *testing.T) {

	entries := []string{
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:00:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:00:30 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:01:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:01:30 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:02:00 -0700] "POST http://my.site.com/sites/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:03:00 -0700] "POST http://my.site.com/pages/create HTTP/1.0" 200 2326`,
		`127.0.0.1 user-identifier frank [10/Oct/2000:13:04:00 -0700] "POST http://my.site.com/sites/create HTTP/1.0" 200 2326`,
	}

	stats := NewLogStatsDefault("my.site.com")
	stats.PrintPopulartSections(10)
	for _, v := range entries {
		err := stats.ProcessEntry(&v)
		if err != nil {
			t.Errorf("Failed to process log entry! %s", err)
		}
	}

	stats.PrintPopulartSections(10)

}
