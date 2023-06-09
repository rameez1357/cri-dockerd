//go:build windows
// +build windows

/*
Copyright 2021 Mirantis

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

package cni

import (
	"context"
	"fmt"
	"net"
	"time"

	cniTypes020 "github.com/containernetworking/cni/pkg/types/020"
	"github.com/sirupsen/logrus"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"

	"github.com/Mirantis/cri-dockerd/config"
	"github.com/Mirantis/cri-dockerd/network"
)

func getLoNetwork(binDirs []string) *cniNetwork {
	return nil
}

func (plugin *cniNetworkPlugin) platformInit() error {
	return nil
}

// GetPodNetworkStatus : Assuming addToNetwork is idempotent, we can call this API as many times as required to get the IPAddress
func (plugin *cniNetworkPlugin) GetPodNetworkStatus(
	namespace string,
	name string,
	id config.ContainerID,
) (*network.PodNetworkStatus, error) {
	netnsPath, err := plugin.host.GetNetNS(id.ID)
	if err != nil {
		return nil, fmt.Errorf("CNI failed to retrieve network namespace path: %v", err)
	}

	if plugin.getDefaultNetwork() == nil {
		return nil, fmt.Errorf(
			"CNI network not yet initialized, skipping pod network status for container %q",
			id,
		)
	}

	// Because the default backend runtime request timeout is 4 min,so set slightly less than 240 seconds
	// Todo get the timeout from parent ctx
	cniTimeoutCtx, cancelFunc := context.WithTimeout(
		context.Background(),
		network.CNITimeoutSec*time.Second,
	)
	defer cancelFunc()
	result, err := plugin.addToNetwork(
		cniTimeoutCtx,
		plugin.getDefaultNetwork(),
		name,
		namespace,
		id,
		netnsPath,
		nil,
		nil,
	)
	if err != nil {
		logrus.Errorf("error while adding to cni network: %s", err)
		return nil, err
	}

	// Parse the result and get the IPAddress
	var result020 *cniTypes020.Result
	result020, err = cniTypes020.GetResult(result)
	if err != nil {
		logrus.Errorf("error while cni parsing result: %s", err)
		return nil, err
	}

	var list = []net.IP{result020.IP4.IP.IP}

	if result020.IP6 != nil {
		list = append(list, result020.IP6.IP.IP)
	}

	return &network.PodNetworkStatus{IP: result020.IP4.IP.IP, IPs: list}, nil
}

// buildDNSCapabilities builds cniDNSConfig from runtimeapi.DNSConfig.
func buildDNSCapabilities(dnsConfig *runtimeapi.DNSConfig) *cniDNSConfig {
	if dnsConfig != nil {
		return &cniDNSConfig{
			Servers:  dnsConfig.Servers,
			Searches: dnsConfig.Searches,
			Options:  dnsConfig.Options,
		}
	}

	return nil
}
