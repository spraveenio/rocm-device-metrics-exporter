//
// Copyright(C) Advanced Micro Devices, Inc. All rights reserved.
//
// You may not use this software and documentation (if any) (collectively,
// the "Materials") except in compliance with the terms and conditions of
// the Software License Agreement included with the Materials or otherwise as
// set forth in writing and signed by you and an authorized signatory of AMD.
// If you do not have a copy of the Software License Agreement, contact your
// AMD representative for a copy.
//
// You agree that you will not reverse engineer or decompile the Materials,
// in whole or in part, except as allowed by applicable law.
//
// THE MATERIALS ARE DISTRIBUTED ON AN "AS IS" BASIS, WITHOUT WARRANTIES OR
// REPRESENTATIONS OF ANY KIND, EITHER EXPRESS OR IMPLIED.
//

package parserutil

import (
	"fmt"
	"strconv"
	"strings"
)

func RangeStrToIntIndices(b string) ([]int, error) {
	var indices []int
	numbers := strings.Split(b, ",")
	for _, numOrRange := range numbers {
		token := strings.Split(numOrRange, "-")
		tokenCount := len(token)
		if tokenCount > 2 {
			return indices, fmt.Errorf("range must be of format 'min-max', but found '%s'", numOrRange)
		} else if tokenCount == 1 {
			number, err := strconv.Atoi(token[0])
			if err != nil {
				return indices, err
			}
			indices = append(indices, number)
		} else {
			start, err := strconv.Atoi(token[0])
			if err != nil {
				return indices, err
			}
			end, err := strconv.Atoi(token[1])
			if err != nil {
				return indices, err
			}
			if start > end {
				return indices, fmt.Errorf("range must be of format 'min-max', but found '%s'", numOrRange)
			}

			// Add the range to the indices
			for i := start; i <= end; i++ {
				indices = append(indices, i)
			}
		}
	}
	return indices, nil
}
