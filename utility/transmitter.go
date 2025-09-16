package utility

import (
	"form_pf_backend/ipc"
	"strconv"
)

func CreateSignal(uid, fid int) *ipc.Signal {
	return &ipc.Signal{
		UserId:   uid,
		FilingId: strconv.Itoa(fid),
	}

}
