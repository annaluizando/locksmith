package notifiers

import secrets "token-toolkit/jwt-rotation"

// broadcasts notifications to multiple notifiers.
type MultiNotifier struct {
	notifiers []secrets.Notifier
}

// creates a new MultiNotifier.
func NewMultiNotifier(notifiers ...secrets.Notifier) *MultiNotifier {
	return &MultiNotifier{notifiers: notifiers}
}

// sends a rotation notification to all configured notifiers.
func (m *MultiNotifier) NotifyRotation(secret *secrets.Secret) {
	for _, n := range m.notifiers {
		if n != nil {
			n.NotifyRotation(secret)
		}
	}
}

// sends an error notification to all configured notifiers.
func (m *MultiNotifier) NotifyError(err error) {
	for _, n := range m.notifiers {
		if n != nil {
			n.NotifyError(err)
		}
	}
}
