package monitor

import (
	"bufio"
	"container/list"
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"regexp"
	"sort"

	"github.com/acidleroy/logparse"
)

type OverallTimeAverage struct {
	firstTs  *int64
	lastTs   int64
	accesses int64
	avgMin   float32
}

func (o *OverallTimeAverage) Update(ts int64) {

	if o.firstTs == nil {
		o.firstTs = new(int64)
		*o.firstTs = ts
		o.lastTs = ts
		o.avgMin = 0.0

	} else {
		if ts > o.lastTs {
			o.lastTs = ts
		}
		denom := float32(o.lastTs - *o.firstTs)
		if denom != 0 {
			o.avgMin = float32(60*o.accesses) / denom
		}
	}
	o.accesses++
}

func NewOverallTimeAverage() *OverallTimeAverage {
	n := new(OverallTimeAverage)
	return n
}

// RollingTimeAverage is a struct to keep track of the rolling time average.
type RollingTimeAverage struct {
	timeToKeepMin int64 // Time to keep rolling in minutes
	savedTimes    *list.List
	avgMin        float32
}

// NewRollingTimeAverage generates a new RollingTimeAverage struct given the
// timeToRoll parameter. TimeToRoll specifies how many minutes to roll
// the moving average.
func NewRollingTimeAverage(timeToRoll int64) *RollingTimeAverage {
	r := new(RollingTimeAverage)
	r.timeToKeepMin = timeToRoll
	r.savedTimes = list.New()
	return r
}

// Update Function to update the rolling time average given a unix epoch
func (r *RollingTimeAverage) Update(ts int64) {
	r.savedTimes.PushBack(ts)

	val := r.savedTimes.Front().Value.(int64)
	for (ts - val) > r.timeToKeepMin*60 {
		r.savedTimes.Remove(r.savedTimes.Front())
		val = r.savedTimes.Front().Value.(int64)
	}

	first := r.savedTimes.Front().Value.(int64)
	denom := float32(ts - first)

	if denom > 0 {
		r.avgMin = float32(r.savedTimes.Len()-1) * 60.0 / (denom)
	} else {
		r.avgMin = 0
	}
}

// SectionStats contains both the rolling average and the overall average for
// how many times a site has been accessed.
type SectionStats struct {
	sectionName       string
	totalAccess       int64
	accessesPerMinute *float32
	firstAccess       *int64
	lastAccess        *int64
	rollingAverage    *RollingTimeAverage
}

func (s *SectionStats) PrintStats() {
	fmt.Println("===== Stats for section ", s.sectionName, "=======")
	fmt.Println("Total Accesses: ", s.totalAccess)
	if s.accessesPerMinute != nil {
		fmt.Println("Average Accesses per minute: ", *s.accessesPerMinute)
	}
	if s.rollingAverage != nil {
		fmt.Println("Rolling average accesses per minute: ", s.rollingAverage.avgMin)
	}
	if s.firstAccess != nil {
		fmt.Println("First Access: ", *s.firstAccess)
	}

	if s.lastAccess != nil {
		fmt.Println("Last access: ", *s.lastAccess)
	}

	fmt.Println()
}

// NewSectionStats creates an object that has statistics about the site. You pass it
// the name of the section and inject a RollingTimeAverage object.
func NewSectionStats(sectionName string, rollingAverage *RollingTimeAverage) *SectionStats {
	s := new(SectionStats)
	s.sectionName = sectionName
	s.rollingAverage = rollingAverage
	return s
}

// ByTotalAccesses is the type used to sort statistics by totalAcceses
type ByTotalAccesses []*SectionStats

func (a ByTotalAccesses) Len() int           { return len(a) }
func (a ByTotalAccesses) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByTotalAccesses) Less(i, j int) bool { return a[i].totalAccess < a[j].totalAccess }

// LogStats is a structure that contains all stats about the site and all sections
type LogStats struct {
	siteName          string
	avg               *OverallTimeAverage
	rollingAvg        *RollingTimeAverage
	sectionStats      map[string]*SectionStats
	sortedSections    []*SectionStats
	totalSiteRequests int
	thresholdMin      float32
	highTrafficAlarm  bool
}

func (s *LogStats) PrintPopulartSections(num int) {
	pop := s.PopularSections()
	min := int(math.Min(float64(num), float64(len(pop))))
	for i := 0; i < min; i++ {
		pop[i].PrintStats()
	}
}

// NewLogStats creates a new LogStats object. siteName is the name of the site to monitor in the logs, avg is the
// averaging object to keep track of overall average statistics, rollingAvg is the object used to keep
// track of rolling averages, thresholdMin is the threshold (in terms of accesses per minte) for setting
// off the high traffic alarm.
func NewLogStats(siteName string, avg *OverallTimeAverage, rollingAvg *RollingTimeAverage, thresholdMin float32) *LogStats {
	s := new(LogStats)
	s.avg = avg
	s.rollingAvg = rollingAvg
	s.siteName = siteName
	s.thresholdMin = thresholdMin
	return s
}

