package freepslib

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
	"unicode/utf16"

	"github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freepslib/fritzbox_upnp"
)

type FBconfig struct {
	Address    string
	User       string
	Password   string
	Verbose    bool
	FB_address string `json:",omitempty"` // deprecated, use Address instead
	FB_user    string `json:",omitempty"` // deprecated, use User instead
	FB_pass    string `json:",omitempty"` // deprecated, use Password instead
}

var DefaultConfig = FBconfig{Address: "fritz.box", User: "freeps", Password: "password"}

type Freeps struct {
	conf          FBconfig
	logger        logrus.FieldLogger
	SID           string
	metricsObject *fritzbox_upnp.Root
}

func NewFreepsLib(conf *FBconfig) (*Freeps, error) {
	logger := logrus.New()
	if conf.Verbose {
		logger.SetLevel(logrus.DebugLevel)
	}
	return NewFreepsLibWithLogger(conf, logger.WithField("module", "freepslib"))
}

func NewFreepsLibWithLogger(conf *FBconfig, logger logrus.FieldLogger) (*Freeps, error) {
	if conf == nil {
		return nil, errors.New("No config provided")
	}
	if logger == nil {
		return nil, errors.New("No logger provided")
	}
	if conf.FB_address != "" {
		logger.Warning("FB_address is deprecated, use Address instead")
		if conf.Address == "" {
			conf.Address = conf.FB_address
			conf.FB_address = ""
		} else {
			logger.Errorf("FB_address and Address both set, using Address: %v", conf.Address)
		}
	}
	if conf.FB_user != "" {
		logger.Warning("FB_user is deprecated, use User instead")
		if conf.User == "" {
			conf.User = conf.FB_user
			conf.FB_user = ""
		} else {
			logger.Errorf("FB_user and User both set, using User: %v", conf.User)
		}
	}
	if conf.FB_pass != "" {
		logger.Warning("FB_pass is deprecated, use Password instead")
		if conf.Password == "" {
			conf.Password = conf.FB_pass
			conf.FB_pass = ""
		} else {
			logger.Errorf("FB_pass and Password both set, using Password: %v", conf.Password)
		}
	}
	f := &Freeps{conf: *conf, logger: logger}
	return f, nil
}

func (f *Freeps) login() error {
	var err error

	f.logger.Debugf("Trying to log into fritzbox")

	f.SID, err = f.getSid()
	if err != nil {
		f.logger.Errorf("Failed to authenticate")
		return err
	}
	return nil
}

func (f *Freeps) getHttpClient() *http.Client {
	tr := &http.Transport{}
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &http.Client{Transport: tr, Timeout: time.Second * 10}
}

/****** AUTH *****/

type AvmSessionInfo struct {
	SID       string
	Challenge string
}

func (f *Freeps) calculateChallengeURL(challenge string) string {
	login_url := "https://" + f.conf.Address + "/login_sid.lua"

	// python: hashlib.md5('{}-{}'.format(challenge, password).encode('utf-16-le')).hexdigest()
	u := utf16.Encode([]rune(challenge + "-" + f.conf.Password))
	b := make([]byte, 2*len(u))
	for index, value := range u {
		binary.LittleEndian.PutUint16(b[index*2:], value)
	}
	h := md5.New()
	h.Write(b)
	chal_repsonse := hex.EncodeToString(h.Sum(nil))

	return fmt.Sprintf("%v?username=%v&response=%v-%v", login_url, f.conf.User, challenge, chal_repsonse)
}

func (f *Freeps) getSid() (string, error) {
	login_url := "https://" + f.conf.Address + "/login_sid.lua"
	client := f.getHttpClient()
	// get Challenge:
	first_resp, err := client.Get(login_url)
	if err != nil {
		return "", err
	}
	defer first_resp.Body.Close()

	var unauth AvmSessionInfo
	byt, err := io.ReadAll(first_resp.Body)
	if err != nil {
		return "", err
	}
	xml.Unmarshal(byt, &unauth)

	// respond to Challenge and get SID
	second_resp, err := client.Get(f.calculateChallengeURL(unauth.Challenge))
	if err != nil {
		return "", err
	}
	defer second_resp.Body.Close()

	byt, err = io.ReadAll(second_resp.Body)
	if err != nil {
		return "", err
	}
	var authenticated AvmSessionInfo
	err = xml.Unmarshal(byt, &authenticated)
	if err != nil {
		return "", err
	}
	if authenticated.SID == "0000000000000000" {
		return "", errors.New("Authentication failed: wrong user/password")
	}

	return authenticated.SID, nil
}

