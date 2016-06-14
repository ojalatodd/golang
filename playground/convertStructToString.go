/* Exercise in converting a struct array to an array of strings 
for use in converting the struct to CSV */

package main

import (
	"fmt"
	"encoding/csv"
)



type MyData struct {
	Data []struct {
                Username string `json:"username"`
                DeviceName string `json:"deviceName`
                OrgName string `json:"orgName"`
            } `json:"data"`
        }
var d = MyData{ [{"Todd","PC1","Default"}]}
		
func main() {
	
	fmt.Println(d)

}
