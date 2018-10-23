package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
	"os"

	"google.golang.org/appengine"
	igc "github.com/marni/goigc"
)

// Some .igc files URLs for use in testing
// http://skypolaris.org/wp-content/uploads/IGS%20Files/Madrid%20to%20Jerez.igc
// http://skypolaris.org/wp-content/uploads/IGS%20Files/Jarez%20to%20Senegal.igc
// http://skypolaris.org/wp-content/uploads/IGS%20Files/Boavista%20Medellin.igc
// http://skypolaris.org/wp-content/uploads/IGS%20Files/Medellin%20Guatemala.igc

// URLTrack - Keep track of the url(s) used for adding the igc file
type URLTrack struct {
	trackName string
	track     igc.Track
}

// Keep count of the number of igc files added to the system
var igcFileCount = 1

// Map where the igcFiles are in-memory stored
var igcFiles = make(map[string]URLTrack) // map["URL"]urlTrack

// Unix timestamp when the service started
var timeStarted = int(time.Now().Unix())

// makes sure that the same track isn't added twice
func urlInMap(url string) bool {
	for urlInMap := range igcFiles {
		if urlInMap == url {
			return true
		}
	}
	return false
}

// Get the index of the track in the igcFiles slice, if it is there
func getTrackIndex(trackName string) string {
	for url, track := range igcFiles {
		if track.trackName == trackName {
			return url
		}
	}
	return ""
}

// ISO8601 duration parsing function
func parseTimeDifference(timeDifference int) string {

	result := "P" // Different time intervals are attached to this, if they are != 0
	// Formulas for calculating different time intervals in seconds
	timeLeft := timeDifference
	years := timeDifference / 31557600
	timeLeft -= years * 31557600
	months := timeLeft / 2629800
	timeLeft -= months * 2629800
	weeks := timeLeft / 604800
	timeLeft -= weeks * 604800
	days := timeLeft / 86400
	timeLeft -= days * 86400
	hours := timeLeft / 3600
	timeLeft -= hours * 3600
	minutes := timeLeft / 60
	timeLeft -= minutes * 60
	seconds := timeLeft

	// Add time invervals to the result only if they are different form 0
	if years != 0 {
		result += fmt.Sprintf("%dY", years)
	}
	if months != 0 {
		result += fmt.Sprintf("%dM", months)
	}
	if weeks != 0 {
		result += fmt.Sprintf("%dW", weeks)
	}
	if days != 0 {
		result += fmt.Sprintf("%dD", days)
	}
	if hours != 0 || minutes != 0 || seconds != 0 { // Check in case time intervals are 0
		result += "T"
		if hours != 0 {
			result += fmt.Sprintf("%dH", hours)
		}
		if minutes != 0 {
			result += fmt.Sprintf("%dM", minutes)
		}
		if seconds != 0 {
			result += fmt.Sprintf("%dS", seconds)
		}
	}
	return result
}

// Calculate the total distance of the track
func calculateTotalDistance(track igc.Track) string {
	totDistance := 0.0
	// For each point of the track, calculate the distance between 2 points in the Point array
	for i := 0; i < len(track.Points)-1; i++ {
		totDistance += track.Points[i].Distance(track.Points[i+1])
	}
	// Parse it to a string value
	return strconv.FormatFloat(totDistance, 'f', 2, 64)
}

// Check if any of the regex patterns supplied in the map parameter match the string parameter
func regexMatches(url string, urlMap map[string]func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	for mapURL := range urlMap {
		res, err := regexp.MatchString(mapURL, url)
		if err != nil {
			return nil
		}

		if res { // If the pattern matching returns true, return the function
			return urlMap[mapURL]
		}
	}
	return nil
}

func apiHandler(w http.ResponseWriter, r *http.Request) {	
	w.Header().Set("Content-Type", "application/json") // Set response content-type to JSON

	timeNow := int(time.Now().Unix()) // Unix timestamp when the handler was called

	duration := parseTimeDifference(timeNow - timeStarted) // Calculate the time elapsed by subtracting the times

	response := "{"
	response += "\"uptime\": \"" + duration + "\","
	response += "\"info\": \"Service for IGC tracks.\","
	response += "\"version\": \"v1\""
	response += "}"
	fmt.Fprintln(w, response)
}

