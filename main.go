package main

import (
	"bytes"
	"encoding/csv"
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"
)

var defaultCSVURL = "https://docs.google.com/spreadsheets/d/e/2PACX-1vTE16CfbUQiYoq6lrYJ27UENAYJWQ2lPtkE4eHUMMGKHnfdZ5d-BwR0gD1eom3IwPuEtVOgG73Y-QKR/pub?gid=0&single=true&output=csv"

var timeLayout = "2006-01-02"
var timeLocation *time.Location

var flagPort = flag.Int("port", 8081, "port to host on")
var flagCSVURL = flag.String("csvurl", defaultCSVURL, "public URL of the google sheets CSV")
var flagHeader = flag.Int("header", 3, "index of the header row with the column names")
var flagTimezone = flag.String("tz", "Europe/Brussels", "timezone to use")
var flagSubject = flag.String("subject", "Order", "the email subject")
var flagEmail = flag.String("email", "test@example.org", "which email to send to")
var flagSheetURL = flag.String("sheet-url", "https://example.com", "spreadsheet url")

const indexTemplate = `
<html>
	<head>
		<title>LunchWeb</title>
		<style>
			* {
				font-family: monospace;
				margin: 0;
				padding: 0;
				line-height: 1.4;
			}
			body {
				padding: 10px;
			}
			a { 
				color: #0af; 
				font-weight: bold;
				text-decoration: none;
			}
			a:hover { text-decoration: underline; }
			li { margin-left: 20px; }
		</style>
	</head>
	<body>
		<h2>LunchWeb</h2>
		<p><a href="{{.SheetURL}}">Fill in your order</a></li>
		or <a href="mailto:{{.Email}}?subject={{.EmailSubject}}%20({{.Today}})&body={{.Order.Summary}}">send an email</a> with all orders.
		</p>
		<br>
		<p>Orders as of {{.Now}}:</p>
		<br>
		{{with .Order}}
			{{range .LineItems}}
			<p>{{.Name}}: {{.Order}}</p>
			{{end}}
			<br>
			<p>{{len .LineItems}} out of {{.MaxCount}} ordered something ({{.OrderPercent | printf "~%.2f%%"}})</p>
		{{end}}

	</body>
</html>
`

func main() {
	flag.Parse()

	// setup template
	t, err := template.New("home").Parse(indexTemplate)
	if err != nil {
		log.Fatal(err)
	}

	// setup time zone
	timeLocation, err = time.LoadLocation(*flagTimezone)
	if err != nil {
		log.Fatal(err)
	}

	// headerIndex indicates the index of the row that contains column names
	headerIndex := *flagHeader

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		rows, err := CSVFromGoogleSheetsURL(*flagCSVURL)
		if err != nil {
			http.Error(w, fmt.Sprintf("error from csv: %v", err), http.StatusInternalServerError)
			return
		}
		header := rows[headerIndex]
		row, err := findRowForToday(rows)
		if err != nil {
			http.Error(w, fmt.Sprintf("error for today's row: %v", err), http.StatusInternalServerError)
			return
		}
		oo := NewOrderOverview(header[1:], row[1:])
		summary := oo.Summary()
		log.Println(summary)

		data := map[string]interface{}{
			"Now":          now().Format(time.RFC1123Z),
			"Today":        now().Format("2006-01-02"),
			"EmailSubject": *flagSubject,
			"Email":        *flagEmail,
			"SheetURL":     *flagSheetURL,
			"Order":        oo,
		}
		if err := t.Execute(w, data); err != nil {
			http.Error(w, fmt.Sprintf("error in template: %v", err), http.StatusInternalServerError)
			return
		}
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

func now() time.Time {
	return time.Now().In(timeLocation)
}

func findRowForToday(rows [][]string) ([]string, error) {
	now := now()
	year, month, day := now.Date()

	for _, row := range rows[(*flagHeader + 1):] {
		date, err := time.ParseInLocation(timeLayout, row[0], timeLocation)
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

type LineItem struct {
	Name  string
	Order string
}

func NewOrderOverview(names, orders []string) *OrderOverview {
	return &OrderOverview{
		Names:  names,
		Orders: orders,
	}
}

func (o *OrderOverview) LineItems() []*LineItem {
	lines := make([]*LineItem, 0)
	for i, name := range o.Names {
		order := strings.TrimSpace(o.Orders[i])
		if name != "" && order != "" {
			lines = append(lines, &LineItem{name, order})
		}
	}
	sort.Sort(ByName(lines))
	return lines
}

func (o *OrderOverview) MaxCount() int {
	return len(o.Names)
}

func (o *OrderOverview) OrderPercent() float32 {
	return 100 * float32(len(o.LineItems())) / float32(o.MaxCount())
}

func (o *OrderOverview) Summary() string {
	var buffer bytes.Buffer

	for _, li := range o.LineItems() {
		// Example: Joe: BLT Sandwich
		buffer.WriteString(fmt.Sprintf("%v: %v\n", li.Name, li.Order))
	}

	return buffer.String()
}

type ByName []*LineItem

func (a ByName) Len() int           { return len(a) }
func (a ByName) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByName) Less(i, j int) bool { return a[i].Name < a[j].Name }