/****** WebInterface functions *****/

type AvmDeviceInfo struct {
	Mac  string
	UID  string
	Name string
	Type string
}

type AvmDataObject struct {
	Active   []*AvmDeviceInfo
	Passive  []*AvmDeviceInfo
	btn_wake string
}

type AvmDataResponse struct {
	Data *AvmDataObject
}

func (f *Freeps) queryData(payload map[string]string, AvmResponse interface{}) error {
	dataURL := "https://" + f.conf.Address + "/data.lua"

	// blindly try twice, because the first one might be an auth issue
	for i := 0; i < 2; i++ {
		data := url.Values{}
		data.Set("sid", f.SID)
		for key, value := range payload {
			data.Set(key, value)
		}

		dataResp, err := f.getHttpClient().PostForm(dataURL, data)
		if err != nil {
			return errors.New("cannot PostForm: " + err.Error())
		}
		defer dataResp.Body.Close()

		byt, err := io.ReadAll(dataResp.Body)
		if err != nil {
			return errors.New("cannot read response")
		}
		if dataResp.StatusCode != 200 {
			f.logger.Debugf("Unexpected http status: %v, Body:\n %v", dataResp.Status, byt)
			return errors.New("http status code != 200")
		}

		f.logger.Debugf("Received data:\n %q\n", byt)

		err = json.Unmarshal(byt, &AvmResponse)
		if err == nil {
			return nil
		}
		if i > 0 {
			return errors.New("cannot parse JSON response: " + err.Error())
		}

		err = f.login()
		if err != nil {
			return err
		}
	}

	return nil
}

func (f *Freeps) GetData() (*AvmDataResponse, error) {
	var avmResp *AvmDataResponse
	var err error
	payload := map[string]string{
		"page":  "netDev",
		"xhrId": "all",
	}

	err = f.queryData(payload, &avmResp)
	return avmResp, err
}

func getDeviceUID(fb_response AvmDataResponse, mac string) string {
	for _, dev := range append(fb_response.Data.Active, fb_response.Data.Passive...) {
		if dev.Mac == mac {
			return dev.UID
		}
	}
	return ""
}

func (f *Freeps) GetDeviceUID(mac string) (string, error) {
	d, err := f.GetData()

	if err != nil {
		return "", err
	}
	return getDeviceUID(*d, mac), nil
}

func (f *Freeps) WakeUpDevice(uid string) error {
	var avmResp *AvmDataResponse
	payload := map[string]string{
		"dev":      uid,
		"oldpage":  "net/edit_device.lua",
		"page":     "edit_device",
		"btn_wake": "",
	}

	err := f.queryData(payload, &avmResp)
	if err != nil {
		return err
	}
	if avmResp.Data.btn_wake != "ok" {
		f.logger.Debugf("%v", avmResp)
		return errors.New("device wakeup seems to have failed")
	}
	return nil
}

/**** HOME AUTOMATION *****/

type AvmDeviceSwitch struct {
	State      bool   `xml:"state"`
	Lock       bool   `xml:"lock"`
	Devicelock bool   `xml:"devicelock"`
	Mode       string `xml:"mode"`
}

type AvmDevicePowermeter struct {
	Power   int `xml:"power"`
	Energy  int `xml:"energy"`
	Voltage int `xml:"voltage"`
}

type AvmDeviceTemperature struct {
	Celsius int `xml:"celsius"`
	Offset  int `xml:"offset"`
}

type AvmDeviceSimpleonoff struct {
	State bool `xml:"state"`
}

type AvmDeviceLevelcontrol struct {
	Level           float32 `xml:"level"`
	LevelPercentage float32 `xml:"levelpercentage"`
}

type AvmDeviceColorcontrol struct {
	Hue         int `xml:"hue"`
	Saturation  int `xml:"saturation"`
	Temperature int `xml:"temperature"`
}

