// Copyright 2018 Microsoft. All rights reserved.
// MIT License
package ipsm

import (
	"fmt"
	"os"
	"testing"

	"github.com/Azure/azure-container-networking/npm/metrics"
	"github.com/Azure/azure-container-networking/npm/metrics/promutil"
	"github.com/Azure/azure-container-networking/npm/util"
)

func TestSave(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestSave failed @ ipsMgr.Save")
	}
}

func TestRestore(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestRestore failed @ ipsMgr.Save")
	}

	if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestRestore failed @ ipsMgr.Restore")
	}
}

func TestCreateList(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestCreateList failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestCreateList failed @ ipsMgr.Restore")
		}
	}()

	if err := ipsMgr.CreateList("test-list"); err != nil {
		t.Errorf("TestCreateList failed @ ipsMgr.CreateList")
	}
}

func TestDeleteList(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestDeleteList failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestDeleteList failed @ ipsMgr.Restore")
		}
	}()

	if err := ipsMgr.CreateList("test-list"); err != nil {
		t.Errorf("TestDeleteList failed @ ipsMgr.CreateList")
	}

	if err := ipsMgr.DeleteList("test-list"); err != nil {
		t.Errorf("TestDeleteList failed @ ipsMgr.DeleteList")
	}
}

func TestAddToList(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestAddToList failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestAddToList failed @ ipsMgr.Restore")
		}
	}()

	if err := ipsMgr.CreateSet("test-set", append([]string{util.IpsetNetHashFlag})); err != nil {
		t.Errorf("TestAddToList failed @ ipsMgr.CreateSet")
	}

	if err := ipsMgr.AddToList("test-list", "test-set"); err != nil {
		t.Errorf("TestAddToList failed @ ipsMgr.AddToList")
	}
}

func TestDeleteFromList(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestDeleteFromList failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestDeleteFromList failed @ ipsMgr.Restore")
		}
	}()

	// Create set and validate set is created.
	setName := "test-set"
	if err := ipsMgr.CreateSet(setName, append([]string{util.IpsetNetHashFlag})); err != nil {
		t.Errorf("TestDeleteFromList failed @ ipsMgr.CreateSet")
	}

	entry := &ipsEntry{
		operationFlag: util.IPsetCheckListFlag,
		set:           util.GetHashedName(setName),
	}

	if _, err := ipsMgr.Run(entry); err != nil {
		t.Errorf("TestDeleteFromList failed @ ipsMgr.CreateSet since %s not exist in kernel", setName)
	}

	// Create list, add set to list and validate set is in the list.
	listName := "test-list"
	if err := ipsMgr.AddToList(listName, setName); err != nil {
		t.Errorf("TestDeleteFromList failed @ ipsMgr.AddToList")
	}

	entry = &ipsEntry{
		operationFlag: util.IpsetTestFlag,
		set:           util.GetHashedName(listName),
		spec:          append([]string{util.GetHashedName(setName)}),
	}

	if _, err := ipsMgr.Run(entry); err != nil {
		t.Errorf("TestDeleteFromList failed @ ipsMgr.AddToList since %s not exist in %s set", listName, setName)
	}

	// Delete set from list and validate set is not in list anymore.
	if err := ipsMgr.DeleteFromList(listName, setName); err != nil {
		t.Errorf("TestDeleteFromList failed @ ipsMgr.DeleteFromList")
	}

	entry = &ipsEntry{
		operationFlag: util.IpsetTestFlag,
		set:           util.GetHashedName(listName),
		spec:          append([]string{util.GetHashedName(setName)}),
	}

	if _, err := ipsMgr.Run(entry); err == nil {
		t.Errorf("TestDeleteFromList failed @ ipsMgr.DeleteFromList since %s still exist in %s set", listName, setName)
	}

	// Delete List and validate list is not exist.

	if err := ipsMgr.DeleteSet(listName); err != nil {
		t.Errorf("TestDeleteSet failed @ ipsMgr.DeleteSet")
	}

	entry = &ipsEntry{
		operationFlag: util.IPsetCheckListFlag,
		set:           util.GetHashedName(listName),
	}

	if _, err := ipsMgr.Run(entry); err == nil {
		t.Errorf("TestDeleteFromList failed @ ipsMgr.DeleteSet since %s still exist in kernel", listName)
	}

	// Delete set and validate set is not exist.
	if err := ipsMgr.DeleteSet(setName); err != nil {
		t.Errorf("TestDeleteSet failed @ ipsMgr.DeleteSet")
	}

	entry = &ipsEntry{
		operationFlag: util.IPsetCheckListFlag,
		set:           util.GetHashedName(setName),
	}

	if _, err := ipsMgr.Run(entry); err == nil {
		t.Errorf("TestDeleteFromList failed @ ipsMgr.DeleteSet since %s still exist in kernel", setName)
	}
}

