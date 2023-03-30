package account

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// Define the suite, and absorb the built-in basic suite
// functionality from testify - including a T() method which
// returns the current testing context
type TestAccountSuite struct {
	suite.Suite
	AccountFromDb        *Account
	AccountWithPublicKey *Account
	organization         string
}

// Make sure that Account is set to five
// before each test
func (suite *TestAccountSuite) SetupTest() {
	// The public key is derived from the following public key
	// private_key := "ndQu.hg=f#P+i+r<.^x-S:oNuEt(?obKd/zD+AjV"
	public_key := "t)fBitv:t=7zX=qzB/.0bOP->YIr[hsw{J*BBh[H"
	nonce := uint64(time.Now().UnixNano())
	organization := "test_org"

	suite.AccountFromDb = New(public_key, nonce, organization)
	suite.AccountWithPublicKey = NewFromPublicKey(public_key)
	suite.organization = organization
}

// All methods that begin with "Test" are run as tests within a
// suite.
func (suite *TestAccountSuite) TestAccountFromDatabase() {
	suite.Equal(suite.AccountFromDb.Organization, suite.organization)
	suite.NotZero(suite.AccountFromDb.NonceTimestamp)
}

func (suite *TestAccountSuite) TestAccountFromPublicKey() {
	suite.Empty(suite.AccountWithPublicKey.Organization)
	suite.Empty(suite.AccountWithPublicKey.NonceTimestamp)
	suite.NotEmpty(suite.AccountWithPublicKey)
}

// In order for 'go test' to run this suite, we need to create
// a normal test function and pass our suite to suite.Run
func TestAccount(t *testing.T) {
	suite.Run(t, new(TestAccountSuite))
}
