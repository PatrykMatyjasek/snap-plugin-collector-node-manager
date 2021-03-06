// +build linux

/*
http://www.apache.org/licenses/LICENSE-2.0.txt


Copyright 2016 Intel Corporation

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

package ipmi

import (
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
)

// LinuxOutOfBand implements communication with openipmi driver on linux
type LinuxOutOfBand struct {
	Device  string
	Channel string
	Slave   string
	Addr    []string
	User    string
	Pass    string
	mutex   sync.Mutex
}

func (al *LinuxOutOfBand) RunParallelRequests(request IpmiRequest, host string, index int) IpmiResponse {
	var res IpmiResponse
	res.Data = ExecIpmiToolRemote(request.Data, al, host)
	res.Source = host
	res.Index = index
	return res
}
