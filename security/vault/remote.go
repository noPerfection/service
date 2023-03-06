// Keep the credentials in a vault
package vault

import (
	"errors"
	"fmt"

	"github.com/blocklords/sds/app/remote/message"

	zmq "github.com/pebbe/zmq4"
)

// Get the string value from the vault
func GetStringFromVault(bucket string, key string) (string, error) {
	// Socket to talk to clients
	socket, err := zmq.NewSocket(zmq.REQ)
	if err != nil {
		return "", err
	}

	if err := socket.Connect("inproc://sds_vault"); err != nil {
		return "", fmt.Errorf("error to bind socket for: " + err.Error())
	}

	request := message.Request{
		Command: "GetString",
		Parameters: map[string]interface{}{
			"bucket": bucket,
			"key":    key,
		},
	}

	request_string, _ := request.ToString()

	//  We send a request, then we work to get a reply
	socket.SendMessage(request_string)

	// Wait for reply.
	r, _ := socket.RecvMessage(0)

	reply, _ := message.ParseReply(r)
	if !reply.IsOK() {
		return "", errors.New(reply.Message)
	}

	value, _ := reply.Parameters.GetString("value")

	return value, nil
}
