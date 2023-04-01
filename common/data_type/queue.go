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
func NewQueue(element_type reflect.Type) *Queue {
	return &Queue{
		element_type: element_type,
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

func (q *Queue) Push(item interface{}) {
	q.l.PushBack(item)
}

// Returns the first element without removing it from the queue
func (q *Queue) First() interface{} {
	return q.l.Front().Value
}

func (q *Queue) Pop() interface{} {
	return q.l.Remove(q.l.Front())
}
