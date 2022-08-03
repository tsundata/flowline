package cache

import (
	"errors"
	"fmt"
	"github.com/tsundata/flowline/pkg/api/meta"
	"github.com/tsundata/flowline/pkg/util/flog"
)

// workerTree is a tree-like data structure that holds worker names in each zone. Zone names are
// keys to "WorkerTree.tree" and values of "WorkerTree.tree" are arrays of worker names.
// WorkerTree is NOT thread-safe, any concurrent updates/reads from it must be synchronized by the caller.
// It is used only by schedulerCache, and should stay as such.
type workerTree struct {
	tree       map[string][]string // a map from zone (region-zone) to an array of workers in the zone.
	zones      []string            // a list of all the zones in the tree (keys)
	numWorkers int
}

// newWorkerTree creates a WorkerTree from workers.
func newWorkerTree(workers []*meta.Worker) *workerTree {
	nt := &workerTree{
		tree: make(map[string][]string),
	}
	for _, n := range workers {
		nt.addWorker(n)
	}
	return nt
}

// addWorker adds a worker and its corresponding zone to the tree. If the zone already exists, the worker
// is added to the array of workers in that zone.
func (nt *workerTree) addWorker(n *meta.Worker) {
	zone := GetZoneKey(n)
	if na, ok := nt.tree[zone]; ok {
		for _, workerName := range na {
			if workerName == n.Name {
				flog.Infof("Worker already exists in the WorkerTree")
				return
			}
		}
		nt.tree[zone] = append(na, n.Name)
	} else {
		nt.zones = append(nt.zones, zone)
		nt.tree[zone] = []string{n.Name}
	}
	flog.Infof("Added worker in listed group to WorkerTree %v", zone)
	nt.numWorkers++
}

// removeWorker removes a worker from the WorkerTree.
func (nt *workerTree) removeWorker(n *meta.Worker) error {
	zone := GetZoneKey(n)
	if na, ok := nt.tree[zone]; ok {
		for i, workerName := range na {
			if workerName == n.Name {
				nt.tree[zone] = append(na[:i], na[i+1:]...)
				if len(nt.tree[zone]) == 0 {
					nt.removeZone(zone)
				}
				flog.Infof("Removed worker in listed group from WorkerTree %v", zone)
				nt.numWorkers--
				return nil
			}
		}
	}
	flog.Errorf("Worker in listed group was not found %v", zone)
	return fmt.Errorf("worker %q in group %q was not found", n.Name, zone)
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

// updateWorker updates a worker in the WorkerTree.
func (nt *workerTree) updateWorker(old, new *meta.Worker) {
	var oldZone string
	if old != nil {
		oldZone = GetZoneKey(old)
	}
	newZone := GetZoneKey(new)
	// If the zone ID of the worker has not changed, we don't need to do anything. Name of the worker
	// cannot be changed in an update.
	if oldZone == newZone {
		return
	}
	_ = nt.removeWorker(old) // No error checking. We ignore whether the old worker exists or not.
	nt.addWorker(new)
}

// list returns the list of names of the worker. WorkerTree iterates over zones and in each zone iterates
// over workers in a round-robin fashion.
func (nt *workerTree) list() ([]string, error) {
	if len(nt.zones) == 0 {
		return nil, nil
	}
	workersList := make([]string, 0, nt.numWorkers)
	numExhaustedZones := 0
	workerIndex := 0
	for len(workersList) < nt.numWorkers {
		if numExhaustedZones >= len(nt.zones) { // all zones are exhausted.
			return workersList, errors.New("all zones exhausted before reaching count of workers expected")
		}
		for zoneIndex := 0; zoneIndex < len(nt.zones); zoneIndex++ {
			na := nt.tree[nt.zones[zoneIndex]]
			if workerIndex >= len(na) { // If the zone is exhausted, continue
				if workerIndex == len(na) { // If it is the first time the zone is exhausted
					numExhaustedZones++
				}
				continue
			}
			workersList = append(workersList, na[workerIndex])
		}
		workerIndex++
	}
	return workersList, nil
}

const (
	LabelTopologyZone   = "zone"
	LabelTopologyRegion = "region"
)

func GetZoneKey(worker *meta.Worker) string {
	labels := worker.Labels
	if labels == nil {
		return ""
	}

	zone, _ := labels[LabelTopologyZone]     //nolint
	region, _ := labels[LabelTopologyRegion] //nolint

	if region == "" && zone == "" {
		return ""
	}

	// We include the null character just in case region or failureDomain has a colon
	// (We do assume there's no null characters in a region or failureDomain)
	// As a nice side-benefit, the null character is not printed by fmt.Print or glog
	return region + ":\x00:" + zone
}
