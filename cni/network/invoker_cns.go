package network

import (
	"encoding/json"
	"fmt"
	"net"
	"strconv"

	"github.com/Azure/azure-container-networking/cni"
	"github.com/Azure/azure-container-networking/cns"
	"github.com/Azure/azure-container-networking/cns/cnsclient"
	"github.com/Azure/azure-container-networking/iptables"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/network"
	cniTypes "github.com/containernetworking/cni/pkg/types"
	cniTypesCurr "github.com/containernetworking/cni/pkg/types/current"
)

const (
	cnsPort = 10090
)

type CNSIPAMInvoker struct {
	podName              string
	podNamespace         string
	primaryInterfaceName string
	cnsClient            *cnsclient.CNSClient
}

type IPv4ResultInfo struct {
	podIPAddress       string
	ncSubnetPrefix     uint8
	ncPrimaryIP        string
	ncGatewayIPAddress string
	hostSubnet         string
	hostPrimaryIP      string
	hostGateway        string
}

func NewCNSInvoker(podName, namespace string) (*CNSIPAMInvoker, error) {
	cnsURL := "http://localhost:" + strconv.Itoa(cnsPort)
	cnsClient, err := cnsclient.InitCnsClient(cnsURL)

	return &CNSIPAMInvoker{
		podName:      podName,
		podNamespace: namespace,
		cnsClient:    cnsClient,
	}, err
}

//Add uses the requestipconfig API in cns, and returns ipv4 and a nil ipv6 as CNS doesn't support IPv6 yet
func (invoker *CNSIPAMInvoker) Add(nwCfg *cni.NetworkConfig, hostSubnetPrefix *net.IPNet, options map[string]interface{}) (*cniTypesCurr.Result, *cniTypesCurr.Result, error) {

	// Parse Pod arguments.
	podInfo := cns.KubernetesPodInfo{PodName: invoker.podName, PodNamespace: invoker.podNamespace}
	orchestratorContext, err := json.Marshal(podInfo)

	log.Printf("Requesting IP for pod %v", podInfo)
	response, err := invoker.cnsClient.RequestIPAddress(orchestratorContext)
	if err != nil {
		log.Printf("Failed to get IP address from CNS with error %v, response: %v", err, response)
		return nil, nil, err
	}

	info := IPv4ResultInfo{
		podIPAddress:       response.PodIpInfo.PodIPConfig.IPAddress,
		ncSubnetPrefix:     response.PodIpInfo.NetworkContainerPrimaryIPConfig.IPSubnet.PrefixLength,
		ncPrimaryIP:        response.PodIpInfo.NetworkContainerPrimaryIPConfig.IPSubnet.IPAddress,
		ncGatewayIPAddress: response.PodIpInfo.NetworkContainerPrimaryIPConfig.GatewayIPAddress,
		hostSubnet:         response.PodIpInfo.HostPrimaryIPInfo.Subnet,
		hostPrimaryIP:      response.PodIpInfo.HostPrimaryIPInfo.PrimaryIP,
		hostGateway:        response.PodIpInfo.HostPrimaryIPInfo.Gateway,
	}

	// set the NC Primary IP in options
	options[network.SNATIPKey] = info.ncPrimaryIP

	log.Printf("[cni-invoker-cns] Received info %v for pod %v", info, podInfo)

	ncgw := net.ParseIP(info.ncGatewayIPAddress)
	if ncgw == nil {
		return nil, nil, fmt.Errorf("Gateway address %v from response is invalid", info.ncGatewayIPAddress)
	}

	// set result ipconfig from CNS Response Body
	ip, ncipnet, err := net.ParseCIDR(info.podIPAddress + "/" + fmt.Sprint(info.ncSubnetPrefix))
	if ip == nil {
		return nil, nil, fmt.Errorf("Unable to parse IP from response: %v with err %v", info.podIPAddress, err)
	}

	// construct ipnet for result
	resultIPnet := net.IPNet{
		IP:   ip,
		Mask: ncipnet.Mask,
	}

	result := &cniTypesCurr.Result{
		IPs: []*cniTypesCurr.IPConfig{
			{
				Version: "4",
				Address: resultIPnet,
				Gateway: ncgw,
			},
		},
		Routes: []*cniTypes.Route{
			{
				Dst: network.Ipv4DefaultRouteDstPrefix,
				GW:  ncgw,
			},
		},
	}

	// set subnet prefix for host vm
	err = SetHostOptions(nwCfg, hostSubnetPrefix, ncipnet, options, info)
	if err != nil {
		return nil, nil, err
	}

	// first result is ipv4, second is ipv6, SWIFT doesn't currently support IPv6
	return result, nil, nil
}

func SetHostOptions(nwCfg *cni.NetworkConfig, hostSubnetPrefix *net.IPNet, ncSubnetPrefix *net.IPNet, options map[string]interface{}, info IPv4ResultInfo) error {
	// get the name of the primary IP address
	_, hostIPNet, err := net.ParseCIDR(info.hostSubnet)
	if err != nil {
		return err
	}

	*hostSubnetPrefix = *hostIPNet

	// get the host ip
	hostIP := net.ParseIP(info.hostPrimaryIP)
	if hostIP == nil {
		return fmt.Errorf("Host IP address %v from response is invalid", info.hostPrimaryIP)
	}

	// get host gateway
	hostGateway := net.ParseIP(info.hostGateway)
	if hostGateway == nil {
		return fmt.Errorf("Host Gateway %v from response is invalid", info.hostGateway)
	}

	// this route is needed when the vm on subnet A needs to send traffic to a pod in subnet B on a different vm
	options[network.RoutesKey] = []network.RouteInfo{
		{
			Dst: *ncSubnetPrefix,
			Gw:  hostGateway,
		},
	}

	azureDNSMatch := fmt.Sprintf(" -m addrtype ! --dst-type local -s %s -d %s -p %s --dport %d", ncSubnetPrefix.String(), iptables.AzureDNS, iptables.UDP, iptables.DNSPort)
	snatPrimaryIPJump := fmt.Sprintf("%s --to %s", iptables.Snat, info.ncPrimaryIP)
	options[network.IPTablesKey] = []iptables.IpTableEntry{
		iptables.GetCreateChainCmd(iptables.V4, iptables.Nat, iptables.Swift),
		iptables.GetAppendIptableRuleCmd(iptables.V4, iptables.Nat, iptables.Postrouting, "", iptables.Swift),
		iptables.GetInsertIptableRuleCmd(iptables.V4, iptables.Nat, iptables.Swift, azureDNSMatch, snatPrimaryIPJump),
	}

	return nil
}

// Delete calls into the releaseipconfiguration API in CNS
func (invoker *CNSIPAMInvoker) Delete(address *net.IPNet, nwCfg *cni.NetworkConfig, options map[string]interface{}) error {

	// Parse Pod arguments.
	podInfo := cns.KubernetesPodInfo{PodName: invoker.podName, PodNamespace: invoker.podNamespace}

	orchestratorContext, err := json.Marshal(podInfo)
	if err != nil {
		return err
	}

	return invoker.cnsClient.ReleaseIPAddress(orchestratorContext)
}
