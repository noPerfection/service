// Package data_type defines the generic data types used in SDS.
//
// Supported data types are:
//   - Queue is the list where the new element is added to the end,
//     but when element is taken its taken from the top.
//     Queue doesn't allow addition of any kind of element. All elements should have the same type.
//   - key_value different kind of maps
//   - serialize functions to serialize any structure to the bytes and vice versa.
package data_type

import (
	"container/list"
	"reflect"
)

type Queue struct {
	l            *list.List
	length       int
	element_type reflect.Type
}

const QUEUE_LENGTH = 10

// Queue of the elements that could contain
// maximum QUEUE_LENGTH amount of elements.
//
// The queue has a function that returns the first element
// by taking it out from the list.
//
// The added elements attached after the last element.
func NewQueue() *Queue {
	return &Queue{
		element_type: nil,
		length:       QUEUE_LENGTH,
		l:            list.New(),
	}
}

func (q *Queue) Len() int {
	return q.l.Len()
}

func (q *Queue) IsEmpty() bool {
	return q.l.Len() == 0
}

func (q *Queue) IsFull() bool {
	return q.l.Len() == q.length
}

// Adds the element to the queue.
// If the element type is not the same as
// the expected type, then
// It will silently drop it.
// Silently drop if the queue is full
func (q *Queue) Push(item interface{}) {
	if q.IsFull() {
		return
	}
	if q.element_type == nil {
		q.element_type = reflect.TypeOf(item)
		q.l.PushBack(item)
	} else if reflect.TypeOf(item) == q.element_type {
		q.l.PushBack(item)
	}
}

// Returns the first element without removing it from the queue
// If there is no element, then returns nil
func (q *Queue) First() interface{} {
	if q.IsEmpty() {
		return nil
	}
	return q.l.Front().Value
}

// Takes from the list and returns it.
// If there is no element in the list, then returns nil
func (q *Queue) Pop() interface{} {
	if q.IsEmpty() {
		return nil
	}
	return q.l.Remove(q.l.Front())
}
