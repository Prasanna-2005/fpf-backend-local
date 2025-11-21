package utility

import (
	"form_pf_backend/ipc"
)

func CreateSignal(uid, fid, series string) *ipc.Signal {
	return &ipc.Signal{
		UserId:   uid,
		FilingId: fid,
		Series:   series,
	}

}
