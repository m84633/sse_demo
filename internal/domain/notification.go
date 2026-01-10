package domain

import "errors"

const (
	NotificationTypeInfo    = "info"
	NotificationTypeWarning = "warning"
	NotificationTypeSystem  = "system"
)

var ErrInvalidNotificationType = errors.New("invalid notification type")

func IsValidNotificationType(value string) bool {
	switch value {
	case NotificationTypeInfo, NotificationTypeWarning, NotificationTypeSystem:
		return true
	default:
		return false
	}
}
