package key_value

import (
	"fmt"
	"reflect"

	"github.com/blocklords/sds/common/data_type"
)

type List struct {
	l          map[interface{}]interface{}
	length     int
	key_type   reflect.Type
	value_type reflect.Type
}

// max amount of data that this list could keep
const LIST_LENGTH = 1_000_000

// List of the elements that could contain
// maximum LIST_LENGTH amount of elements.
//
// The queue has a function that returns the first element
// by taking it out from the list.
//
// The added elements attached after the last element.
func NewList() *List {
	return &List{
		key_type:   nil,
		value_type: nil,
		length:     0,
		l:          map[interface{}]interface{}{},
	}
}

func (q *List) Len() int {
	return q.length
}

func (q *List) IsEmpty() bool {
	return q.length == 0
}

func (q *List) IsFull() bool {
	return q.length == LIST_LENGTH
}

func (q *List) List() map[interface{}]interface{} {
	return q.l
}

// Adds the element to the queue.
// If the element type is not the same as
// the expected type, then
// It will silently drop it.
// Silently drop if the queue is full
func (q *List) Add(key interface{}, value interface{}) error {
	if q.IsFull() {
		return fmt.Errorf("list is already full")
	}
	if data_type.IsNil(key) {
		return fmt.Errorf("the key parameter is nil")
	}
	if data_type.IsPointer(key) {
		return fmt.Errorf("the key was passed by the pointer")
	}
	if data_type.IsNil(value) {
		return fmt.Errorf("the value parameer is nil")
	}

	key_type := reflect.TypeOf(key)
	value_type := reflect.TypeOf(value)

	if q.key_type == nil {
		q.key_type = key_type
		q.value_type = value_type
	} else if _, ok := q.l[key]; ok {
		return fmt.Errorf("the element exists")
	}

	if key_type == q.key_type && value_type == q.value_type {
		q.l[key] = value
		q.length++
		return nil
	}

	return fmt.Errorf(
		"expected key type %T against %T and expected value type %T against %T",
		q.key_type,
		key_type,
		q.value_type,
		value_type,
	)
}

// Returns the element in the list to the value.
// The value should be passed by pointer
func (q *List) Get(key interface{}) (interface{}, error) {
	if data_type.IsNil(key) {
		return nil, fmt.Errorf("the parameter is nil")
	}
	if data_type.IsPointer(key) {
		return nil, fmt.Errorf("the key was passed by the pointer")
	}
	if q.IsEmpty() {
		return nil, fmt.Errorf("the list is empty")
	}

	key_type := reflect.TypeOf(key)
	if key_type != q.key_type {
		return nil, fmt.Errorf("the data mismatch: expected key type %T against %T", q.key_type, key_type)
	}

	value, ok := q.l[key]
	if !ok {
		return nil, fmt.Errorf("the element not found")
	}
	return value, nil
}
