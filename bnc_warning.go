package bncclient

func newWaring(message string) error {
	return &warning{message: message}
}

type warning struct {
	message string
}

func (w *warning) Error() string { // warning structure implementing "error" interface
	return (*w).message
}