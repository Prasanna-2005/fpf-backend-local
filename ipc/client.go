package ipc

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
)

// TransferSignal takes up the signal and transfers them to the service via uds
func TransferSignal(signal *Signal) error {

	udsConn, err := net.Dial("unix", os.Getenv("STORAGE_PATH")+os.Getenv("SOCK_NAME"))
	if err != nil {
		return fmt.Errorf("creating conn: %w", err)
	}

	data, err := json.Marshal(signal)
	if err != nil {
		return fmt.Errorf("marshaling signal for transfer: %w", err)
	}

	_, err = udsConn.Write(data)
	if err != nil {
		return fmt.Errorf("writing to the socket: %w", err)
	}

	return nil
}
