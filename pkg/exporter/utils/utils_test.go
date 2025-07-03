/*
Copyright (c) Advanced Micro Devices, Inc. All rights reserved.

Licensed under the Apache License, Version 2.0 (the \"License\");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

     http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an \"AS IS\" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"math"
	"testing"
)

func TestGetPCIeBaseAddress(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Standard PCIe address with function",
			input:    "0000:03:00.0",
			expected: "0000:03:00",
		},
		{
			name:     "PCIe address with multi-digit function",
			input:    "0000:03:00.12",
			expected: "0000:03:00",
		},
		{
			name:     "Malformed address no dot",
			input:    "0000:03:00",
			expected: "0000:03:00",
		},
		{
			name:     "Empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "Only function",
			input:    ".0",
			expected: "",
		},
		{
			name:     "Multiple dots",
			input:    "0000:03:00.0.1",
			expected: "0000:03:00.0.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetPCIeBaseAddress(tt.input)
			if got != tt.expected {
				t.Errorf("GetPCIeBaseAddress(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsValueApplicable(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected bool
	}{
		{
			name:     "uint64 - na",
			input:    0xFFFFFFFFFFFFFFFF,
			expected: false,
		},
		{
			name:     "uint32 - na",
			input:    4294967295,
			expected: false,
		},
		{
			name:     "uint32 - na",
			input:    0xFFFFFFFF,
			expected: false,
		},
		{
			name:     "uint16 - na",
			input:    65535,
			expected: false,
		},
		{
			name:     "uint16 - na",
			input:    0xFFFF,
			expected: false,
		},
		{
			name:     "uint8 - na",
			input:    255,
			expected: false,
		},
		{
			name:     "uint8 - na",
			input:    0xFF,
			expected: false,
		},
		{
			name:     "uint32 - valid",
			input:    200,
			expected: true,
		},
		{
			name:     "uint16 - valid",
			input:    100,
			expected: true,
		},
		{
			name:     "uint8 - valid",
			input:    50,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsValueApplicable(tt.input)
			if got != tt.expected {
				t.Errorf("IsApplicable(%v) = %v; want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name     string
		input    uint64
		expected float64
	}{
		{
			name:     "uint64 - na",
			input:    0xFFFFFFFFFFFFFFFF,
			expected: 0,
		},
		{
			name:     "uint32 - na",
			input:    0xFFFFFFFF,
			expected: 0,
		},
		{
			name:     "uint16 - na",
			input:    0xFFFF,
			expected: 0,
		},
		{
			name:     "uint8 - na",
			input:    0xFF,
			expected: 0,
		},
		{
			name:     "uint32 - valid",
			input:    200,
			expected: 200,
		},
		{
			name:     "uint16 - valid",
			input:    100,
			expected: 100,
		},
		{
			name:     "uint8 - valid",
			input:    50,
			expected: 50,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeUint64(tt.input)
			if got != tt.expected {
				t.Errorf("IsApplicable(%v) = %v; want %v", tt.input, got, tt.expected)
			}
		})
	}
}
