package model

import "github.com/QuantumNous/new-api/common"

func init() {
	common.SetEmailLogRecorder(RecordEmailLog)
}