func igcHandler(w http.ResponseWriter, r *http.Request) {

	if r.Method == "POST" { // If method is POST, user has entered the URL
		var data map[string]string // POST body is of content-type: JSON; the result can be stored in a map
		err := json.NewDecoder(r.Body).Decode(&data)
		if err != nil {
			panic(err)
		}

		track, err := igc.ParseLocation(data["url"]) // call the igc library
		if err != nil {
			panic(err)
		}

		// Check if track map contains the url
		// Or if the map is empty
		if !urlInMap(data["url"]) || len(igcFiles) == 0 {
			igcFiles[data["url"]] = URLTrack{"igc" + strconv.Itoa(igcFileCount), track} // Add the result to igcFiles map
			igcFileCount++                                                              // Increase the count
		}
		response := "{"
		response += "\"id\": " + "\"" + igcFiles[data["url"]].trackName + "\""
		response += "}"
		w.Header().Set("Content-Type", "application/json") // Set response content-type to JSON
		fmt.Fprintf(w, response)

	} else if r.Method == "GET" { // If the method is GET
		w.Header().Set("Content-Type", "application/json") // Set response content-type to JSON

		y := 0 // numeric iterator

		response := "["
		for j := range igcFiles { // Get all the IDs of .igc files stored in the igcFiles map
			if y != len(igcFiles)-1 { // If it's the last item in the array, don't add the ","
				response += "\"" + igcFiles[j].trackName + "\","
				y++ // Increment the iterator
			} else {
				response += "\"" + igcFiles[j].trackName + "\""
			}
		}
		response += "]"

		fmt.Fprintf(w, response)
	}
}

func idHandler(w http.ResponseWriter, r *http.Request) {
	urlID := path.Base(r.URL.Path) // returns the part after the last '/' in the url

	trackURL := getTrackIndex(urlID)
	if trackURL != "" { // Check whether the url is different from an empty string
		w.Header().Set("Content-Type", "application/json") // Set response content-type to JSON

		response := "{"
		response += "\"H_date\": " + "\"" + igcFiles[trackURL].track.Date.String() + "\","
		response += "\"pilot\": " + "\"" + igcFiles[trackURL].track.Pilot + "\","
		response += "\"glider\": " + "\"" + igcFiles[trackURL].track.GliderType + "\","
		response += "\"glider_id\": " + "\"" + igcFiles[trackURL].track.GliderID + "\","
		response += "\"track_length\": " + "\"" + calculateTotalDistance(igcFiles[trackURL].track) + "\"" 
		response += "}"
		fmt.Fprintf(w, response)
	} else {
		w.WriteHeader(http.StatusNotFound) // If it isn't, send a 404 Not Found status
	}
}

func fieldHandler(w http.ResponseWriter, r *http.Request) {

	pathArray := strings.Split(r.URL.Path, "/") // split the URL Path into parts, whenever there's a "/"
	field := pathArray[len(pathArray)-1]        // The part after the last "/", is the field
	uniqueID := pathArray[len(pathArray)-2]     // The part after the second to last "/", is the unique ID

	trackURL := getTrackIndex(uniqueID)

	if trackURL != "" { // Check whether the url is different from an empty string

		something := map[string]string{ // Map the field to one of the Track struct attributes in the igcFiles slice
			"pilot":        igcFiles[trackURL].track.Pilot,
			"glider":       igcFiles[trackURL].track.GliderType,
			"glider_id":    igcFiles[trackURL].track.GliderID,
			"track_length": calculateTotalDistance(igcFiles[trackURL].track),
			"H_date":       igcFiles[trackURL].track.Date.String(),
		}

		response := something[field] // This will work because the RegEx checks whether the name is written correctly
		fmt.Fprintf(w, response)
	} else {
		w.WriteHeader(http.StatusNotFound) // sends error if field is not written correctly
	}
}

func urlRouter(w http.ResponseWriter, r *http.Request) {
	urlMap := map[string]func(http.ResponseWriter, *http.Request){ // A map of accepted URL RegEx patterns
		"^/igcinfo/api$":                      apiHandler,
		"^/igcinfo/api/igc$":                   igcHandler,
		"^/igcinfo/api/igc/[a-zA-Z0-9]{3,10}$": idHandler,
		"^/igcinfo/api/igc/[a-zA-Z0-9]{3,10}/(pilot|glider|glider_id|track_length|H_date)$": fieldHandler,
	}

	result := regexMatches(r.URL.Path, urlMap) // Perform the RegEx check to see if any pattern matches

	if result != nil { // If a function is returned, call that handler function
		result(w, r)
	} else {
		w.WriteHeader(http.StatusNotFound) // If no function is returned, send a 404 Not Found status
	}
}

func main() {
	http.HandleFunc("/", urlRouter)
	appengine.Main()	
	log.Fatal(http.ListenAndServe(":" + os.Getenv("PORT"), nil))
}
