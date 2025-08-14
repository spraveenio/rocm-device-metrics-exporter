/**
# Copyright (c) Advanced Micro Devices, Inc. All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the \"License\");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an \"AS IS\" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
**/

package utils

type ErrorCode int

const (
	ErrorNone ErrorCode = iota
	ErrorNotApplicable
	ErrorInvalidArgument
)

// ErrorCodeToString returns a string representation of the given ErrorCode.
func ErrorCodeToString(code ErrorCode) string {
	switch code {
	case ErrorNone:
		return "None"
	case ErrorNotApplicable:
		return "NotApplicable"
	case ErrorInvalidArgument:
		return "InvalidArgument"
	default:
		return "UnknownErrorCode"
	}
}

// NewError returns an error with the given ErrorCode and message.
func NewError(code ErrorCode, msg string) error {
	return &ErrorWithCode{
		Code:    code,
		Message: msg,
	}
}

// ErrorWithCode wraps an error code and message.
type ErrorWithCode struct {
	Code    ErrorCode
	Message string
}

func (e *ErrorWithCode) Error() string {
	return ErrorCodeToString(e.Code) + ": " + e.Message
}
