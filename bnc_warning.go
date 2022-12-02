package bncclient

type Warning interface {
	Error() string
	GetRetryAfterTimeMS() int64
}

func newWaring(retryAfter int64, message string) Warning {
	return warningSt{retryAfter: retryAfter, message: message}
}

type warningSt struct {
	retryAfter int64
	message    string
}

func (w warningSt) Error() string { // warning structure implementing "error" interface
	return w.message
}

func (w warningSt) GetRetryAfterTimeMS() int64 {
	return w.retryAfter
}
