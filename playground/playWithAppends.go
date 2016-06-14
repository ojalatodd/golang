package main

import (
	"fmt"
	)
	

type  TestStruct	struct {
	Data	[]struct {
		First	string
		Last 	string
	}


}

type Record	struct {
	First	string
	Last	string
	}


func main() {
	var testStruct TestStruct
	var record = Record{ "Todd", "Ojala" }
	testStruct.Data = append(testStruct.Data, record)


	
	fmt.Println(testStruct)


}