type AvmNextChange struct {
	Endperiod int `xml:"endperiod"`
	TChange   int `xml:"tchange"`
}

type AvmDeviceHkr struct {
	Tist                    int            `xml:"tist"`
	Tsoll                   int            `xml:"tsoll"`
	Komfort                 int            `xml:"komfort"`
	Absenk                  int            `xml:"absenk"`
	Batterylow              bool           `xml:"batterylow"`
	Battery                 int            `xml:"battery"`
	Windowopenactive        bool           `xml:"windowopenactiv"` // cannot ignore the typo here
	Windowopenactiveendtime int            `xml:"windowopenactiveendtime"`
	Boostactive             bool           `xml:"boostactive"`
	Boostactiveendtime      int            `xml:"boostactiveendtime"`
	Holidayactive           bool           `xml:"holidayactive"`
	Summeractive            bool           `xml:"summeractive"`
	Lock                    bool           `xml:"lock"`
	Devicelock              bool           `xml:"devicelock"`
	NextChange              *AvmNextChange `xml:"nextchange"`
}

type AvmDeviceAlert struct {
	State                 int `xml:"state"`
	LastAlertChgTimestamp int `xml:"lastalertchgtimestamp"`
}

type AvmButton struct {
	Name                 *string `xml:"name"`
	LastPressedTimestamp int     `xml:"lastpressedtimestamp"`
}

type AvmEtsiUnitInfo struct {
	DeviceID   string `xml:"etsideviceid"`
	UnitType   int    `xml:"unittype"`
	Interfaces string `xml:"interfaces"`
}

type AvmDevice struct {
	Name         string                 `xml:"name" json:",omitempty"`
	AIN          string                 `xml:"identifier,attr"`
	DeviceID     string                 `xml:"id,attr"`
	ProductName  string                 `xml:"productname,attr" json:",omitempty"`
	Present      bool                   `xml:"present" json:",omitempty"`
	Switch       *AvmDeviceSwitch       `xml:"switch" json:",omitempty"`
	Temperature  *AvmDeviceTemperature  `xml:"temperature" json:",omitempty"`
	Powermeter   *AvmDevicePowermeter   `xml:"powermeter" json:",omitempty"`
	SimpleOnOff  *AvmDeviceSimpleonoff  `xml:"simpleonoff" json:",omitempty"`
	LevelControl *AvmDeviceLevelcontrol `xml:"levelcontrol" json:",omitempty"`
	ColorControl *AvmDeviceColorcontrol `xml:"colorcontrol" json:",omitempty"`
	HKR          *AvmDeviceHkr          `xml:"hkr" json:",omitempty"`
	Alert        *AvmDeviceAlert        `xml:"alert" json:",omitempty"`
	Button       *AvmButton             `xml:"button" json:",omitempty"`
	EtsiUnitInfo *AvmEtsiUnitInfo       `xml:"etsiunitinfo" json:",omitempty"`
}

type AvmDeviceList struct {
	Device []AvmDevice `xml:"device"`
}

type AvmTemplate struct {
	Name       string         `xml:"name"`
	Identifier string         `xml:"identifier,attr"`
	ID         string         `xml:"id,attr"`
	Devices    *AvmDeviceList `xml:"devices"`
}

type AvmTemplateList struct {
	Template []AvmTemplate `xml:"template"`
}

