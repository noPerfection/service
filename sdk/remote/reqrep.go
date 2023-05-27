// This package establishes a short living connection to the SDS Server
package remote

import (
	"log"

	"github.com/blocklords/sds/service/communication/message"

	zmq "github.com/pebbe/zmq4"
)

// Request Reply pattern. In the web it's called RPC.
func ReqReply(host string, req message.Request, logger log.Logger) message.Reply {
	socket, sockErr := zmq.NewSocket(zmq.REQ)
	if sockErr != nil {
		return message.Fail(`remote: failed to create a socket: ` + sockErr.Error())
	}
	defer socket.Close()

	conErr := socket.Connect("tcp://" + host)
	if conErr != nil {
		socket.Close()
		return message.Fail(`remote: failed to connect to the SDS Gateway: ` + conErr.Error())
	}

	if _, err := socket.SendMessage(req.ToString()); err != nil {
		socket.Close()
		return message.Fail("smartcontract_read command sending error: " + err.Error())
	}

	replyMsg, err := socket.RecvMessage(0)
	if err != nil {
		socket.Close()
		return message.Fail("Failed to read message: " + err.Error())
	}

	reply, err := message.ParseReply(replyMsg)
	if err != nil {
		socket.Close()
		return message.Fail("parse error of SDS Gateway reply: " + err.Error())
	}

	socket.Close()

	return reply
}
