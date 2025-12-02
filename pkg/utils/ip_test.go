package utils

import (
	"testing"
)

func TestValidateIPOrCIDR(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{
			name:    "Valid raw IPv4",
			input:   "183.137.36.138",
			wantErr: false,
		},
		{
			name:    "Valid raw IPv4 - localhost",
			input:   "127.0.0.1",
			wantErr: false,
		},
		{
			name:    "Valid IPv4 CIDR /24",
			input:   "192.168.1.0/24",
			wantErr: false,
		},
		{
			name:    "Valid IPv4 CIDR /22",
			input:   "10.0.0.0/22",
			wantErr: false,
		},
		{
			name:    "Valid IPv4 CIDR /32",
			input:   "1.2.3.4/32",
			wantErr: false,
		},
		{
			name:    "Invalid - CIDR prefix too small /21",
			input:   "10.0.0.0/21",
			wantErr: true,
		},
		{
			name:    "Invalid - CIDR prefix too small /16",
			input:   "192.168.0.0/16",
			wantErr: true,
		},
		{
			name:    "Valid raw IPv6",
			input:   "2001:db8::1",
			wantErr: false,
		},
		{
			name:    "Valid IPv6 CIDR /64",
			input:   "2001:db8::/64",
			wantErr: false,
		},
		{
			name:    "Invalid IP",
			input:   "invalid-ip",
			wantErr: true,
		},
		{
			name:    "Invalid CIDR - bad prefix",
			input:   "192.168.1.0/33",
			wantErr: true,
		},
		{
			name:    "Empty string",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateIPOrCIDR(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateIPOrCIDR() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
