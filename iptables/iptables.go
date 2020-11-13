package iptables

// This package contains wrapper functions to program iptables rules

import (
	"fmt"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/platform"
)

// cni iptable chains
const (
	CNIInputChain  = "AZURECNIINPUT"
	CNIOutputChain = "AZURECNIOUTPUT"
)

// standard iptable chains
const (
	Input       = "INPUT"
	Output      = "OUTPUT"
	Forward     = "FORWARD"
	Prerouting  = "PREROUTING"
	Postrouting = "POSTROUTING"
	Swift       = "SWIFT"
	Snat        = "SNAT"
	Return      = "RETURN"
)

// Standard Table names
const (
	Filter = "filter"
	Nat    = "nat"
	Mangle = "mangle"
)

// target
const (
	Accept     = "ACCEPT"
	Drop       = "DROP"
	Masquerade = "MASQUERADE"
)

// actions
const (
	Insert = "I"
	Append = "A"
	Delete = "D"
)

// states
const (
	Established = "ESTABLISHED"
	Related     = "RELATED"
)

const (
	iptables    = "iptables"
	ip6tables   = "ip6tables"
	lockTimeout = 60
)

const (
	V4 = "4"
	V6 = "6"
)

// known ports
const (
	DNSPort = 53
)

// known protocols
const (
	UDP = "udp"
	TCP = "tcp"
)

// known IP's
const (
	AzureDNS = "168.63.129.16"
)

var (
	DisableIPTableLock bool
)

// Run iptables command
func runCmd(version, params string) error {
	var cmd string

	iptCmd := iptables
	if version == V6 {
		iptCmd = ip6tables
	}

	if DisableIPTableLock {
		cmd = fmt.Sprintf("%s %s", iptCmd, params)
	} else {
		cmd = fmt.Sprintf("%s -w %d %s", iptCmd, lockTimeout, params)
	}

	if _, err := platform.ExecuteCommand(cmd); err != nil {
		return err
	}

	return nil
}

// check if iptable chain alreay exists
func ChainExists(version, tableName, chainName string) bool {
	params := fmt.Sprintf("-t %s -L %s", tableName, chainName)
	if err := runCmd(version, params); err != nil {
		return false
	}

	return true
}

// create new iptable chain under specified table name
func CreateChain(version, tableName, chainName string) error {
	var err error

	if !ChainExists(version, tableName, chainName) {
		params := fmt.Sprintf("-t %s -N %s", tableName, chainName)
		err = runCmd(version, params)
	} else {
		log.Printf("%s Chain exists in table %s", chainName, tableName)
	}

	return err
}

// check if iptable rule alreay exists
func RuleExists(version, tableName, chainName, match, target string) bool {
	params := fmt.Sprintf("-t %s -C %s %s -j %s", tableName, chainName, match, target)
	if err := runCmd(version, params); err != nil {
		return false
	}
	return true
}

// Insert iptable rule at beginning of iptable chain
func InsertIptableRule(version, tableName, chainName, match, target string) error {
	if RuleExists(version, tableName, chainName, match, target) {
		log.Printf("Rule already exists")
		return nil
	}

	params := fmt.Sprintf("-t %s -I %s 1 %s -j %s", tableName, chainName, match, target)
	return runCmd(version, params)
}

// Append iptable rule at end of iptable chain
func AppendIptableRule(version, tableName, chainName, match, target string) error {
	if RuleExists(version, tableName, chainName, match, target) {
		log.Printf("Rule already exists")
		return nil
	}

	params := fmt.Sprintf("-t %s -A %s %s -j %s", tableName, chainName, match, target)
	return runCmd(version, params)
}

// Delete matched iptable rule
func DeleteIptableRule(version, tableName, chainName, match, target string) error {
	params := fmt.Sprintf("-t %s -D %s %s -j %s", tableName, chainName, match, target)
	return runCmd(version, params)
}