// NewLogStatsDefault defaults the rolling average and the threshold. The rolling
// average keeps a history of 2 minutes by default, and the alarm is set whenever
// the rolling average exceeds 1 access per minute
func NewLogStatsDefault(siteName string) *LogStats {
	avg := NewOverallTimeAverage()
	rollingAverage := NewRollingTimeAverage(2)
	stats := NewLogStats(siteName, avg, rollingAverage, 1.0)
	return stats
}

// ProcessEntry is a function that processes a single log entry given an string
// representing that log entry
func (stats *LogStats) ProcessEntry(e *string) error {
	l, err := logparse.Common(*e)
	if err != nil {
		return err
	}

	if l.Request.URL.Hostname() != stats.siteName {
		log.Printf("site %s != %s\n", l.Request.URL.Hostname(), stats.siteName)
		return nil
	}

	if stats.sectionStats == nil {
		log.Println("Creating site stats map!")
		m := make(map[string]*SectionStats) //TODO: Initialize in constructor
		stats.sectionStats = m
	}

	log.Println("The site is: ", l.Request.URL.Hostname())
	log.Println("The entry is = ", *e)

	section := GetSectionFromURL(l.Request.URL.String())

	elem, ok := stats.sectionStats[section]
	if !ok {
		log.Println("Section ", section, " has never been accessed, adding it.")
		roll := NewRollingTimeAverage(2)
		tmp := NewSectionStats(section, roll)
		elem = tmp
		stats.sectionStats[section] = tmp
		stats.sortedSections = append(stats.sortedSections, tmp)

	}

	stats.rollingAvg.Update(l.Time.Unix())
	stats.avg.Update(l.Time.Unix())
	UpdateSectionStats(elem, l.Time.Unix())
	stats.totalSiteRequests++

	if (stats.rollingAvg.avgMin > stats.thresholdMin) && (stats.highTrafficAlarm == false) {
		fmt.Println("SITE RECEIVING HIGH TRAFFIC!!!!!")
		stats.highTrafficAlarm = true
	}

	if (stats.rollingAvg.avgMin <= stats.thresholdMin) && (stats.highTrafficAlarm == true) {
		fmt.Println("SITE TRAFFIC RETURNING TO NORMAL!!")
		stats.highTrafficAlarm = false
	}
	return nil
}

//TotalSiteRequests returns the total number of requests made to the site.
func (stats *LogStats) TotalSiteRequests() int {
	return stats.totalSiteRequests
}

// AccessesPerMinute returns the total number of accesses per minute
func (stats *LogStats) AccessesPerMinute(s string) (float32, error) {
	elem, ok := stats.sectionStats[s]
	if ok {
		return *elem.accessesPerMinute, nil
	}
	return -1.0, errors.New("Section " + s + " doesn't exist!")
}

//UniqueSiteVisits returns how many sections have been accessed in total
func (stats *LogStats) UniqueSiteVisits() int {
	return len(stats.sectionStats)
}

//PopularSections returns a sorted list of popular sections on the site.
func (stats *LogStats) PopularSections() []*SectionStats {
	sort.Sort(sort.Reverse(ByTotalAccesses(stats.sortedSections)))
	return stats.sortedSections
}

//UpdateSectionStats updates the statistics each time a new entry is encountered.
func UpdateSectionStats(stats *SectionStats, ts int64) {

	if stats.firstAccess == nil {
		stats.firstAccess = &ts
	} else {
		stats.lastAccess = &ts
		avg := float32(stats.totalAccess) * 60.0 / float32(*stats.lastAccess-*stats.firstAccess)
		stats.accessesPerMinute = &avg
	}
	stats.totalAccess++
	stats.rollingAverage.Update(ts)
}

var re = regexp.MustCompile(`(http:\/\/)?(.+)(\/.*)`)

// GetSectionFromURL returns a string representing the site
func GetSectionFromURL(url string) string {
	res := re.FindStringSubmatch(url)
	return res[2]
}

// LogReader is a struct that contains some basic information about the file
type LogReader struct {
	fileName string
	lastSize int64
}

// NewLogReader constructs a new log reader object
func NewLogReader(fName string) *LogReader {
	r := new(LogReader)
	r.fileName = fName
	// fmt.Println("The filename = ", r.fileName)
	return r
}

// GetNewLogEntries returns a a slice of new strings that have been appended to
// the file
func (l *LogReader) GetNewLogEntries() ([]string, error) {

	info, err1 := os.Lstat(l.fileName)

	if err1 != nil {
		return nil, err1
	}

	fSize := info.Size()

	if fSize <= l.lastSize {
		//log.Println("The were no changes to the file!")
		return nil, nil
	}

	f, err := os.Open(l.fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Go to the end of the last read
	f.Seek(l.lastSize, 0)
	var entries []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		entries = append(entries, scanner.Text())
	}
	l.lastSize = fSize
	return entries, nil
}
