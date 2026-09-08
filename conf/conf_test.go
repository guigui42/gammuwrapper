package conf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigValidateTimeouts(t *testing.T) {
	valid := Config{
		GammuConf:                     defaultGammuConf,
		Port:                          defaultPort,
		SMSQueueMaxSize:               defaultSMSQueueMaxSize,
		GammuSendTimeoutSeconds:       defaultGammuSendTimeoutSeconds,
		GammuDiagnosticTimeoutSeconds: defaultGammuDiagnosticTimeoutSeconds,
	}
	require.NoError(t, valid.Validate())

	tests := []struct {
		name      string
		configure func(*Config)
		expected  string
	}{
		{
			name: "zero send timeout",
			configure: func(config *Config) {
				config.GammuSendTimeoutSeconds = 0
			},
			expected: "GAMMUSENDTIMEOUTSECONDS must be positive",
		},
		{
			name: "negative send timeout",
			configure: func(config *Config) {
				config.GammuSendTimeoutSeconds = -1
			},
			expected: "GAMMUSENDTIMEOUTSECONDS must be positive",
		},
		{
			name: "zero diagnostic timeout",
			configure: func(config *Config) {
				config.GammuDiagnosticTimeoutSeconds = 0
			},
			expected: "GAMMUDIAGNOSTICTIMEOUTSECONDS must be positive",
		},
		{
			name: "negative diagnostic timeout",
			configure: func(config *Config) {
				config.GammuDiagnosticTimeoutSeconds = -1
			},
			expected: "GAMMUDIAGNOSTICTIMEOUTSECONDS must be positive",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := valid
			test.configure(&config)
			require.EqualError(t, config.Validate(), test.expected)
		})
	}
}

func TestConfigDefaultsPreserveExistingSettings(t *testing.T) {
	require.Equal(t, "/etc/gammu-smsdrc", defaultGammuConf)
	require.Equal(t, 8083, defaultPort)
	require.Equal(t, 10, defaultSMSQueueMaxSize)
	require.Equal(t, 45, defaultGammuSendTimeoutSeconds)
	require.Equal(t, 10, defaultGammuDiagnosticTimeoutSeconds)
}
