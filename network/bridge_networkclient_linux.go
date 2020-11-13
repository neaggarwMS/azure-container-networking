package network

import (
	"fmt"
	"net"
	"strings"

	"github.com/Azure/azure-container-networking/ebtables"
	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/netlink"
	"github.com/Azure/azure-container-networking/network/epcommon"
)

const (
	multicastSolicitPrefix = "ff02::1:ff00:0/104"
)

type LinuxBridgeClient struct {
	bridgeName        string
	hostInterfaceName string
	nwInfo            NetworkInfo
}

func NewLinuxBridgeClient(bridgeName string, hostInterfaceName string, nwInfo NetworkInfo) *LinuxBridgeClient {
	client := &LinuxBridgeClient{
		bridgeName:        bridgeName,
		nwInfo:            nwInfo,
		hostInterfaceName: hostInterfaceName,
	}

	return client
}

func (client *LinuxBridgeClient) CreateBridge() error {
	log.Printf("[net] Creating bridge %v.", client.bridgeName)

	link := netlink.BridgeLink{
		LinkInfo: netlink.LinkInfo{
			Type: netlink.LINK_TYPE_BRIDGE,
			Name: client.bridgeName,
		},
	}

	if err := netlink.AddLink(&link); err != nil {
		return err
	}

	return epcommon.DisableRAForInterface(client.bridgeName)
}

func (client *LinuxBridgeClient) AddRoutes(nwInfo *NetworkInfo, interfaceName string) error {
	if client.nwInfo.IPAMType == AzureCNS {
		// add pod subnet to host
		devIf, _ := net.InterfaceByName(interfaceName)
		ifIndex := devIf.Index
		family := netlink.GetIpAddressFamily(Ipv4DefaultRouteDstPrefix.IP)

		nlRoute := &netlink.Route{
			Family:    family,
			Dst:       &client.nwInfo.PodSubnet.Prefix,
			Gw:        Ipv4DefaultRouteDstPrefix.IP,
			LinkIndex: ifIndex,
		}

		if err := netlink.AddIpRoute(nlRoute); err != nil {
			if !strings.Contains(strings.ToLower(err.Error()), "file exists") {
				return fmt.Errorf("Failed to add route to host interface with error: %v", err)
			}
			log.Printf("[cni-cns-net] route already exists: dst %+v, gw %+v, interfaceName %v", nlRoute.Dst, nlRoute.Gw, interfaceName)
		}

		// Add snat Rules
		snatIP := client.nwInfo.Options[SNATIPKey]
		if snatIP == nil {
			return fmt.Errorf("snatIP in Options not set %v", snatIP)
		}

		ncPrimaryIP := net.ParseIP(fmt.Sprintf("%v", snatIP))
		if ncPrimaryIP == nil {
			return fmt.Errorf("Failed to parse SNAT IP from options %v", client.nwInfo.Options)
		}

		epcommon.SNATfromSubnetToDNSWithNCPrimaryIP(ncPrimaryIP, client.nwInfo.PodSubnet.Prefix)
	}
	return nil
}

func (client *LinuxBridgeClient) DeleteBridge() error {
	// Disconnect external interface from its bridge.
	err := netlink.SetLinkMaster(client.hostInterfaceName, "")
	if err != nil {
		log.Printf("[net] Failed to disconnect interface %v from bridge, err:%v.", client.hostInterfaceName, err)
	}

	// Delete the bridge.
	err = netlink.DeleteLink(client.bridgeName)
	if err != nil {
		log.Printf("[net] Failed to delete bridge %v, err:%v.", client.bridgeName, err)
	}

	return nil
}

