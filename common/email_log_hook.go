package common

var recordEmailLog = func(provider, receiver, subject, content, status string, durationMs int64, err error) {}

func SetEmailLogRecorder(recorder func(provider, receiver, subject, content, status string, durationMs int64, err error)) {
	if recorder == nil {
		recordEmailLog = func(provider, receiver, subject, content, status string, durationMs int64, err error) {}
		return
	}
	recordEmailLog = recorder
}
