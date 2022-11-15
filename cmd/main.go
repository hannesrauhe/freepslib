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
	flag.StringVar(&freepsConfig.FB_address, "h", "fritz.box", "Address of your FritzBox")
	flag.StringVar(&freepsConfig.FB_pass, "p", "", "Password")
	flag.StringVar(&freepsConfig.FB_user, "u", "", "User")
	flag.BoolVar(&freepsConfig.Verbose, "v", false, "Verbose output")

	flag.Parse()
	fl, _ := freepslib.NewFreepsLib(&freepsConfig)
	dl, _ := fl.GetDeviceList()
	b, _ := json.MarshalIndent(*dl, "", "  ")
	fmt.Println(string(b))
	d, _ := fl.GetData()
	fmt.Println(*d)
}
