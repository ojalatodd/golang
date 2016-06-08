/* The purpose of this program is to set the purge date of archives in cold storage in Code42 CrashPlan servers to a new date based on
two parameters: a baseline date and the number of days, N, after the baseline date to set the purge date to.

The program accepts the following command line parameters:
1. [-b date ] Baseline date . Date format: either TODAY or MM-DD-YYYY
2. [-d N ] Number of days in future after baseline date for new purge date.
3. [-t ] test : whether or not to really change the purge dates, or just output the number of archives
	that would be changed. Default is 'false'.
4. [-a ] all : sets date for all archives in cold storage to the date, not just those that have a purge date
	later than N days later than baseline). Default is 'false'
5. [-s] Skip destinations that report having zero cold storage bytes. Default is 'false'.
6. [-help] Show help.

Example command:
> ./setColdStoragePurgeDate -b 05-12-2016 -d 30 -t -s
 The example command above would use May 5, 2016 as the baseline date. The new archive expiration date would be 30 days after May 5, 2016.
 The program would run in test mode only (and not actually change the archive expiration dates). Finally, the program would
 skips any destination in the environment that reports have zero bytes in cold storage.

Required file: hostinfo.conf
		This file stores the master server to query, and the username/password combo to use for authentication.

Format of hostinfo.config:
	A file with one entry per line:
		master server url, e.g.: https://master.example.com:4285
		username
		password

Output:
	1. log file: one date-stamped log file per calendar day. Multiple runs on the same day append to this file.
	2. Date-stamped CSV file with list of archives with changed purge date. Fields: Archive GUID, Old Purge Date, New Purge Date
		When run in test mode, the CSV file has the prefix "test_"
	3. Console output similar to what is in the log file

Misc. Notes:
	When the baseline date is given as a date (format = MM-DD-YYYY), the time zone associated with the resultant date
	object is UTC or GMT. When the date is specified with TODAY, the associated date object is the time zone of the
	host machine. This produces an unintended effect: if you run the program more than once with the same baseline date
	(not specified as TODAY), the program will keep rewriting the new purge date with PUT calls to ColdStorage API, because
	it thinks the new purge date is GMT, whereas the purge date stored in the server is the time zone of the server.
	Since users will not need to run more than once with a particular new archive expiration date, I don't think it's
	worth spending time to fix at this time.

Version 1.0
Created 4-27-2016
Author: Todd Ojala

Modified 5-13-2016
	Added help option.
	Doesn't display cold bytes info for destinations of -s option selected
	Logs and prings number of archives with null of malformed expiration dates

Modified 5-12-2016
	Changes to account for the fact that PROVIDER destinations provide string values for api/ColdStorage GET queries, in the
	ColdBytes field, whereas CLUSTER destinations provide integer values. Stragen and inconsistent. Need to unmarshal the
	ColdBytes data into an interface datatype, then convert based on provider type.
	Also, I discovered that destinations with archives in cold storage sometimes have zero bytes reported in cold storage,
	so I provided a way to skip filtering destinations for amount of data in cold storage, and made skipping this check the
	default. Seems to all work.

	Changes suggested by Jon Carlon's code review. Removed quotes surrounding guid in CSV output file, to make it easier
	to find and sort by GUID.

Modified 5-11-2016
	Test runs now produce a CSV file with the prefix test_

Modified 5-10-2016
	It now successfully changes archives, and logging is pretty good.
	Adding CSV output result file now.

Modified 5-9-2016
	Much progress.

Modified 5-6-2016
	Added many things. Can now query server to get info on destinations.

Copyright (c)2016, Todd Ojala

Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby granted,
provided that the above copyright notice and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE
INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE
OR ANY SPECIAL, DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM
LOSS OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION,
ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.

*/

package main

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	API_DESTINATION         = "/api/Destination"
	API_COLDSTORAGE         = "/api/ColdStorage"
	shortFormDate           = "01-02-2006"
	code42ArchiveTimeFormat = "2006-01-02T15:04:05.000-07:00"

	helpText = "Command line parameters: \n [-b date] [-d days] [-t ] [-a ] [-s ] [-help]\n" +
		"\n Semantics:\n-b specifies the baseline date; -d specifies how many days later the purge date should be;\n" +
		"-t tells program to run in test  mode (default is false);\n-a tells program to change all archive expiration dates, not just " +
		"archives that have an exp date greater than b+d (default is false);\n-s tells program to skip destinations that report have zero bytes in cold storage (default is false);\n" +
		"-help displays this help message.\n"
)

type Records [][]string // The datatype that holds the results just before conversion to CSV

var (
	baseLineDate time.Time
	daysLater    int
	testOnly     bool
	setAll       bool

	url      string
	username string
	password string

	destinations        []int
	archivesToChange    []string
	originalPurgeDates  []string
	destinationsChanged []int

	newPurgeDate time.Time
)

