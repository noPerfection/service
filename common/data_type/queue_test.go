package data_type

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestQueueSuite struct {
	suite.Suite
	key *Queue
}

// Setup
// Setup checks the New() functions
// Setup checks ToMap() functions
func (suite *TestQueueSuite) SetupTest() {

	queue := NewQueue()
	suite.key = queue

	suite.Require().True(queue.IsEmpty())
	suite.Require().False(queue.IsFull())
	suite.Require().Nil(queue.First())
	suite.Require().Zero(queue.Len())
}

func (suite *TestQueueSuite) TestPushPull() {
	type Item struct {
		param_1 string
		param_2 uint64
	}
	// This type of data can not be added if the first
	// element was added
	type InvalidItem struct {
		param_1 string
		param_2 uint64
	}
	sample := Item{param_1: "hello", param_2: uint64(0)}
	suite.key.Push(&sample)
	suite.Require().EqualValues(suite.key.Len(), 1)
	suite.Require().False(suite.key.IsFull())
	suite.Require().False(suite.key.IsEmpty())

	// silently skip adding of the element of another type
	invalid_sample := InvalidItem{param_1: "hello", param_2: uint64(0)}
	suite.key.Push(invalid_sample)
	suite.Require().EqualValues(suite.key.Len(), 1)

	// index till QUEUE_LENGTH - 2
	for i := 1; i <= 8; i++ {
		sample := Item{param_1: "hello", param_2: uint64(i)}
		suite.key.Push(&sample)
		suite.Require().EqualValues(suite.key.Len(), i+1)
		suite.Require().False(suite.key.IsFull())
		suite.Require().False(suite.key.IsEmpty())
	}

	// add the last element so the length should be equal to QUEUE_LENGTH
	sample = Item{param_1: "hello", param_2: uint64(0)}
	suite.key.Push(&sample)
	suite.Require().EqualValues(suite.key.Len(), 10)
	suite.Require().True(suite.key.IsFull())
	suite.Require().False(suite.key.IsEmpty())

	// should not add more element if the queue is full
	sample = Item{param_1: "hello", param_2: uint64(11)}
	suite.key.Push(&sample)
	suite.Require().EqualValues(suite.key.Len(), 10)

	// index till QUEUE_LENGTH - 1
	// pops up the first element
	for i := 10; i > 1; i-- {
		first := suite.key.First()
		suite.Require().NotNil(first)
		elem := suite.key.Pop()
		suite.Require().NotNil(elem)
		suite.Require().EqualValues(elem, first)

		suite.Require().EqualValues(suite.key.Len(), i-1)
		suite.Require().False(suite.key.IsFull())
		suite.Require().False(suite.key.IsEmpty(), "element", i)
	}

	// try to get the last element
	// after that the queue should be empty
	elem := suite.key.Pop()
	suite.Require().NotNil(elem)
	suite.Require().EqualValues(suite.key.Len(), 0)
	suite.Require().False(suite.key.IsFull())
	suite.Require().True(suite.key.IsEmpty())

	// try to get another element, it should be empty
	elem = suite.key.Pop()
	suite.Require().Nil(elem)
	suite.Require().True(suite.key.IsEmpty())

	// Once the data is cleared let's try to add more elements
	// index till QUEUE_LENGTH
	// pops up the first element
	for i := 0; i < 10; i++ {
		sample := Item{param_1: "hello", param_2: uint64(i)}
		suite.key.Push(&sample)
		suite.Require().EqualValues(suite.key.Len(), i+1)
		suite.Require().False(suite.key.IsEmpty())
	}

	suite.Require().True(suite.key.IsFull())
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestQueue(t *testing.T) {
	suite.Run(t, new(TestQueueSuite))
}
