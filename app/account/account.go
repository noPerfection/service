// Handles the user's authentication
package account

// Requester to the SDS Service. It's either a developer or another SDS service.
type Account struct {
	PublicKey      string `json:"public_key"`      // Public Key for authentication.
	Organization   string `json:"organization"`    // Organization
	NonceTimestamp uint64 `json:"nonce_timestamp"` // Nonce since the last usage. Only acceptable for developers
}

type Accounts []*Account

// Creates the account from the public key
func NewFromPublicKey(public_key string) *Account {
	return &Account{
		PublicKey:      public_key,
		Organization:   "",
		NonceTimestamp: 0,
	}
}

// Creates a new Account for a developer.
func New(public_key string, nonce_timestamp uint64, organization string) *Account {
	return &Account{
		PublicKey:      public_key,
		NonceTimestamp: nonce_timestamp,
		Organization:   organization,
	}
}

///////////////////////////////////////////////////////////
//
// Group operations
//
///////////////////////////////////////////////////////////

func NewAccounts(new_accounts ...*Account) Accounts {
	accounts := make(Accounts, len(new_accounts))
	copy(accounts, new_accounts)

	return accounts
}

func (accounts Accounts) PublicKeys() []string {
	public_keys := make([]string, len(accounts))

	for i, account := range accounts {
		public_keys[i] = account.PublicKey
	}

	return public_keys
}

func (accounts Accounts) Add(new_accounts ...*Account) Accounts {
	for _, account := range new_accounts {
		accounts = append(accounts, account)
	}

	return accounts
}

func (accounts Accounts) Remove(new_accounts ...*Account) Accounts {
	for _, account := range new_accounts {
		for i := range accounts {
			if account.PublicKey == accounts[i].PublicKey {
				accounts = append(accounts[:i], accounts[i+1:]...)
				return accounts
			}
		}
	}

	return accounts
}