func main() {
	timeStamp := makeTimestamp() // For log file

	/* Open a log file. One log file created per day. Appends to day's log file if it already exists */
	f, err := os.OpenFile("setColdStoragePurgeDateLog_"+timeStamp, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		fmt.Println("Can't open log file. Quitting.")
		os.Exit(1)
	}
	defer f.Close()

	log.SetOutput(f)
	log.Println("Start at ", time.Now().String())

	/* Define the command line flags and default options */
	baseLineDateArg := flag.String("b", "TODAY", "Baseline date for calculating purge date: MM-DD-YYYY or TODAY")
	daysLaterArg := flag.Int("d", 0, "Number of days after baseline to set purge data to: integer")
	testOnlyArg := flag.Bool("t", false, "Test only")
	setAllArg := flag.Bool("a", false, "Set all archives in cold storage to the new date, instead of only archives with purge date > b+d.")
	skipDestWithZeroCB := flag.Bool("s", false, "Skips destinations that have zero bytes in cold storage as reported by the API. Default is false.")
	showHelp := flag.Bool("help", false, "Show help.")

	flag.Parse()

	if *showHelp {
		fmt.Println(helpText)
		log.Println("Showing help and exiting.")
		os.Exit(0)
	}

	log.Println("Command line arguments:")
	log.Println("-b:", *baseLineDateArg)
	log.Println("-d:", *daysLaterArg)
	log.Println("-t:", *testOnlyArg)
	log.Println("-a:", *setAllArg)
	log.Println("-s", *skipDestWithZeroCB)

	/*Convert baseline date parameter to a real date datatype/object */
	if *baseLineDateArg == "TODAY" {
		baseLineDate = time.Now()
	} else {
		baseLineDateTmp, err := time.Parse(shortFormDate, *baseLineDateArg)
		if err != nil {
			fmt.Println("Date argument not formatted correctly:", err)
			log.Fatalf("Date argument not formatted correctly: %v", err)
		} else {
			baseLineDate = baseLineDateTmp
		}
	}

	/* Process the "days later" parameter */
	if *daysLaterArg >= 0 {
		daysLater = *daysLaterArg
	} else {
		fmt.Println("The days later parameter is not valid. Must be greater than or equal to zero.")
		log.Println("The days later parameter is not valid. Must be greater than or equal to zero. \n")
		log.Fatalln("Quitting.")
	}
	// fmt.Println("Days later paremeter =", daysLater)

	/* Since we are here, calculate the new purge date */
	daysLaterHours := time.Hour * 24 * time.Duration(daysLater)
	newPurgeDate := baseLineDate.Add(daysLaterHours)
	fmt.Println("New purge date=", newPurgeDate.Format(time.ANSIC))
	log.Println("New purge date=", newPurgeDate.Format(time.ANSIC)) // Log the new purge date

	testOnly = *testOnlyArg
	setAll = *setAllArg

	/* Read the host and user authentication info from the hostinfo.config file */
	lines, err := readLines("hostinfo.config")

	if err != nil {
		log.Fatalf("Can't read hostinfo.config file. Error: %s", err)
	}
	if len(lines) < 3 {
		log.Fatalf("Info is missing from the config file. Check hostinfo.config file. Quitting. \n")
	}

	url = strings.Trim(lines[0], " ") // Trimming extra spaces at beginning and end of lines
	username = strings.Trim(lines[1], " ")
	password = strings.Trim(lines[2], " ")

	// println(url, username, password )
	log.Println("Connecting to host:", url)

	/* Get a list of all the archives in cold storage in this Code42 environment */
	destinationRespMsg := struct {
		Data struct {
			Destinations []struct {
				DestinationId   int         `json:"destinationId"`
				Guid            string      `json:"guid"`
				DestinationName string      `json:"destinationName"`
				Type            string      `json:"type"`
				ColdBytes       interface{} `json:"coldBytes"`
			} `json:"destinations"`
		} `json:"data"`
	}{}

	contents := makeRequest(url, API_DESTINATION)
	/* Deserialize the JSON data into the right struct */
	errJson := json.Unmarshal(contents, &destinationRespMsg)
	if errJson != nil {
		log.Println("Error unmarshalling JSON from Destination API:", errJson)
		log.Fatalln("Quitting.")
	}

	/* Filter out destinations that do not have cold storage bytes and place remaining in a list */
	var coldBytesConverted int
	for _, dest := range destinationRespMsg.Data.Destinations {

		if *skipDestWithZeroCB {
			/* The data returned by the Destinations API, coldBytes field is of type String for Provider, but int for Cluster.
			Until this is made consistent, need to deal with both types. 5-12-2016 */

			switch dest.Type {
			case "PROVIDER":
				coldBytesString, _ := dest.ColdBytes.(string)
				if coldBytesTmp, err := strconv.Atoi(coldBytesString); err != nil {
					coldBytesConverted = 0
				} else {
					coldBytesConverted = coldBytesTmp
				}
			default:
				coldBytesTmp, _ := dest.ColdBytes.(float64) // You can only parse numbers of type float64 into interfaces from JSON
				coldBytesConverted = int(coldBytesTmp)      // Now convert the float64 to an int
			}

			fmt.Printf("Destination %v cold bytes= %v \n", dest.DestinationId, coldBytesConverted)
			log.Printf("Destination %v cold bytes= %v \n", dest.DestinationId, coldBytesConverted)
		} else {
			coldBytesConverted = 1 // Not skipping any destinations, even if Cold Bytes is zero or null
		}

		if coldBytesConverted > 0 { // replaced 'coldBytesTmp > 0' with true since I took out the coldbytes check
			destinations = append(destinations, dest.DestinationId)
		}
	}

	// fmt.Println("The list of destinations:", destinations)
	fmt.Printf("%d destinations have archives in cold storage.\n", len(destinations))
	log.Printf("%d destinations have archives in cold storage.\n", len(destinations))

	/* Define a struct to receive contents of ColdStorage API GET calls */
	coldStorageRespMsg := struct {
		Data struct {
			ColdStorageRows []struct {
				ArchiveGuid           string `json:"archiveGuid"`
				ArchiveBytes          int    `json:"archiveBytes"` // Might use this to total purge bytes for log/reporting
				ArchiveHoldExpireDate string `json:"archiveHoldExpireDate"`
			} `json:"coldStorageRows"`
		} `json:"data"`
	}{}

	/* Retrieve list of all cold storage archives that meet date criteria  */
	endData := false                    // Flag for paging loop
	nullArchiveHoldExpireDateCount := 0 // Keep track of the odd phenomomen of archives with null expire dates
	for _, destId := range destinations {
		endData = false // Reset flag to false again for each destination
		fmt.Println("Retrieving list of cold storage archives from destination Id:", destId)
		log.Println("Retrieving list of cold storage archives from destination Id:", destId)
		/* Need to page through the data. Can't get it all at once! */
		for page := 1; endData != true; page++ {

			resourcePlusQuery := API_COLDSTORAGE + "?destinationId=" + strconv.Itoa(destId) + "&pgNum=" + strconv.Itoa(page) //+ "&pgSize=2" // Change pagesize to 100+ in production
			contents := makeRequest(url, resourcePlusQuery)

			/* Deserialize the JSON data into a struct */
			errJson := json.Unmarshal(contents, &coldStorageRespMsg)
			if errJson != nil {
				log.Println("Error unmarshalling JSON from coldStorage API GET call:", errJson)
				log.Fatalln("Quitting.")
			}
			if len(coldStorageRespMsg.Data.ColdStorageRows) == 0 {
				endData = true // No more data. Set flag to true.
			} else {
				for _, coldStorageRow := range coldStorageRespMsg.Data.ColdStorageRows {
					// fmt.Println("Cold Storage found. GUID:", coldStorageRow.ArchiveGuid)

					/* Does this archive meet the criteria? */
					if setAll { // The -a flag was set. Change all archives' purge date
						archivesToChange = append(archivesToChange, coldStorageRow.ArchiveGuid)
						originalPurgeDates = append(originalPurgeDates, coldStorageRow.ArchiveHoldExpireDate)
						destinationsChanged = append(destinationsChanged, destId)

					} else {
						// fmt.Println("Purge date:", coldStorageRow.ArchiveHoldExpireDate)
						archivePurgeDateTmp, err := time.Parse(code42ArchiveTimeFormat, coldStorageRow.ArchiveHoldExpireDate)
						if err != nil {
							log.Printf("Date argument not formatted correctly for archive %v. Error: %v. Skipping.", coldStorageRow.ArchiveGuid, err)
							nullArchiveHoldExpireDateCount++

						} else {
							/* Compare the archive's purge date with the new desired one. If it is bigger, add it to the list to change */

							// fmt.Println("Diff between archive purge date and new purge date:", archivePurgeDateTmp.Sub(newPurgeDate))
							if (archivePurgeDateTmp.Unix() - newPurgeDate.Unix()) > 0 { // In this case, the desired purge date is before the current
								archivesToChange = append(archivesToChange, coldStorageRow.ArchiveGuid)
								originalPurgeDates = append(originalPurgeDates, coldStorageRow.ArchiveHoldExpireDate)
								destinationsChanged = append(destinationsChanged, destId)

							}
						}
					}
				}
			}

		}

	}
	if !setAll {
		fmt.Printf("%d archives had a null or malformed expiration date. See log for archive GUIDs.\n", nullArchiveHoldExpireDateCount)
		log.Printf("%d archives had a null or malformed expiration date. See log for archive GUIDs.\n", nullArchiveHoldExpireDateCount)
	}
	// fmt.Println("list of archives to change:", archivesToChange)

	/* Create header for CSV output file */
	changeResults := make(Records, 1)
	headers := [4]string{"Archive GUID", "Old Purge Date", "New Purge Date", "DestinationId"}
	for _, columnName := range headers {
		changeResults[0] = append(changeResults[0], columnName)
	}

	csvFilePrefix := "" // Prefix will say test_ if it was only a test run

	totalCount := 0 // Keep track of total number of cold storage purge date changes made
	if !testOnly {
		fmt.Println("Starting to change achive expiration dates.")
		log.Println("Starting to change achive expiration dates.")
		/* Use the Cold Storage API with PUT to change the purge date */
		for i, archiveGuid := range archivesToChange {
			fmt.Print(".")
			success := changePurgeDate(archiveGuid, newPurgeDate)
			if success == false {
				log.Println("Could not change purge date for archive with GUID=", archiveGuid)
			} else {
				totalCount++
				data := []string{archivesToChange[i], originalPurgeDates[i], newPurgeDate.Format(code42ArchiveTimeFormat), strconv.Itoa(destinationsChanged[i])}
				changeResults = append(changeResults, data)
			}
		}

	} else {
		for i, _ := range archivesToChange {
			data := []string{archivesToChange[i], originalPurgeDates[i], newPurgeDate.Format(code42ArchiveTimeFormat), strconv.Itoa(destinationsChanged[i])}
			changeResults = append(changeResults, data)
		}
		fmt.Printf("This was only a test. %d archives in cold storage would have had their purge dates changed.\n", len(archivesToChange))
		log.Printf("This was only a test. %d archives in cold storage would have had their purge dates changed.\n", len(archivesToChange))
		csvFilePrefix = "test_"
	}
	fmt.Print("\n") // Separate dot progress indicator from next message
	fmt.Println("Total number of purge dates changed:", totalCount)
	log.Println("Total number of purge dates changed:", totalCount)

	/* Write CSV file and exit */
	csvfile, csv_err := os.Create(csvFilePrefix + "results_" + strings.Replace(time.Now().Format(time.Stamp), " ", "_", -1) + ".csv")
	if csv_err != nil {
		log.Fatalln("Error creating CSV file:", csv_err)
	}

	defer csvfile.Close()
	w := csv.NewWriter(csvfile)
	w.WriteAll(changeResults) // calls Flush internally

	if write_err := w.Error(); write_err != nil {
		log.Fatalln("Error writing csv:", write_err)
	}

	fmt.Println("Done.")
	log.Println("Done.")

}

