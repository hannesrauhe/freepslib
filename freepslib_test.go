package freepslib

import (
	"encoding/json"
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

var testConfig = FBconfig{
	Address:  "fritz.box",
	User:     "freeps",
	Password: "freeps",
}

func TestChallenge(t *testing.T) {
	c := FBconfig{Address: "a", User: "u", Password: "p", Verbose: true}
	f, err := NewFreepsLib(&c)
	assert.NilError(t, err)
	expectedURL := "https://a/login_sid.lua?username=u&response=a51eacbd-05f2dd791db47141584e0f220b12c7e1"

	assert.Equal(t, f.calculateChallengeURL("a51eacbd"), expectedURL)
}

func TestGetUID(t *testing.T) {
	t.SkipNow()
	byteValue, err := os.ReadFile("./_testdata/test_data.json")
	assert.NilError(t, err)

	mac := "40:8D:5C:5B:63:2D"
	var data *AvmDataResponse
	err = json.Unmarshal(byteValue, &data)
	assert.NilError(t, err)
	assert.Equal(t, getDeviceUID(*data, mac), "landevice3489")
}

func TestDeviceListUnmarshal(t *testing.T) {
	byteValue, err := os.ReadFile("./_testdata/test_devicelist.xml")
	assert.NilError(t, err)

	dl, err := parseDeviceList(byteValue)
	assert.NilError(t, err)
	assert.Equal(t, len(dl.Device), 3)
	assert.Equal(t, dl.Device[0].Name, "Steckdose")

	// test button-backward compatibility
	assert.Assert(t, dl.Device[2].Button != nil)
	assert.Equal(t, len(dl.Device[2].ButtonFunctions), 2)
}

func TestDeviceListUnmarshal2(t *testing.T) {
	xmlBytes, err := os.ReadFile("./_testdata/large_devicelist.xml")
	assert.NilError(t, err)

	jsonFile, err := os.Open("./_testdata/large_devicelist.json")
	assert.NilError(t, err)
	dec := json.NewDecoder(jsonFile)
	assert.Assert(t, dec != nil)

	dlFromXML, err := parseDeviceList(xmlBytes)
	var dlExpected AvmDeviceList
	err = dec.Decode(&dlExpected)
	assert.NilError(t, err)
	assert.Assert(t, dlFromXML != nil)
	assert.DeepEqual(t, *dlFromXML, dlExpected)

	// newJsonFile, err := os.Create("./_testdata/large_devicelist.json")
	// json.NewEncoder(newJsonFile).Encode(dlFromXML)
	// assert.NilError(t, err)
}
