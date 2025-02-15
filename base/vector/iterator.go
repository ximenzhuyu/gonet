package vector

import "gonet/base/containers"

func assertIteratorImplementation() {
	var _ containers.ReverseIteratorWithIndex = (*Iterator)(nil)
}

// Iterator holding the iterator's state
type Iterator struct {
	vec   *Vector
	index int
}

// Iterator returns a stateful iterator whose values can be fetched by an index.
func (v *Vector) Iterator() Iterator {
	return Iterator{vec: v, index: -1}
}

// Next moves the iterator to the next element and returns true if there was a next element in the container.
// If Next() returns true, then next element's index and value can be retrieved by Index() and Value().
// If Next() was called for the first time, then it will point the iterator to the first element if it exists.
// Modifies the state of the iterator.
func (v *Iterator) Next() bool {
	if v.index < v.vec.elementCount {
		v.index++
	}
	return v.vec.withinRange(v.index)
}

// Prev moves the iterator to the previous element and returns true if there was a previous element in the container.
// If Prev() returns true, then previous element's index and value can be retrieved by Index() and Value().
// Modifies the state of the iterator.
func (v *Iterator) Prev() bool {
	if v.index >= 0 {
		v.index--
	}
	return v.vec.withinRange(v.index)
}

// Value returns the current element's value.
// Does not modify the state of the iterator.
func (v *Iterator) Value() interface{} {
	return v.vec.Get(v.index)
}

// Index returns the current element's index.
// Does not modify the state of the iterator.
func (v *Iterator) Index() int {
	return v.index
}

// Begin resets the iterator to its initial state (one-before-first)
// Call Next() to fetch the first element if any.
func (v *Iterator) Begin() {
	v.index = -1
}

// End moves the iterator past the last element (one-past-the-end).
// Call Prev() to fetch the last element if any.
func (v *Iterator) End() {
	v.index = v.vec.elementCount
}

// First moves the iterator to the first element and returns true if there was a first element in the container.
// If First() returns true, then first element's index and value can be retrieved by Index() and Value().
// Modifies the state of the iterator.
func (v *Iterator) First() bool {
	v.Begin()
	return v.Next()
}

// Last moves the iterator to the last element and returns true if there was a last element in the container.
// If Last() returns true, then last element's index and value can be retrieved by Index() and Value().
// Modifies the state of the iterator.
func (v *Iterator) Last() bool {
	v.End()
	return v.Prev()
}
