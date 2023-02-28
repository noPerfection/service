package remote

import (
	"errors"
	"fmt"
	"time"

	"github.com/blocklords/gosds/app/remote/message"

	zmq "github.com/pebbe/zmq4"
)

// The Socket if its a Subscriber applies a filter to listen certain data from the Broadcaster.
func (socket *Socket) SetSubscribeFilter(topic string) error {
	socketType, err := socket.socket.GetType()
	if err != nil {
		return fmt.Errorf("zmq socket get type: %w", err)
	}
	if socketType != zmq.SUB {
		return errors.New("the socket is not a Broadcast. Can not call subscribe")
	}

	err = socket.socket.SetSubscribe(topic)
	if err != nil {
		return fmt.Errorf("zmq socket set subscribe: %w", err)
	}

	return nil
}

// Subscribe to the SDS Broadcast.
// The function is intended to be called as a gouritine.
//
// When a new message arrives, the method will send it to the channel.
//
// if time is out, it will send the timeout message.
func (socket *Socket) Subscribe(channel chan message.Reply, exit_channel chan int, time_out time.Duration) {
	socketType, err := socket.socket.GetType()
	if err != nil {
		channel <- message.Fail("failed to check the socket type. the socket error: " + err.Error())
		return
	}
	if socketType != zmq.SUB {
		channel <- message.Fail("the socket is not a Broadcast. Can not call subscribe")
		return
	}

	timer := time.AfterFunc(time_out, func() {
		channel <- message.Fail("timeout")
	})
	defer timer.Stop()

	for {
		select {
		case <-exit_channel:
			if !timer.Stop() {
				<-timer.C
			}
			fmt.Println("exit signal was received for subscriber")
			return
		default:
			msgRaw, err := socket.socket.RecvMessage(zmq.DONTWAIT)

			if err != nil {
				fmt.Println("receive message", err)
				time.Sleep(time.Millisecond * 200)
				continue
			}
			timer.Reset(time_out)

			broadcast, err := message.ParseBroadcast(msgRaw)
			if err != nil {
				channel <- message.Fail("Error when parsing message: " + err.Error())
				continue
			}

			channel <- broadcast.Reply
		}
	}
}
