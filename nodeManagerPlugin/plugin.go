// +build linux

/*
http://www.apache.org/licenses/LICENSE-2.0.txt


Copyright 2015 Intel Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nodeManagerPlugin

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/intelsdi-x/snap-plugin-collector-node-manager/ipmi"
	"github.com/intelsdi-x/snap/control/plugin"
	"github.com/intelsdi-x/snap/control/plugin/cpolicy"
	"github.com/intelsdi-x/snap/core/ctypes"
	"bufio"
)

const (
	//Name is name of plugin
	Name = "node-manager"
	//Version of plugin
	Version = 7
	//Type of plugin
	Type = plugin.CollectorPluginType
)

var namespacePrefix = []string{"intel", "node_manager"}

func makeName(metric string) []string {
	return append(namespacePrefix, strings.Split(metric, "/")...)
}

func parseName(namespace []string) string {
	return strings.Join(namespace[len(namespacePrefix):], "/")
}

func extendPath(path, ext string) string {
	if ext == "" {
		return path
	}
	return path + "/" + ext
}

// IpmiCollector Plugin class.
// IpmiLayer specifies interface to perform ipmi commands.
// NSim is number of requests allowed to be 'in processing' state.
// Vendor is list of request descriptions. Each of them specifies
// RAW request data, root path for metrics
// and format (which also specifies submetrics)
type IpmiCollector struct {
	IpmiLayer   ipmi.IpmiAL
	Vendor      map[string][]ipmi.RequestDescription
	Hosts       []string
	Mode        string
	Initialized bool
	NSim        int
}

// CollectMetrics Performs metric collection.
// Ipmi request are never duplicated in order to read multiple metrics.
// Timestamp is set to time when batch processing is complete.
// Source is hostname returned by operating system.
func (ic *IpmiCollector) CollectMetrics(mts []plugin.PluginMetricType) ([]plugin.PluginMetricType, error) {
	if !ic.Initialized {
		ic.construct(mts[0].Config().Table()) //reinitialize plugin
	}
	requestList := make(map[string][]ipmi.IpmiRequest, 0)
	requestDescList := make(map[string][]ipmi.RequestDescription, 0)
	responseCache := map[string]map[string]uint16{}
	hosts := make([]string, 0)
	requests := make([]string, 0)
	for _, mt := range mts {
		ns := parseName(mt.Namespace())
		if contains(hosts, mt.Namespace()[2]) == false {
			hosts = append(hosts, mt.Namespace()[2])
			requestDescList[mt.Namespace()[2]] = make([]ipmi.RequestDescription, 0)
			requestList[mt.Namespace()[2]] = make([]ipmi.IpmiRequest, 0)
		}
		for _, rq := range ic.Vendor[mt.Namespace()[2]] {
			if strings.Contains(ns, rq.MetricsRoot) && contains(requests, rq.MetricsRoot) == false {
				requests = append(requests, rq.MetricsRoot)
				requestList[mt.Namespace()[2]] = append(requestList[mt.Namespace()[2]], rq.Request)
				requestDescList[mt.Namespace()[2]] = append(requestDescList[mt.Namespace()[2]], rq)
			}
		}
//		requestDescList[host] = append(requestDescList[host], request)



	}
//
//	for _, host := range ic.Hosts {
//		requestList[host] = make([]ipmi.IpmiRequest, 0)
//		requestDescList[host] = make([]ipmi.RequestDescription, 0)
//		for _, request := range ic.Vendor[host] {
//			requestList[host] = append(requestList[host], request.Request)
//			requestDescList[host] = append(requestDescList[host], request)
//		}
//	}
	response := make(map[string][]ipmi.IpmiResponse, 0)

	for _, host := range hosts {
		response[host], _ = ic.IpmiLayer.BatchExecRaw(requestList[host], host)
	}

	for nmResponseIdx, hostResponses := range response {
		cached := map[string]uint16{}
		for i, resp := range hostResponses {
			format := requestDescList[nmResponseIdx][i].Format
			if err := format.Validate(resp); err != nil {
				resp.IsValid = 0
			}
			submetrics := format.Parse(resp)
			for k, v := range submetrics {
				path := extendPath(nmResponseIdx, requestDescList[nmResponseIdx][i].MetricsRoot)
				path = extendPath(path, k)
				cached[path] = v
			}
			responseCache[nmResponseIdx] = cached
		}
	}

//	results := make([]plugin.PluginMetricType, len(mts))
	var responseMetrics []plugin.PluginMetricType
	responseMetrics = make([]plugin.PluginMetricType, 0)
	t := time.Now()

//	for _, host := range ic.Hosts {
	for _, mt := range mts {
		ns := mt.Namespace()
		key := parseName(ns)
		data := responseCache[mt.Namespace()[2]][key]
		metric := plugin.PluginMetricType{Namespace_: ns, Source_: mt.Namespace()[2],
			Timestamp_: t, Data_: data}
//		results[i] = metric
		responseMetrics = append(responseMetrics, metric)
	}
//	}
	fmt.Println("MTS: ", len(mts))
	fmt.Println("RESPONSE: ", len(responseMetrics))

	return responseMetrics, nil
}

// GetMetricTypes Returns list of metrics available for current vendor.
func (ic *IpmiCollector) GetMetricTypes(cfg plugin.PluginConfigType) ([]plugin.PluginMetricType, error) {
	ic.construct(cfg.Table())
	var mts []plugin.PluginMetricType
	mts = make([]plugin.PluginMetricType, 0)
	if ic.IpmiLayer == nil {
		ic.Initialized = false
		return mts, fmt.Errorf("Wrong mode configuration")
	}
	for _, host := range ic.Hosts {
		for _, req := range ic.Vendor[host] {
			for _, metric := range req.Format.GetMetrics() {
				path := extendPath(host, req.MetricsRoot)
				path = extendPath(path, metric)
				mts = append(mts, plugin.PluginMetricType{Namespace_: makeName(path), Source_: host})
			}
		}
	}
	ic.Initialized = true
	return mts, nil
}

// GetConfigPolicy creates policy based on global config
func (ic *IpmiCollector) GetConfigPolicy() (*cpolicy.ConfigPolicy, error) {
	c := cpolicy.New()
	return c, nil
}

// New is simple collector constuctor
func New() *IpmiCollector {
	collector := &IpmiCollector{Initialized: false}
	return collector
}

func (ic *IpmiCollector) validateName(namespace []string) error {
	for i, e := range namespacePrefix {
		if namespace[i] != e {
			return fmt.Errorf("Wrong namespace prefix in namespace %v", namespace)
		}
	}
	return nil
}

func contains(slice []string, item string) bool {
	set := make(map[string]struct{}, len(slice))
	for _, s := range slice {
		set[s] = struct{}{}
	}
	_, ok := set[item]
	return ok
}

func getMode(config map[string]ctypes.ConfigValue) string {
	if mode, ok := config["mode"]; ok {
		return mode.(ctypes.ConfigValueStr).Value
	}
	return ""
}

func getChannel(config map[string]ctypes.ConfigValue) string {
	if channel, ok := config["channel"]; ok {
		return channel.(ctypes.ConfigValueStr).Value
	}
	return "0x00" //Default channel addr
}

func getSlave(config map[string]ctypes.ConfigValue) string {
	if slave, ok := config["slave"]; ok {
		return slave.(ctypes.ConfigValueStr).Value
	}
	return "0x00" //Default slave addr
}

func getPass(config map[string]ctypes.ConfigValue) string {
	if pass, ok := config["password"]; ok {
		return pass.(ctypes.ConfigValueStr).Value
	}
	return ""
}

func getUser(config map[string]ctypes.ConfigValue) string {
	if user, ok := config["user"]; ok {
		return user.(ctypes.ConfigValueStr).Value
	}
	return ""
}

func getHosts(config map[string]ctypes.ConfigValue) []string {
	if hosts, ok := config["hosts"]; ok {
		file, _ := os.Open(hosts.(ctypes.ConfigValueStr).Value)
		defer file.Close()
		var hostList []string
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			hostList = append(hostList, scanner.Text())
		}
		return hostList
	}
	return []string{""}
}

func (ic *IpmiCollector) construct(cfg map[string]ctypes.ConfigValue) {
	var hostList []string
	var ipmiLayer ipmi.IpmiAL
	ic.Mode = getMode(cfg)
	channel := getChannel(cfg)
	slave := getSlave(cfg)
	user := getUser(cfg)
	pass := getPass(cfg)
	host, _ := os.Hostname()
	if ic.Mode == "legacy_inband" {
		ipmiLayer = &ipmi.LinuxInBandIpmitool{Device: "ipmitool", Channel: channel, Slave: slave}
		hostList = []string{host}
	} else if ic.Mode == "oob" {
		ipmiLayer = &ipmi.LinuxOutOfBand{Device: "ipmitool", Channel: channel, Slave: slave, User: user, Pass: pass}
		hostList = getHosts(cfg)
	} else if ic.Mode == "legacy_inband_openipmi" {
		ipmiLayer = &ipmi.LinuxInband{}
	} else {
		return
	}

	ic.IpmiLayer = ipmiLayer
	ic.Hosts = hostList
	ic.Vendor = ipmiLayer.GetPlatformCapabilities(ipmi.GenericVendor, hostList)
	ic.Initialized = true

}

