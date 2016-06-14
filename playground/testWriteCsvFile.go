/* This program tests the function of creating and writing a CSV file to disk
*/

package main

import (
	"encoding/csv"
	"os"
	"log"

)

func main() {

	csvfile, err := os.Create("output.csv")
	if err != nil {
		log.Fatalln("Error:", err)
		return
	}

	defer csvfile.Close()


	records := [][]string{
		{"first_name", "last_name", "username"},
		{"Rob", "Pike", "rob"},
		{"Ken", "Thompson", "ken"},
		{"Robert", "Griesemer", "gri"},
	}

	w := csv.NewWriter(csvfile)
	w.WriteAll(records) // calls Flush internally

	if err := w.Error(); err != nil {
		log.Fatalln("error writing csv:", err)
	}
}

