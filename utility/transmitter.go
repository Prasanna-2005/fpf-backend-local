package utility

import (
	"form_pf_backend/ipc"
	"time"
)

func CreateSignal(uid, fid, series string, submissionTimestamp time.Time) *ipc.Signal {
	normalized, _ := time.Parse("2006-01-02 15:04:05", submissionTimestamp.Format("2006-01-02 15:04:05"))
	return &ipc.Signal{
		UserId:    uid,
		FilingId:  fid,
		Series:    series,
		Timestamp: normalized,
	}

}
