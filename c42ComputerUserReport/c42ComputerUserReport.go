/*  c42ComputerUserReport
File: c42ComputerUserReport

Copyright (c) 2016 Code42 Software, Inc.
Permission is hereby granted, free of charge, to any person obtaining a copy 
of this software and associated documentation files (the "Software"), to deal 
in the Software without restriction, including without limitation the rights 
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell 
copies of the Software, and to permit persons to whom the Software is 
furnished to do so, subject to the following conditions:
The above copyright notice and this permission notice shall be included in all 
copies or substantial portions of the Software.
THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR 
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, 
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE 
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER 
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, 
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE 
SOFTWARE.

Author: Todd Ojala
Last modified 05-25-2016
	1. MIT License added to top comments section 
	2. API version info added 
05-05-2016
	1. Added column: BackupCompletePercentage
	2. Command line option to show only active devices

05-17-2016
	Customer request to sort results by last-connected date.
05-16-2016
	1. Added -help option

04-27-2016
	Tweaks by customer request:
	1. Change label for Device field to "Device Status"
	2. Create new log file each time program runs, instead of appending
	3. Do not record each call to the Computer resource in the log file
	4. Report the total number of objects queried in the log file before exiting

04-22/2016
	Added paging, because the DeviceBackupReport resource returns 1000 records max.
	Paging still not added to Users query, because no limit is enforeed.

The purpose of this script is to generate a customized report for Oliver Wyman, then write the results to a CSV
file on disk, named output.csv .

Usage:

Command to run
	c42ComputerUserReport [-active] [-limit <number>]
	Example command: c42ComputerUserReport -active -limit 100  (This example shows only active devices and limits
	calls to the Computer API to 100)

	Required file: userinfo.conf
		This file stores the master server to query, and the username/password combo to use for authentication.

Command Line Arguments
	Required command line arguments: none.

	The optional command-line argument "-limit" limits the number of calls to the Computer resource used to generate the report.
	This can be done for testing purposes, to reduce the program run time, or in production, if it takes too long to run the
	program.

	The optional command-line argument "-active" filters out all deactivated devices from the report. Only active devices appear.

Format of userinfo.config:
	A file with one entry per line:
		master server url, e.g.: https://master.example.com:4285
		username
		password

API version compatability note: this software should work with Code42 server versions 4.3 to 5.3. Changes to the API in newer versions 
	may cause the application to cease working. See API specification and release notes for more information.
	The API Docviewer for the latest version can be vewied at: https://www.crashplan.com/apidocviewer/

*/

package main

import (
	"bufio"
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
	DeviceBackupReport = "/api/DeviceBackupReport?pgSize=1000" // Page size of 1000 is the default and current max as of 5.1.2.
	Computer           = "/api/Computer"
	User               = "/api/User?pgSize=99999"

	helpText = "Command line parameters: \n [-active] [-limit <number> ] [-nousers] [-help]\n" +
		"USAGE: \nThe -active option filters out deactivated devices from the report.\n" +
		"The -limit option limits the number of calls made to the Computer resource of the Code42 API. \n" +
		"These API calls to Computer are needed to fill in some fields of the report, but can be time-consuming. \n" +
		"For initial testing, it may be useful to limit these calls. \n" +
		"The -nousers option tells the program to skip the process of appending users who do not have registered devices to the report. \n" +
		"Note: when the -active option is specified, the list of users without devices will also include users with deactivated devices. \n" +
		"If the -active option is not specified, the list of users at the end of the report includes only users who have never had an active device."
)

type Records [][]string // The datatype that holds the results just before conversion to CSV

var (
	/* URL of the master server, with port */
	url      string
	resource string // Data-type for the resource to call within the API: either DeviceBackup report, User, or Computer

	/* Credentials for master server below.  */
	username        string
	password        string
	testLimitNumber int    // Stores the limit to the number of calls to the computer resource.
	activeFilter    string // Stores the query that filters out deactivated devices if desired
)

/* Complex datastructures defined below */
type ReportData struct {
	Data ReportDataArray
}

type ReportDataArray []ReportDataRecord

