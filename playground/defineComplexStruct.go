// Testing how to define a complex struct type as simply as possible.


package main
import "fmt"

type MyData struct{
                /* Metadata struct {
                                Timestamp string `json:"timestamp"`
                        } */
                 Data []struct {
                        Email   string `json:"email"`
                        //Username string `json:"username"`
                        DeviceName              string `json:"deviceName`
                        Status                  string `json:"status"`
                        SelectedBytes           string `json:"selectedBytes"` // To be replaced with selected files
                        LastCompletedBackupDate string `json:"lastCompletedBackupDate"`
                        LastConnectedDate       string `json:"lastConnectedDate"`
                        BytesToDo               string          // From Computer resource
                        FilesToDo               string          // From Computer resource
                        AlertStates             string  `json:"alertStates"`
                        DestinationName         string  `json:destinationName`
                        OrgName                 string `json:"orgName"`
                        UserUid                 string `json:"userUid"`
                } 
                }

type OneRecord struct {
			Email   string `json:"email"`
                        //Username string `json:"username"`
                        DeviceName              string `json:"deviceName`
                        Status                  string `json:"status"`
                        SelectedBytes           string `json:"selectedBytes"` // To be replaced with selected files
                        LastCompletedBackupDate string `json:"lastCompletedBackupDate"`
                        LastConnectedDate       string `json:"lastConnectedDate"`
                        BytesToDo               string          // From Computer resource
                        FilesToDo               string          // From Computer resource
                        AlertStates             string  `json:"alertStates"`
                        DestinationName         string  `json:destinationName`
                        OrgName                 string `json:"orgName"`
                        UserUid                 string `json:"userUid"`
	}

func main() {
	var mydata = MyData{}
	var onerecord = OneRecord{}

	fmt.Print(onerecord)
	fmt.Print(mydata)
	mydata = append(mydata.Data, onerecord ) 
	fmt.Print(mydata)


}
