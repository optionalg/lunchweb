package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"
)

var defaultCSVURL = "https://docs.google.com/spreadsheets/d/e/2PACX-1vTE16CfbUQiYoq6lrYJ27UENAYJWQ2lPtkE4eHUMMGKHnfdZ5d-BwR0gD1eom3IwPuEtVOgG73Y-QKR/pub?gid=0&single=true&output=csv"

var timeLayout = "2006-01-02"
var timeLocation = "Europe/Brussels"

var flagPort = flag.Int("port", 8081, "port to host on")
var flagCSVURL = flag.String("csvurl", defaultCSVURL, "public URL of the google sheets CSV")
var flagHeader = flag.Int("header", 3, "index of the header row with the column names")

func main() {
	flag.Parse()

	// headerIndex indicates the index of the row that contains column names
	headerIndex := *flagHeader

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rows, err := CSVFromGoogleSheetsURL(*flagCSVURL)
		if err != nil {
			http.Error(w, fmt.Sprintf("error: %v", err), http.StatusInternalServerError)
			return
		}
		header := rows[headerIndex]
		row, err := findRowForToday(rows)
		if err != nil {
			http.Error(w, fmt.Sprintf("error: %v", err), http.StatusInternalServerError)
			return
		}
		oo := NewOrderOverview(header[1:], row[1:])
		summary := oo.Summary()
		log.Println(summary)
		fmt.Fprintf(w, "%v", summary)
	})

	addr := fmt.Sprintf(":%d", *flagPort)
	log.Printf("Starting server (%s)", addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}

// CSVFromGoogleSheetsURL returns the contents of a CSV available via URL
func CSVFromGoogleSheetsURL(url string) ([][]string, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	r := csv.NewReader(bytes.NewReader(body))
	return r.ReadAll()
}

func findRowForToday(rows [][]string) ([]string, error) {
	now := time.Now()
	year, month, day := now.Date()
	location, err := time.LoadLocation(timeLocation)
	if err != nil {
		return nil, err
	}

	for _, row := range rows[(*flagHeader + 1):] {
		date, err := time.ParseInLocation(timeLayout, row[0], location)
		if err != nil {
			log.Println(err)
			continue
		}
		if date.Year() == year && date.Month() == month && date.Day() == day {
			return row, nil
		}
	}

	return nil, fmt.Errorf("no row found for today (%v)", now)
}

type OrderOverview struct {
	Names  []string
	Orders []string
}

func NewOrderOverview(names, orders []string) *OrderOverview {
	return &OrderOverview{
		Names:  names,
		Orders: orders,
	}
}

func (o *OrderOverview) Summary() string {
	var buffer bytes.Buffer

	fmt.Fprintf(&buffer, "Time: %v \n\n", time.Now().Format(time.RFC1123Z))

	didNotOrder := 0
	for i, col := range o.Names {
		if strings.TrimSpace(o.Orders[i]) != "" {
			// Example: Joe: BLT Sandwich
			buffer.WriteString(fmt.Sprintf("%v: %v\n", col, o.Orders[i]))
		} else {
			didNotOrder++
		}
	}

	total := len(o.Names)
	percentDidNot := float64(didNotOrder) / float64(total) * 100.0
	didOrder := total - didNotOrder
	percentDid := float64(didOrder) / float64(total) * 100.0
	buffer.WriteString("\n")
	buffer.WriteString(fmt.Sprintf("%v of %v did not order (~%.1f%%)\n", didNotOrder, total, percentDidNot))
	buffer.WriteString(fmt.Sprintf("%v of %v did order (~%.1f%%)\n", didOrder, total, percentDid))
	return buffer.String()
}
