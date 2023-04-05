package key_value

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

// We won't test the requests.
// The requests are tested in the controllers
// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestListQueue struct {
	suite.Suite
	list *List
}

// Setup
// Setup checks the New() functions
// Setup checks ToMap() functions
func (suite *TestListQueue) SetupTest() {

	list := NewList()
	suite.list = list

	suite.Require().True(list.IsEmpty())
	suite.Require().False(list.IsFull())
	suite.Require().Zero(list.Len())
}

func (suite *TestListQueue) TestAddGet() {
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
	err := suite.list.Add(uint64(1), sample)
	suite.Require().NoError(err)
	suite.Require().EqualValues(suite.list.Len(), 1)
	suite.Require().False(suite.list.IsFull())
	suite.Require().False(suite.list.IsEmpty())

	// the value type are not matching
	// therefore it should fail
	invalid_sample := InvalidItem{param_1: "hello", param_2: uint64(0)}
	err = suite.list.Add(uint64(2), invalid_sample)
	suite.Require().Error(err)
	suite.Require().EqualValues(suite.list.Len(), 1)

	// invalid type
	// already addded by value, now pointer type is not valid
	err = suite.list.Add(uint64(3), &sample)
	suite.Require().Error(err)

	// invalid key type
	err = suite.list.Add(5, sample)
	suite.Require().Error(err)

	// key value already exists
	err = suite.list.Add(uint64(1), sample)
	suite.Require().Error(err)

	// key can not be a pointer
	key := uint64(6)
	err = suite.list.Add(&key, sample)
	suite.Require().Error(err)

	// get the data
	list := suite.list.List()
	list_sample := list[uint64(1)].(Item)
	suite.Require().EqualValues(sample, list_sample)

	// should be successful
	returned_sample, err := suite.list.Get(uint64(1))
	suite.Require().NoError(err)
	suite.Require().EqualValues(sample, returned_sample)

	// should fail since key doesn't exist
	_, err = suite.list.Get(uint64(10))
	suite.Require().Error(err)

	// should fail since key type is invalid
	_, err = suite.list.Get(1)
	suite.Require().Error(err)

	// should fail to get data from empty list

}

func (suite *TestListQueue) TestListLimit() {
	new_list := NewList()

	// index till QUEUE_LENGTH - 2
	for i := 0; i < LIST_LENGTH; i++ {
		err := new_list.Add(i, i*2)
		suite.Require().NoError(err)
	}

	suite.Require().True(new_list.IsFull())
	suite.Require().Equal(new_list.Len(), LIST_LENGTH)

	// can not add when the new list is full
	err := new_list.Add(LIST_LENGTH, LIST_LENGTH*2)
	suite.Require().Error(err)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestList(t *testing.T) {
	suite.Run(t, new(TestListQueue))
}