type ReportDataRecord struct {
	Email string `json:"email"`
	//Username string `json:"username"`
	DeviceName               string `json:"deviceName`
	Status                   string `json:"status"`
	SelectedFiles            string // From Computer resource
	LastBackupDate           string // From Computer resource
	LastCompletedBackupDate  string `json:"lastCompletedBackupDate"`
	LastConnectedDate        string `json:"lastConnectedDate"`
	BytesToDo                string // From Computer resource
	FilesToDo                string // From Computer resource
	BackupCompletePercentage string `json:"backupCompletePercentage"`
	AlertStates              string `json:"alertStates"`
	DestinationName          string `json:destinationName`
	OrgName                  string `json:"orgName"`
	UserUid                  string `json:"userUid"`   // Not in report. Used to join data.
	DeviceUid                string `json:"deviceUid"` // Not in report. Used to find data from the Computer API resource
}

type UsersData struct {
	Data struct {
		TotalCount int `json:"totalCount"`
		Users      []struct {
			UserUid string `json:"userUid"`
			Email   string `json:"email"`
		}
	} `json:"data"`
}

type ComputerData struct {
	Data struct {
		Guid        string `json:"guid"`
		BackupUsage []struct {
			SelectedFiles int    `json:"selectedFiles"`
			LastBackup    string `json:"lastBackup"`
			TodoBytes     int    `json:"todoBytes"`
			TodoFiles     int    `json:"todoFiles"`
		}
	}
}

func main() {
	timeStamp := makeTimestamp()

	f, err := os.OpenFile("c42ComputerUserReportLogFile"+timeStamp, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("error opening file: %v", err)
	}
	defer f.Close()

	log.SetOutput(f)
	log.Println("Start at", time.Now().String())

	activeOnlyArg := flag.Bool("active", false, "If set, shows only active devices. Default is false.")
	testLimitNumberArg := flag.Int("limit", -1, "Limits the calls to the computer API to this number.")
	noUsers := flag.Bool("nousers", false, "Do not append users without active or inactive devices.")
	showHelp := flag.Bool("help", false, "Show help.")

	flag.Parse()

	if *showHelp {
		fmt.Println(helpText)
		log.Println("Showing help and exiting.")
		os.Exit(0)
	}

	testLimitNumber = *testLimitNumberArg

	/* Create the string that is added to the query to DeviceBackupReport API that filters out deactivated devices */

	if *activeOnlyArg {
		activeFilter = "&active=true"
	}

	/* Read the user authentication info from file userinfo.config */
	lines, err := readLines("userinfo.config")

	if err != nil {
		log.Fatalln("readLines: %s", err)
	}
	if len(lines) < 3 {
		log.Fatalln("Info is missing from the config file.")
	}
	url = strings.Trim(lines[0], " ") // Trimming extra spaces at beginning and end of lines
	username = strings.Trim(lines[1], " ")
	password = strings.Trim(lines[2], " ")

	/* Retrieve the DeviceBackupReport data, the first part of the report
	Need to loop and get data page by page until no data is left. Page limit of 1000 hard-coded into API */
	deviceReportMsg := ReportData{}
	endDeviceData := false // A flag to signal that the last page of data from DeviceBackup has been reached
	for page := 1; endDeviceData != true; page++ {
		deviceReportMsgPage := ReportData{} // Store each page in this variable

		resourcePlusQuery := DeviceBackupReport + "&pgNum=" + strconv.Itoa(page) + activeFilter
		contents := makeRequest(url, resourcePlusQuery) // Call the function that makes http requests and returns a response in of type []byte

		/* Deserialize the JSON data into a struct */
		errJson := json.Unmarshal(contents, &deviceReportMsgPage)
		if errJson != nil {
			log.Fatalln("Error unmarshalling JSON from device report api:", errJson)
		}

		if len(deviceReportMsgPage.Data) == 0 {
			endDeviceData = true
		} else {
			for _, pageStruxData := range deviceReportMsgPage.Data {
				deviceReportMsg.Data = append(deviceReportMsg.Data, pageStruxData)
			}
		}
	}

	/* Convert the deviceReportMsg data to an array */
	var reportDataArray ReportDataArray

	reportDataArray = deviceReportMsg.Data
	reportDataRecord := ReportDataRecord{}

	/* Get missing fields from Computer  */
	computerMsg := ComputerData{}

	for j, strux := range deviceReportMsg.Data {
		if testLimitNumber != -1 && j >= testLimitNumber {
			break
		}

		/* Get missing info from Computer resource here.
		Use deviceUid as the key */
		deviceUid := strux.DeviceUid
		// log.Println("Retrieving info from Computer resource from device with guid:",deviceUid)
		query := Computer + "/" + deviceUid + "?idType=guid&incAll=true"

		contents := makeRequest(url, query)
		errJson := json.Unmarshal(contents, &computerMsg)
		if errJson != nil {
			log.Fatalln("Error unmarshalling JSON from the Computer API resource:", errJson)
		}
		if len(computerMsg.Data.BackupUsage) != 0 {
			reportDataArray[j].SelectedFiles = strconv.Itoa(computerMsg.Data.BackupUsage[0].SelectedFiles)
			reportDataArray[j].LastBackupDate = computerMsg.Data.BackupUsage[0].LastBackup
			reportDataArray[j].BytesToDo = strconv.Itoa(computerMsg.Data.BackupUsage[0].TodoBytes)
			reportDataArray[j].FilesToDo = strconv.Itoa(computerMsg.Data.BackupUsage[0].TodoFiles)
		}
	}

	totalDeviceObjects := len(reportDataArray) // Store total number of devices found for log file

	/* Any users who have no registered device need to be found and appended to the report */
	if !*noUsers {
		userMsg := UsersData{}
		contents := makeRequest(url, User) // Call makeRequest again to get the data from the User resource of the API
		/* Deserialize the JSON data into a struct */
		errJson := json.Unmarshal(contents, &userMsg)
		if errJson != nil {
			log.Fatalln("Error unmarshalling JSON from the User API resource:", errJson)
		}

		for _, strux := range userMsg.Data.Users {

			userGuid := strux.UserUid
			flag := false
			for _, struxInner := range deviceReportMsg.Data {
				if userGuid == struxInner.UserUid {
					flag = true
					break
				}
			}
			if !flag && strux.Email != "" {
				reportDataArray = append(reportDataArray, reportDataRecord)
				reportDataArray[len(reportDataArray)-1].Email = strux.Email
			}

		}
	}

	deviceReportMsg.Data = reportDataArray

	finalRecords := convertStructToRecords(deviceReportMsg) // store the results in finalRecords variable which is an array of string arrays
	/* Create and write the CSV file  */
	csvfile, csv_err := os.Create("output.csv")
	if csv_err != nil {
		log.Fatalln("Error creating CSV file:", csv_err)
	}

	defer csvfile.Close()
	w := csv.NewWriter(csvfile)
	w.WriteAll(finalRecords) // calls Flush internally

	if write_err := w.Error(); write_err != nil {
		log.Fatalln("error writing csv:", write_err)
	}
	log.Println("Total number of device objects:", totalDeviceObjects)
	log.Println("Report generated. Exiting")
}

