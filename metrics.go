package freepslib

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/hannesrauhe/freepslib/fritzbox_upnp"
)

type FritzBoxMetrics struct {
	DeviceModelName      string
	DeviceFriendlyName   string
	Uptime               int64
	BytesReceived        int64 `json:"X_AVM_DE_TotalBytesReceived64,string"`
	BytesSent            int64 `json:"X_AVM_DE_TotalBytesSent64,string"`
	TransmissionRateUp   int64 `json:"ByteReceiveRate"`
	TransmissionRateDown int64 `json:"ByteSendRate"`
}

func (f *Freeps) initMetrics() error {
	if f.metricsObject != nil {
		return nil
	}

	var err error
	f.metricsObject, err = fritzbox_upnp.LoadServices("http://"+f.conf.Address+":49000", f.conf.User, f.conf.Password, false)
	if err != nil {
		return err
	}
	return nil
}

// helper function to deal with short service names
func (f *Freeps) getService(svcName string) (*fritzbox_upnp.Service, error) {
	err := f.initMetrics()
	if err != nil {
		return nil, err
	}
	svc, ok := f.metricsObject.Services[svcName]
	if ok {
		return svc, nil
	}

	for k, v := range f.metricsObject.Services {
		if svcName == f.getShortServiceName(k) {
			return v, nil
		}
	}

	if f.conf.Verbose {
		log.Printf("Available services:\n %v\n", f.metricsObject.Services)
	}
	return nil, errors.New("cannot find service " + svcName)
}

func (f *Freeps) getAction(serviceName string, actionName string) (*fritzbox_upnp.Action, error) {
	service, err := f.getService(serviceName)
	if err != nil {
		return nil, err
	}
	action, ok := service.Actions[actionName]
	if !ok {
		if f.conf.Verbose {
			log.Printf("Available actions:\n %v\n", service.Actions)
		}
		return nil, fmt.Errorf("cannot find action %s/%s ", serviceName, actionName)
	}
	return action, nil
}

func (f *Freeps) getMetricsMap(serviceName string, actionName string, arg *fritzbox_upnp.ActionArgument) (fritzbox_upnp.Result, error) {
	rmap := fritzbox_upnp.Result{}

	action, err := f.getAction(serviceName, actionName)
	if err != nil {
		return rmap, err
	}
	rmap, err = action.Call(arg)
	if err != nil {
		return rmap, fmt.Errorf("cannot call action %v: %w", actionName, err)
	}
	return rmap, nil
}

func (f *Freeps) GetMetrics() (FritzBoxMetrics, error) {
	var r FritzBoxMetrics
	m, err := f.getMetricsMap("urn:schemas-upnp-org:service:WANCommonInterfaceConfig:1", "GetAddonInfos", nil)
	if f.metricsObject != nil {
		r.DeviceModelName = f.metricsObject.Device.ModelName
		r.DeviceFriendlyName = f.metricsObject.Device.FriendlyName
	}
	if err != nil {
		return r, err
	}

	m2, err := f.getMetricsMap("urn:schemas-upnp-org:service:WANIPConnection:1", "GetStatusInfo", nil)
	if err != nil {
		return r, err
	}

	for k, v := range m2 {
		m[k] = v
	}

	byt, err := json.Marshal(m)
	if err != nil {
		return r, err
	}
	if f.conf.Verbose {
		log.Printf("Received metrics:\n %q\n", byt)
	}
	err = json.Unmarshal(byt, &r)
	return r, err
}

func (f *Freeps) GetUpnpDataMap(serviceName string, actionName string) (map[string]interface{}, error) {
	return f.getMetricsMap(serviceName, actionName, nil)
}

func (f *Freeps) CallUpnpActionWithArgument(serviceName string, actionName string, argName string, argValue interface{}) (map[string]interface{}, error) {
	return f.getMetricsMap(serviceName, actionName, &fritzbox_upnp.ActionArgument{Name: argName, Value: argValue})
}

func (f *Freeps) GetUpnpServices() ([]string, error) {
	err := f.initMetrics()
	if err != nil {
		return []string{}, err
	}
	keys := make([]string, 0, len(f.metricsObject.Services))
	for k := range f.metricsObject.Services {
		keys = append(keys, k)
	}

	return keys, nil
}

func (f *Freeps) GetUpnpServicesShort() ([]string, error) {
	err := f.initMetrics()
	if err != nil {
		return []string{}, err
	}
	keys := make([]string, 0, len(f.metricsObject.Services))
	for k := range f.metricsObject.Services {
		keys = append(keys, f.getShortServiceName(k))
	}

	return keys, nil
}

func (f *Freeps) GetUpnpServiceActions(serviceName string) ([]string, error) {
	service, err := f.getService(serviceName)
	if err != nil {
		return []string{}, err
	}

	keys := make([]string, 0, len(service.Actions))
	for k := range service.Actions {
		keys = append(keys, k)
	}

	return keys, nil
}

func (f *Freeps) GetUpnpServiceActionArguments(serviceName string, actionName string) ([]string, error) {
	action, err := f.getAction(serviceName, actionName)
	if err != nil {
		return []string{}, err
	}

	keys := make([]string, 0, len(action.Arguments))
	for _, k := range action.Arguments {
		if strings.ToLower(k.Direction) == "in" {
			keys = append(keys, k.Name)
		}
	}

	return keys, nil
}

func (f *Freeps) getShortServiceName(svcName string) string {
	shorts := strings.Split(svcName, ":")
	if len(shorts) < 2 {
		return "INVALID"
	}
	return shorts[len(shorts)-2]
}
