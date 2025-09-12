package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

// TransferSignal takes up the signal and transfers them to the service via uds
func TransferSignal(signal *Signal) error {

	udsConn, err := net.Dial("unix", os.Getenv("SOCK_PATH"))
	if err != nil {
		return fmt.Errorf("creating conn: " + err.Error())
	}

	data, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("marshaling signal for transfer: " + err.Error())
	}

	_, err = udsConn.Write(data)
	if err != nil {
		return fmt.Errorf("writing to the socket: " + err.Error())
	}

	return nil
}
