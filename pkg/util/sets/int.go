package sets

import (
	"reflect"
	"sort"
)

// Int sets.Int is a set of ints, implemented via map[int]struct{} for minimal memory consumption.
type Int map[int]struct{}

// NewInt creates a Int from a list of values.
func NewInt(items ...int) Int {
	ss := Int{}
	ss.Insert(items...)
	return ss
}

// IntKeySet creates a Int from a keys of a map[int](? extends interface{}).
// If the value passed in is not actually a map, this will panic.
func IntKeySet(theMap interface{}) Int {
	v := reflect.ValueOf(theMap)
	ret := Int{}

	for _, keyValue := range v.MapKeys() {
		ret.Insert(keyValue.Interface().(int))
	}
	return ret
}

// Insert adds items to the set.
func (s1 Int) Insert(items ...int) Int {
	for _, item := range items {
		s1[item] = struct{}{}
	}
	return s1
}

// Delete removes all items from the set.
func (s1 Int) Delete(items ...int) Int {
	for _, item := range items {
		delete(s1, item)
	}
	return s1
}

// Has returns true if and only if item is contained in the set.
func (s1 Int) Has(item int) bool {
	_, contained := s1[item]
	return contained
}

// HasAll returns true if and only if all items are contained in the set.
func (s1 Int) HasAll(items ...int) bool {
	for _, item := range items {
		if !s1.Has(item) {
			return false
		}
	}
	return true
}

// HasAny returns true if any items are contained in the set.
func (s1 Int) HasAny(items ...int) bool {
	for _, item := range items {
		if s1.Has(item) {
			return true
		}
	}
	return false
}

// Difference returns a set of objects that are not in s2
// For example:
// s1 = {a1, a2, a3}
// s2 = {a1, a2, a4, a5}
// s1.Difference(s2) = {a3}
// s2.Difference(s1) = {a4, a5}
func (s1 Int) Difference(s2 Int) Int {
	result := NewInt()
	for key := range s1 {
		if !s2.Has(key) {
			result.Insert(key)
		}
	}
	return result
}

// Union returns a new set which includes items in either s1 or s2.
// For example:
// s1 = {a1, a2}
// s2 = {a3, a4}
// s1.Union(s2) = {a1, a2, a3, a4}
// s2.Union(s1) = {a1, a2, a3, a4}
func (s1 Int) Union(s2 Int) Int {
	result := NewInt()
	for key := range s1 {
		result.Insert(key)
	}
	for key := range s2 {
		result.Insert(key)
	}
	return result
}

// Intersection returns a new set which includes the item in BOTH s1 and s2
// For example:
// s1 = {a1, a2}
// s2 = {a2, a3}
// s1.Intersection(s2) = {a2}
func (s1 Int) Intersection(s2 Int) Int {
	var walk, other Int
	result := NewInt()
	if s1.Len() < s2.Len() {
		walk = s1
		other = s2
	} else {
		walk = s2
		other = s1
	}
	for key := range walk {
		if other.Has(key) {
			result.Insert(key)
		}
	}
	return result
}

// IsSuperset returns true if and only if s1 is a superset of s2.
func (s1 Int) IsSuperset(s2 Int) bool {
	for item := range s2 {
		if !s1.Has(item) {
			return false
		}
	}
	return true
}

// Equal returns true if and only if s1 is equal (as a set) to s2.
// Two sets are equal if their membership is identical.
// (In practice, this means same elements, order doesn't matter)
func (s1 Int) Equal(s2 Int) bool {
	return len(s1) == len(s2) && s1.IsSuperset(s2)
}

type sortableSliceOfInt []int

func (s sortableSliceOfInt) Len() int           { return len(s) }
func (s sortableSliceOfInt) Less(i, j int) bool { return lessInt(s[i], s[j]) }
func (s sortableSliceOfInt) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// List returns the contents as a sorted int slice.
func (s1 Int) List() []int {
	res := make(sortableSliceOfInt, 0, len(s1))
	for key := range s1 {
		res = append(res, key)
	}
	sort.Sort(res)
	return []int(res)
}

// UnsortedList returns the slice with contents in random order.
func (s1 Int) UnsortedList() []int {
	res := make([]int, 0, len(s1))
	for key := range s1 {
		res = append(res, key)
	}
	return res
}

// PopAny Returns a single element from the set.
func (s1 Int) PopAny() (int, bool) {
	for key := range s1 {
		s1.Delete(key)
		return key, true
	}
	var zeroValue int
	return zeroValue, false
}

// Len returns the size of the set.
func (s1 Int) Len() int {
	return len(s1)
}

func lessInt(lhs, rhs int) bool {
	return lhs < rhs
}
