package freepslib

import (
	"encoding/json"
	"encoding/xml"
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

	var data *AvmDeviceList
	err = xml.Unmarshal(byteValue, &data)
	assert.NilError(t, err)
	assert.Equal(t, data.Device[0].Name, "Steckdose")
}
