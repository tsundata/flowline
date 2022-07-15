package cache

import (
	"errors"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/util/flog"
)

// workerTree is a tree-like data structure that holds node names in each zone. Zone names are
// keys to "NodeTree.tree" and values of "NodeTree.tree" are arrays of node names.
// NodeTree is NOT thread-safe, any concurrent updates/reads from it must be synchronized by the caller.
// It is used only by schedulerCache, and should stay as such.
type workerTree struct {
	tree     map[string][]string // a map from zone (region-zone) to an array of nodes in the zone.
	zones    []string            // a list of all the zones in the tree (keys)
	numNodes int
}

// newWorkerTree creates a NodeTree from nodes.
func newWorkerTree(workers []*meta.Worker) *workerTree {
	nt := &workerTree{
		tree: make(map[string][]string),
	}
	for _, n := range workers {
		nt.addWorker(n)
	}
	return nt
}

// addNode adds a node and its corresponding zone to the tree. If the zone already exists, the node
// is added to the array of nodes in that zone.
func (nt *workerTree) addWorker(n *meta.Worker) {
	zone := GetZoneKey(n)
	if na, ok := nt.tree[zone]; ok {
		for _, nodeName := range na {
			if nodeName == n.Name {
				flog.Infof("Node already exists in the NodeTree")
				return
			}
		}
		nt.tree[zone] = append(na, n.Name)
	} else {
		nt.zones = append(nt.zones, zone)
		nt.tree[zone] = []string{n.Name}
	}
	flog.Infof("Added node in listed group to NodeTree %v", zone)
	nt.numNodes++
}

// removeNode removes a node from the NodeTree.
func (nt *workerTree) removeWorker(n *meta.Worker) error {
	zone := GetZoneKey(n)
	if na, ok := nt.tree[zone]; ok {
		for i, nodeName := range na {
			if nodeName == n.Name {
				nt.tree[zone] = append(na[:i], na[i+1:]...)
				if len(nt.tree[zone]) == 0 {
					nt.removeZone(zone)
				}
				flog.Infof("Removed node in listed group from NodeTree %v", zone)
				nt.numNodes--
				return nil
			}
		}
	}
	flog.Errorf("Node in listed group was not found %v", zone)
	return fmt.Errorf("node %q in group %q was not found", n.Name, zone)
}

// removeZone removes a zone from tree.
// This function must be called while writer locks are hold.
func (nt *workerTree) removeZone(zone string) {
	delete(nt.tree, zone)
	for i, z := range nt.zones {
		if z == zone {
			nt.zones = append(nt.zones[:i], nt.zones[i+1:]...)
			return
		}
	}
}

// updateNode updates a node in the NodeTree.
func (nt *workerTree) updateWorker(old, new *meta.Worker) {
	var oldZone string
	if old != nil {
		oldZone = GetZoneKey(old)
	}
	newZone := GetZoneKey(new)
	// If the zone ID of the node has not changed, we don't need to do anything. Name of the node
	// cannot be changed in an update.
	if oldZone == newZone {
		return
	}
	nt.removeWorker(old) // No error checking. We ignore whether the old node exists or not.
	nt.addWorker(new)
}

// list returns the list of names of the node. NodeTree iterates over zones and in each zone iterates
// over nodes in a round robin fashion.
func (nt *workerTree) list() ([]string, error) {
	if len(nt.zones) == 0 {
		return nil, nil
	}
	nodesList := make([]string, 0, nt.numNodes)
	numExhaustedZones := 0
	nodeIndex := 0
	for len(nodesList) < nt.numNodes {
		if numExhaustedZones >= len(nt.zones) { // all zones are exhausted.
			return nodesList, errors.New("all zones exhausted before reaching count of nodes expected")
		}
		for zoneIndex := 0; zoneIndex < len(nt.zones); zoneIndex++ {
			na := nt.tree[nt.zones[zoneIndex]]
			if nodeIndex >= len(na) { // If the zone is exhausted, continue
				if nodeIndex == len(na) { // If it is the first time the zone is exhausted
					numExhaustedZones++
				}
				continue
			}
			nodesList = append(nodesList, na[nodeIndex])
		}
		nodeIndex++
	}
	return nodesList, nil
}

const (
	LabelTopologyZone   = "zone"
	LabelTopologyRegion = "region"
)

func GetZoneKey(node *meta.Worker) string {
	labels := node.Labels
	if labels == nil {
		return ""
	}

	zone, _ := labels[LabelTopologyZone]
	region, _ := labels[LabelTopologyRegion]

	if region == "" && zone == "" {
		return ""
	}

	// We include the null character just in case region or failureDomain has a colon
	// (We do assume there's no null characters in a region or failureDomain)
	// As a nice side-benefit, the null character is not printed by fmt.Print or glog
	return region + ":\x00:" + zone
}