func convertStructToRecords(data ReportData) Records {
	/* This function converts data that is in the form defined in the datatype ReportData into an array of string arrays
	as defined in the data type Records */

	converted := make(Records, len(data.Data)+1)

	/* Add headers to columns */
	headers := [13]string{"Email", "DeviceName", "DeviceStatus", "SelectedFiles", "LastBackup", "LastCompletedBackup", "LastConnected", "BytesToDo", "FilesToDo", "BackupCompletePercentage", "Alerts", "Destination", "OrgName"}
	for _, header := range headers {
		converted[0] = append(converted[0], header)
	}

	for index, strux := range data.Data {

		index++ // Increment index because of header added above

		converted[index] = append(converted[index], strux.Email)
		converted[index] = append(converted[index], strux.DeviceName)
		converted[index] = append(converted[index], strux.Status)
		converted[index] = append(converted[index], strux.SelectedFiles)
		converted[index] = append(converted[index], strux.LastBackupDate)
		converted[index] = append(converted[index], strux.LastCompletedBackupDate)
		converted[index] = append(converted[index], strux.LastConnectedDate)
		converted[index] = append(converted[index], strux.BytesToDo)
		converted[index] = append(converted[index], strux.FilesToDo)
		converted[index] = append(converted[index], strux.BackupCompletePercentage)
		converted[index] = append(converted[index], strux.AlertStates)
		converted[index] = append(converted[index], strux.DestinationName)
		converted[index] = append(converted[index], strux.OrgName)

	}

	return converted

}

func makeRequest(url, resource string) []byte {
	// Build the transport method for the request. Skip ssl errors
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
		log.Fatalf("Error making request: %s", err)
	}

	/* Get the results */
	defer resp.Body.Close()
	contents, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("%s", err)
	}

	return contents

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

/* This function returns a string timestamp for loggin */
func makeTimestamp() string {
	timenow := time.Now().String()
	return timenow[0:10]
}