func TestCreateSet(t *testing.T) {
	metrics.NumIPSetEntries.Set(0)
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestCreateSet failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestCreateSet failed @ ipsMgr.Restore")
		}
	}()

	gaugeVal, err1 := promutil.GetValue(metrics.NumIPSets)
	countVal, err2 := promutil.GetCountValue(metrics.AddIPSetExecTime)

	testSet1Name := "test-set"
	if err := ipsMgr.CreateSet(testSet1Name, []string{util.IpsetNetHashFlag}); err != nil {
		t.Errorf("TestCreateSet failed @ ipsMgr.CreateSet")
	}

	testSet2Name := "test-set-with-maxelem"
	spec := append([]string{util.IpsetNetHashFlag, util.IpsetMaxelemName, util.IpsetMaxelemNum})
	if err := ipsMgr.CreateSet(testSet2Name, spec); err != nil {
		t.Errorf("TestCreateSet failed @ ipsMgr.CreateSet when set maxelem")
	}

	testSet3Name := "test-set-with-port"
	spec = append([]string{util.IpsetIPPortHashFlag})
	if err := ipsMgr.CreateSet(testSet3Name, spec); err != nil {
		t.Errorf("TestCreateSet failed @ ipsMgr.CreateSet when creating port set")
	}
	if err := ipsMgr.AddToSet(testSet3Name, fmt.Sprintf("%s,%s%d", "1.1.1.1", "tcp", 8080), util.IpsetIPPortHashFlag, "0"); err != nil {
		t.Errorf("AddToSet failed @ ipsMgr.CreateSet when set port")
	}

	newGaugeVal, err3 := promutil.GetValue(metrics.NumIPSets)
	newCountVal, err4 := promutil.GetCountValue(metrics.AddIPSetExecTime)
	testSet1Count, err5 := promutil.GetVecValue(metrics.IPSetInventory, metrics.GetIPSetInventoryLabels(testSet1Name))
	testSet2Count, err6 := promutil.GetVecValue(metrics.IPSetInventory, metrics.GetIPSetInventoryLabels(testSet2Name))
	testSet3Count, err7 := promutil.GetVecValue(metrics.IPSetInventory, metrics.GetIPSetInventoryLabels(testSet3Name))
	entryCount, err8 := promutil.GetValue(metrics.NumIPSetEntries)
	promutil.NotifyIfErrors(t, err1, err2, err3, err4, err5, err6, err7, err8)
	if newGaugeVal != gaugeVal+3 {
		t.Errorf("Change in ipset number didn't register in Prometheus")
	}
	if newCountVal != countVal+3 {
		t.Errorf("Execution time didn't register in Prometheus")
	}
	if testSet1Count != 0 || testSet2Count != 0 || testSet3Count != 1 || entryCount != 1 {
		t.Errorf("Prometheus IPSet count has incorrect number of entries")
	}
}

func TestDeleteSet(t *testing.T) {
	metrics.NumIPSetEntries.Set(0)
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestDeleteSet failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestDeleteSet failed @ ipsMgr.Restore")
		}
	}()

	testSetName := "test-set"
	if err := ipsMgr.CreateSet(testSetName, append([]string{util.IpsetNetHashFlag})); err != nil {
		t.Errorf("TestDeleteSet failed @ ipsMgr.CreateSet")
	}

	gaugeVal, err1 := promutil.GetValue(metrics.NumIPSets)

	if err := ipsMgr.DeleteSet(testSetName); err != nil {
		t.Errorf("TestDeleteSet failed @ ipsMgr.DeleteSet")
	}

	newGaugeVal, err2 := promutil.GetValue(metrics.NumIPSets)
	testSetCount, err3 := promutil.GetVecValue(metrics.IPSetInventory, metrics.GetIPSetInventoryLabels(testSetName))
	entryCount, err4 := promutil.GetValue(metrics.NumIPSetEntries)
	promutil.NotifyIfErrors(t, err1, err2, err3, err4)
	if newGaugeVal != gaugeVal-1 {
		t.Errorf("Change in ipset number didn't register in prometheus")
	}
	if testSetCount != 0 || entryCount != 0 {
		t.Errorf("Prometheus IPSet count has incorrect number of entries")
	}
}

