package main

import "strings"
import "fmt"

func main() {
	var mystring string = "Hello there Mom!"
	
	fields := strings.Fields(mystring)

	fmt.Println(fields)
	fmt.Println(len(fields))
	fmt.Println(fields[1])

	}


