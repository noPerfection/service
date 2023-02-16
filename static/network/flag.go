// The flag.go keeps the functions to detect the network types
package network

const (
	ALL        int8 = 0 // any blockchain
	WITH_VM    int8 = 1 // with EVM
	WITHOUT_VM int8 = 2 // without EVM, it's an L2
)

// Whether the given flag is valid Network Flag or not.
func IsValidFlag(flag int8) bool {
	return flag == WITH_VM || flag == WITHOUT_VM || flag == ALL
}