func TestAddToSet(t *testing.T) {
	metrics.NumIPSetEntries.Set(0)
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestAddToSet failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestAddToSet failed @ ipsMgr.Restore")
		}
	}()

	testSetName := "test-set"
	if err := ipsMgr.AddToSet(testSetName, "1.2.3.4", util.IpsetNetHashFlag, ""); err != nil {
		t.Errorf("TestAddToSet failed @ ipsMgr.AddToSet")
	}

	if err := ipsMgr.AddToSet(testSetName, "1.2.3.4/nomatch", util.IpsetNetHashFlag, ""); err != nil {
		t.Errorf("TestAddToSet with nomatch failed @ ipsMgr.AddToSet")
	}

	testSetCount, err1 := promutil.GetVecValue(metrics.IPSetInventory, metrics.GetIPSetInventoryLabels(testSetName))
	entryCount, err2 := promutil.GetValue(metrics.NumIPSetEntries)
	promutil.NotifyIfErrors(t, err1, err2)
	if testSetCount != 2 || entryCount != 2 {
		t.Errorf("Prometheus IPSet count has incorrect number of entries")
	}
}

func TestAddToSetWithCachePodInfo(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestAddToSetWithCachePodInfo failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestAddToSetWithCachePodInfo failed @ ipsMgr.Restore")
		}
	}()

	var pod1 = "pod1"
	var setname = "test-podcache_new"
	var ip = "10.0.2.7"
	if err := ipsMgr.AddToSet(setname, ip, util.IpsetNetHashFlag, pod1); err != nil {
		t.Errorf("TestAddToSetWithCachePodInfo with pod1 failed @ ipsMgr.AddToSet, setname: %s, hashedname: %s", setname, util.GetHashedName(setname))
	}

	// validate if Pod1 exists
	cachedPodUid := ipsMgr.setMap[setname].elements[ip]
	if cachedPodUid != pod1 {
		t.Errorf("setname: %s, hashedname: %s is added with wrong podUid: %s, expected: %s", setname, util.GetHashedName(setname), cachedPodUid, pod1)
	}

	// now add pod2 with the same ip. This is possible if DeletePod1 is handled after AddPod2 event callback.
	var pod2 = "pod2"
	if err := ipsMgr.AddToSet(setname, ip, util.IpsetNetHashFlag, pod2); err != nil {
		t.Errorf("TestAddToSetWithCachePodInfo with pod2 failed @ ipsMgr.AddToSet")
	}

	cachedPodUid = ipsMgr.setMap[setname].elements[ip]
	if cachedPodUid != pod2 {
		t.Errorf("setname: %s, hashedname: %s is added with wrong podUid: %s, expected: %s", setname, util.GetHashedName(setname), cachedPodUid, pod2)
	}

	// Delete from set, it will delete the set if this is the last member
	ipsMgr.DeleteFromSet(setname, ip, pod2)
}

func TestDeleteFromSet(t *testing.T) {
	metrics.NumIPSetEntries.Set(0)
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestDeleteFromSet failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestDeleteFromSet failed @ ipsMgr.Restore")
		}
	}()

	testSetName := "test-set"
	if err := ipsMgr.AddToSet(testSetName, "1.2.3.4", util.IpsetNetHashFlag, ""); err != nil {
		t.Errorf("TestDeleteFromSet failed @ ipsMgr.AddToSet")
	}

	if len(ipsMgr.setMap[testSetName].elements) != 1 {
		t.Errorf("TestDeleteFromSet failed @ ipsMgr.AddToSet")
	}

	if err := ipsMgr.DeleteFromSet(testSetName, "1.2.3.4", ""); err != nil {
		t.Errorf("TestDeleteFromSet failed @ ipsMgr.DeleteFromSet")
	}

	// After deleting the only entry, "1.2.3.4" from "test-set", "test-set" ipset won't exist
	if _, exists := ipsMgr.setMap[testSetName]; exists {
		t.Errorf("TestDeleteFromSet failed @ ipsMgr.DeleteFromSet")
	}

	testSetCount, err1 := promutil.GetVecValue(metrics.IPSetInventory, metrics.GetIPSetInventoryLabels(testSetName))
	entryCount, err2 := promutil.GetValue(metrics.NumIPSetEntries)
	promutil.NotifyIfErrors(t, err1, err2)
	if testSetCount != 0 || entryCount != 0 {
		t.Errorf("Prometheus IPSet count has incorrect number of entries %v", entryCount)
	}
}