/* Functions used in this program are defined below */

/* This function returns a string timestamp for loggin */
func makeTimestamp() string {
	timenow := time.Now().String()
	return timenow[0:10]
}

func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func makeRequest(url, resource string) []byte {
	/* This function performs a GET request on the desired resource and returns a response */

	/* Build the transport method for the request. Skip ssl errors */
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr} // Create an http client

	/* Build a request to the DeviceBackupReport resource */
	fullPath := url + resource

	req, err := http.NewRequest("GET", fullPath, nil)
	req.SetBasicAuth(username, password)
	resp, err := client.Do(req) // Perform the GET request
	if err != nil {
		log.Fatalln("Error making request in makeRequest function. Error:", err)
	}

	/* Get the results */
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("Error reading response in makeRequest function: %s \n", err)
	}

	return contents

}

func changePurgeDate(guid string, date time.Time) bool {
	/* Changes the archiveHoldExpireDate (aka purge date) of archives in cold storage. This function depends
	on some global variables and constants: url, API_COLDSTORAGE. Ideally, these would be passed as parameters. (future change?) */

	/* Convert the new purge date time into a string of the right format for the ColdStorage API */
	stringDate := date.Format("2006-01-02")

	var jsonStr = []byte(`{ "archiveHoldExpireDate" : "` + stringDate + `" }`)

	/* Build the transport method for the request. Skip ssl errors */
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	client := &http.Client{Transport: tr} // Create an http client
	/* Build a request to the DeviceBackupReport resource */
	fullPath := url + API_COLDSTORAGE + "/" + guid + "?idType=guid"
	req, err := http.NewRequest("PUT", fullPath, bytes.NewBuffer(jsonStr))
	req.SetBasicAuth(username, password)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error making request while changing purge date for archive with GUID %v: %s \n", guid, err) // Continue with other archives though
		return false
	}
	defer resp.Body.Close()

	/* Below is for debugging only */
	// fmt.Println("response Status:", resp.Status)
	// fmt.Println("response Headers:", resp.Header)
	// body, _ := ioutil.ReadAll(resp.Body)
	// fmt.Println("response Body:", string(body))
	return true

}
