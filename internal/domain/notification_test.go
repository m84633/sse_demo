package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsValidNotificationType(t *testing.T) {
	t.Run("valid types", func(t *testing.T) {
		valid := []string{
			NotificationTypeInfo,
			NotificationTypeWarning,
			NotificationTypeSystem,
		}
		for _, v := range valid {
			require.True(t, IsValidNotificationType(v), "expected valid type: %s", v)
		}
	})

	t.Run("invalid types", func(t *testing.T) {
		invalid := []string{"", "infoo", "systemx", "warning1"}
		for _, v := range invalid {
			require.False(t, IsValidNotificationType(v), "expected invalid type: %s", v)
		}
	})
}
