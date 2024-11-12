package main

import (
	"encoding/json"
	"flag"
	"fmt"

	"github.com/hannesrauhe/freepslib"
)

// just a stub for manual testing
func main() {
	var freepsConfig freepslib.FBconfig
	flag.StringVar(&freepsConfig.Address, "h", "fritz.box", "Address of your FritzBox")
	flag.StringVar(&freepsConfig.Password, "p", "", "Password")
	flag.StringVar(&freepsConfig.User, "u", "", "User")

	service := flag.String("s", "Hosts", "Service to call")
	action := flag.String("a", "X_AVM-DE_GetSpecificHostEntryByIP", "Action to call")
	argumentName := flag.String("k", "NewIPAddress", "Argument name")
	argumentValue := flag.String("v", "192.168.10.1", "Argument value")

	mode := flag.String("m", "call", "Mode: call, getsvc, getactions, getarguments, deviceList")

	flag.Parse()

	fl, err := freepslib.NewFreepsLib(&freepsConfig)
	if err != nil {
		fmt.Println(err)
		return
	}
	switch *mode {
	case "call":
		x, err := fl.CallUpnpActionWithArgument(*service, *action, *argumentName, *argumentValue)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(x)
	case "getsvc":
		x, err := fl.GetUpnpServices()
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(x)
	case "getactions":
		x, err := fl.GetUpnpServiceActions(*service)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(x)
	case "getarguments":
		x, err := fl.GetUpnpServiceActionArguments(*service, *action)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println(x)
	case "devicelist":
		x, err := fl.GetDeviceList()
		if err != nil {
			fmt.Println(err)
		}
		for _, d := range x.Device {
			json, _ := json.Marshal(d)
			fmt.Println(string(json))
		}
	}
}