func (f *Freeps) queryHomeAutomation(switchcmd string, ain string, payload map[string]string) ([]byte, error) {
	mTime := time.Now()

	baseUrl := "https://" + f.conf.Address + "/webservices/homeautoswitch.lua"
	var dataURL string
	var dataResp *http.Response
	var byt []byte
	var err error
	retry := true
	for {
		if len(ain) == 0 {
			dataURL = fmt.Sprintf("%v?sid=%v&switchcmd=%v", baseUrl, f.SID, switchcmd)
		} else {
			dataURL = fmt.Sprintf("%v?sid=%v&switchcmd=%v&ain=%v", baseUrl, f.SID, switchcmd, ain)
		}
		for key, value := range payload {
			dataURL += "&" + key + "=" + value
		}

		dataResp, err = f.getHttpClient().Get(dataURL)
		if err != nil {
			return nil, fmt.Errorf("Request to '%s' failed: %v", dataURL, err)
		}
		defer dataResp.Body.Close()

		byt, err = io.ReadAll(dataResp.Body)
		if err != nil {
			return nil, errors.New("cannot read response")
		}
		if dataResp.StatusCode == 403 && retry {
			retry = false
			err = f.login()
			if err != nil {
				return nil, errors.New("failed to login: " + err.Error())
			}
			continue
		}
		break
	}

	if dataResp.StatusCode != 200 {
		f.logger.Debugf("Unexpected http status: %v, Body:\n %q", dataResp.Status, byt)
		return nil, errors.New("http status code != 200")
	}

	time1 := time.Now().Unix() - mTime.Unix()

	f.logger.Debugf("Request took %vs.\nReceived data:\n %q\n", time1, byt)
	return bytes.Trim(byt, "\n"), nil
}

func (f *Freeps) GetDeviceList() (*AvmDeviceList, error) {
	byt, err := f.queryHomeAutomation("getdevicelistinfos",
		"", make(map[string]string))
	if err != nil {
		return nil, err
	}

	var avm_resp *AvmDeviceList
	err = xml.Unmarshal(byt, &avm_resp)
	if err != nil {
		f.logger.Debugf("Cannot parse XML: %q, err: %v", byt, err)
		return nil, errors.New("cannot parse XML response")
	}

	return avm_resp, nil
}

func (f *Freeps) GetTemplateList() (*AvmTemplateList, error) {
	byt, err := f.queryHomeAutomation("gettemplatelistinfos",
		"", make(map[string]string))
	if err != nil {
		return nil, err
	}

	var avm_resp *AvmTemplateList
	err = xml.Unmarshal(byt, &avm_resp)
	if err != nil {
		f.logger.Debugf("Cannot parse XML: %q, err: %v", byt, err)
		return nil, errors.New("cannot parse XML response")
	}

	return avm_resp, nil
}

func (f *Freeps) HomeAutoSwitch(switchcmd string, ain string, payload map[string]string) error {
	_, err := f.queryHomeAutomation(switchcmd, ain, payload)
	return err
}

func (f *Freeps) HomeAutomation(switchcmd string, ain string, payload map[string]string) ([]byte, error) {
	return f.queryHomeAutomation(switchcmd, ain, payload)
}

func (f *Freeps) SetLevel(ain string, level int) error {
	payload := map[string]string{
		"level": fmt.Sprint(level),
	}
	_, err := f.queryHomeAutomation("setlevel", ain, payload)
	return err
}

// GetSuggestedSwitchCmds returns all known switch commands and their expected parameters
func (f *Freeps) GetSuggestedSwitchCmds() map[string][]string {
	return switchCmds
}

var switchCmds map[string][]string = map[string][]string{
	"getswitchlist":        {"device"},
	"setswitchon":          {"device"},
	"setswitchoff":         {"device"},
	"setswitchtoggle":      {"device"},
	"getswitchstate":       {"device"},
	"getswitchpresent":     {"device"},
	"getswitchpower":       {"device"},
	"getswitchenergy":      {"device"},
	"getswitchname":        {"device"},
	"getdevicelistinfos":   {""},
	"gettemperature":       {"device"},
	"gethkrtsoll":          {"device"},
	"gethkrkomfort":        {"device"},
	"gethkrabsenk":         {"device"},
	"sethkrtsoll":          {"device", "param"},
	"getbasicdevicestats":  {"device"},
	"gettemplatelistinfos": {""},
	"applytemplate":        {"template"},
	"setsimpleonoff":       {"device", "onoff"},
	"setlevel":             {"device", "level"},
	"setlevelpercentage":   {"device", "level"},
	"setcolor":             {"device", "hue", "saturation", "duration"},
	"setcolortemperature":  {"device", "temperature", "duration"},
	"getcolordefaults":     {"device"},
	"sethkrboost":          {"device", "endtimestamp"},
	"sethkrwindowopen":     {"device"},
	"setblind":             {"device", "target"},
	"setname":              {"device", "name"},
	"startulesubscription": {"device"},
	"getsubscriptionstate": {"device"},
	"getdeviceinfos":       {"device"},
}