func TestDeleteFromSetWithPodCache(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestDeleteFromSetWithPodCache failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestDeleteFromSetWithPodCache failed @ ipsMgr.Restore")
		}
	}()

	var setname = "test-deleteset-withcache"
	var ip = "10.0.2.8"
	var pod1 = "pod1"
	if err := ipsMgr.AddToSet(setname, ip, util.IpsetNetHashFlag, pod1); err != nil {
		t.Errorf("TestDeleteFromSetWithPodCache failed for pod1 @ ipsMgr.AddToSet")
	}

	if len(ipsMgr.setMap[setname].elements) != 1 {
		t.Errorf("TestDeleteFromSetWithPodCache failed @ ipsMgr.AddToSet")
	}

	if err := ipsMgr.DeleteFromSet(setname, ip, pod1); err != nil {
		t.Errorf("TestDeleteFromSetWithPodCache for pod1 failed @ ipsMgr.DeleteFromSet")
	}

	// now add the set again and then replace it with pod2
	var pod2 = "pod2"
	if err := ipsMgr.AddToSet(setname, ip, util.IpsetNetHashFlag, pod1); err != nil {
		t.Errorf("TestDeleteFromSetWithPodCache failed for pod1 @ ipsMgr.AddToSet")
	}

	// Add Pod2 with same ip (This could happen if AddPod2 is served before DeletePod1)
	if err := ipsMgr.AddToSet(setname, ip, util.IpsetNetHashFlag, pod2); err != nil {
		t.Errorf("TestDeleteFromSetWithPodCache failed for pod2 @ ipsMgr.AddToSet")
	}

	// Process DeletePod1
	if err := ipsMgr.DeleteFromSet(setname, ip, pod1); err != nil {
		t.Errorf("TestDeleteFromSetWithPodCache for pod1 failed @ ipsMgr.DeleteFromSet")
	}

	// note the set will stil exist with pod ip
	cachedPodUid := ipsMgr.setMap[setname].elements[ip]
	if cachedPodUid != pod2 {
		t.Errorf("setname: %s, hashedname: %s is added with wrong podUid: %s, expected: %s", setname, util.GetHashedName(setname), cachedPodUid, pod2)
	}

	// Now cleanup and delete pod2
	if err := ipsMgr.DeleteFromSet(setname, ip, pod2); err != nil {
		t.Errorf("TestDeleteFromSetWithPodCache for pod2 failed @ ipsMgr.DeleteFromSet")
	}

	if _, exists := ipsMgr.setMap[setname]; exists {
		t.Errorf("TestDeleteFromSetWithPodCache failed @ ipsMgr.DeleteFromSet")
	}
}

func TestClean(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestClean failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestClean failed @ ipsMgr.Restore")
		}
	}()

	if err := ipsMgr.CreateSet("test-set", append([]string{util.IpsetNetHashFlag})); err != nil {
		t.Errorf("TestClean failed @ ipsMgr.CreateSet")
	}

	if err := ipsMgr.Clean(); err != nil {
		t.Errorf("TestClean failed @ ipsMgr.Clean")
	}
}

func TestDestroy(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestDestroy failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestDestroy failed @ ipsMgr.Restore")
		}
	}()

	if err := ipsMgr.AddToSet("test-destroy-set", "1.2.3.4", util.IpsetNetHashFlag, ""); err != nil {
		t.Errorf("TestDestroy failed @ ipsMgr.AddToSet")
	}

	if err := ipsMgr.Destroy(); err != nil {
		t.Errorf("TestDestroy failed @ ipsMgr.Destroy")
	}
}

func TestRun(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestRun failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestRun failed @ ipsMgr.Restore")
		}
	}()

	entry := &ipsEntry{
		operationFlag: util.IpsetCreationFlag,
		set:           "test-set",
		spec:          append([]string{util.IpsetNetHashFlag}),
	}
	if _, err := ipsMgr.Run(entry); err != nil {
		t.Errorf("TestRun failed @ ipsMgr.Run")
	}
}

func TestMain(m *testing.M) {
	metrics.InitializeAll()
	ipsMgr := NewIpsetManager()
	ipsMgr.Save(util.IpsetConfigFile)

	exitCode := m.Run()

	ipsMgr.Restore(util.IpsetConfigFile)

	os.Exit(exitCode)
}
