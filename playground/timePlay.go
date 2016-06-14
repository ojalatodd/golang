/* playing with time */

package main

import (
	"time"
	"fmt"

)

var dayslater time.Duration


func	main() {

	fmt.Println(time.Hour)
	fmt.Println(time.Now().Hour)

	now := time.Now()
	dayslater = 1

	threeDays := time.Hour * 24 * dayslater

	diff := now.Add(threeDays)

	fmt.Println(diff.Format(time.ANSIC))

	if diff.Unix() > now.Unix() {

	fmt.Println("This is in the future!")
	}

}