func (client *LinuxBridgeClient) AddL2Rules(extIf *externalInterface) error {
	hostIf, err := net.InterfaceByName(client.hostInterfaceName)
	if err != nil {
		return err
	}

	// Add SNAT rule to translate container egress traffic.
	log.Printf("[net] Adding SNAT rule for egress traffic on %v.", client.hostInterfaceName)
	if err := ebtables.SetSnatForInterface(client.hostInterfaceName, hostIf.HardwareAddr, ebtables.Append); err != nil {
		return err
	}

	// Add ARP reply rule for host primary IP address.
	// ARP requests for all IP addresses are forwarded to the SDN fabric, but fabric
	// doesn't respond to ARP requests from the VM for its own primary IP address.
	primary := extIf.IPAddresses[0].IP
	log.Printf("[net] Adding ARP reply rule for primary IP address %v.", primary)
	if err := ebtables.SetArpReply(primary, hostIf.HardwareAddr, ebtables.Append); err != nil {
		return err
	}

	// Add DNAT rule to forward ARP replies to container interfaces.
	log.Printf("[net] Adding DNAT rule for ingress ARP traffic on interface %v.", client.hostInterfaceName)
	if err := ebtables.SetDnatForArpReplies(client.hostInterfaceName, ebtables.Append); err != nil {
		return err
	}

	if client.nwInfo.IPV6Mode != "" {
		// for ipv6 node cidr set broute accept
		if err := ebtables.SetBrouteAcceptByCidr(&client.nwInfo.Subnets[1].Prefix, ebtables.IPV6, ebtables.Append, ebtables.Accept); err != nil {
			return err
		}

		_, mIpNet, _ := net.ParseCIDR(multicastSolicitPrefix)
		if err := ebtables.SetBrouteAcceptByCidr(mIpNet, ebtables.IPV6, ebtables.Append, ebtables.Accept); err != nil {
			return err
		}

		if err := ebtables.DropICMPv6Solicitation(client.hostInterfaceName, ebtables.Append); err != nil {
			return err
		}

		if err := client.setBrouteRedirect(ebtables.Append); err != nil {
			return err
		}

		if err := epcommon.EnableIPV6Forwarding(); err != nil {
			return err
		}
	}

	// Enable VEPA for host policy enforcement if necessary.
	if client.nwInfo.Mode == opModeTunnel {
		log.Printf("[net] Enabling VEPA mode for %v.", client.hostInterfaceName)
		if err := ebtables.SetVepaMode(client.bridgeName, commonInterfacePrefix, virtualMacAddress, ebtables.Append); err != nil {
			return err
		}
	}

	return nil
}

func (client *LinuxBridgeClient) DeleteL2Rules(extIf *externalInterface) {
	ebtables.SetVepaMode(client.bridgeName, commonInterfacePrefix, virtualMacAddress, ebtables.Delete)
	ebtables.SetDnatForArpReplies(extIf.Name, ebtables.Delete)
	ebtables.SetArpReply(extIf.IPAddresses[0].IP, extIf.MacAddress, ebtables.Delete)
	ebtables.SetSnatForInterface(extIf.Name, extIf.MacAddress, ebtables.Delete)
	if client.nwInfo.IPV6Mode != "" {
		if len(extIf.IPAddresses) > 1 {
			ebtables.SetBrouteAcceptByCidr(extIf.IPAddresses[1], ebtables.IPV6, ebtables.Delete, ebtables.Accept)
		}
		_, mIpNet, _ := net.ParseCIDR(multicastSolicitPrefix)
		ebtables.SetBrouteAcceptByCidr(mIpNet, ebtables.IPV6, ebtables.Delete, ebtables.Accept)
		client.setBrouteRedirect(ebtables.Delete)
		ebtables.DropICMPv6Solicitation(extIf.Name, ebtables.Delete)
	}
}

func (client *LinuxBridgeClient) SetBridgeMasterToHostInterface() error {
	return netlink.SetLinkMaster(client.hostInterfaceName, client.bridgeName)
}

func (client *LinuxBridgeClient) SetHairpinOnHostInterface(enable bool) error {
	return netlink.SetLinkHairpin(client.hostInterfaceName, enable)
}

func (client *LinuxBridgeClient) setBrouteRedirect(action string) error {
	if client.nwInfo.ServiceCidrs != "" {
		if err := ebtables.SetBrouteAcceptByCidr(nil, ebtables.IPV4, ebtables.Append, ebtables.RedirectAccept); err != nil {
			return err
		}

		if err := ebtables.SetBrouteAcceptByCidr(nil, ebtables.IPV6, ebtables.Append, ebtables.RedirectAccept); err != nil {
			return err
		}
	}

	return nil
}
