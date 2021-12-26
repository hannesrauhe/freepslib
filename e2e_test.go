package freeps

import (
	"os"
	"testing"

	"gotest.tools/v3/assert"
)

func skipCI(t *testing.T) {
	if os.Getenv("E2E") != "1" {
		t.Skip("Skipping testing in CI environment")
	}
}

func TestData(t *testing.T) {
	skipCI(t)

	f, err := NewFreeps("./config_for_gotest_real.json")
	assert.NilError(t, err)

	mac := "40:8D:5C:5B:63:2D"
	uid, err := f.GetDeviceUID(mac)
	assert.NilError(t, err)
	assert.Equal(t, uid, "landevice3489")
}

func TestWakeUp(t *testing.T) {
	skipCI(t)

	f, err := NewFreeps("./config_for_gotest_real.json")
	assert.NilError(t, err)

	err = f.WakeUpDevice("landevice3489")
	assert.NilError(t, err)
}

func TestDeviceList(t *testing.T) {
	skipCI(t)

	f, err := NewFreeps("./config_for_gotest_real.json")
	assert.NilError(t, err)

	_, err = f.GetDeviceList()
	assert.NilError(t, err)
}

func TestSwitchLampOff(t *testing.T) {
	skipCI(t)

	f, err := NewFreeps("./config_for_gotest_real.json")
	assert.NilError(t, err)

	err = f.SetLevel("13077 0013108-1", 0)
	assert.NilError(t, err)
}

func TestSwitchLampOn(t *testing.T) {
	skipCI(t)

	f, err := NewFreeps("./config_for_gotest_real.json")
	assert.NilError(t, err)

	err = f.SetLevel("13077 0013108-1", 37)
	assert.NilError(t, err)
}
