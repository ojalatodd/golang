/* Exercise in converting a struct array to an array of strings 
for use in converting the struct to CSV.
It expanded to show the full path from taking a struct coming from JSON, converting it to an array of string arrays, to writing it to a CSV
file on disk. */

package main

import (
	"fmt"
	"encoding/csv"
	"os"
	"log"
)



type MyData struct {
	Data	Data
        }

type Data []Record

type Record struct {
		 Username string //`json:"username"`
                 DeviceName string //`json:"deviceName`
                 OrgName string //`json:"orgName"`
             } //`json:"data"`

type Records [][]string

		
func main() {

	record1 := Record{"Todd","Pc","default"}
	record2 := Record{"John", "Mac", "default"}

	data := Data{record1, record2}

	myData  := MyData{data}

	
//	fmt.Println(data)
	fmt.Println(myData.Data)

	fmt.Println(convertStructToRecords(myData.Data))

	writeCsvFile(convertStructToRecords(myData.Data))  // Send the data converted from array of structs to array of string arrays to file

}

func convertStructToRecords(data Data) Records {
	/* testRecords := [][]string{
	                {"first_name", "last_name", "username"},
		        {"Rob", "Pike", "rob"},
	                {"Ken", "Thompson", "ken"},
	                {"Robert", "Griesemer", "gri"},
	        } */
	
	converted := make(Records, len(data))

	for index, strux := range data {
			//fmt.Printf("Index=%d, data=%v\n", index, strux) //This line for testing contents of data passed in

			converted[index] = append(converted[index], strux.Username)
			converted[index] = append(converted[index], strux.DeviceName)
			converted[index] = append(converted[index], strux.OrgName)
		}
	
	return  converted

	}

func writeCsvFile(records Records) {
	csvfile, err := os.Create("output.csv")
        if err != nil {
                log.Fatalln("Error:", err)
                return
        }
	defer csvfile.Close()

	w := csv.NewWriter(csvfile)
        w.WriteAll(records) // calls Flush internally

	if err := w.Error(); err != nil {
                log.Fatalln("error writing csv:", err)
        }	

}
