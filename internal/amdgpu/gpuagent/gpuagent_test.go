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

package gpuagent

import (
	"testing"

	"gotest.tools/assert"
)

func TestGpuAgent(t *testing.T) {
	teardownSuite := setupTest(t)
	defer teardownSuite(t)

	ga := getNewAgent(t)
	defer ga.Close()
	t.Logf("gpuagent : %+v", ga)

	req, err := ga.GPUGet()
	assert.Assert(t, err == nil, "expecting nil response")

	t.Logf("req :%+v", req)

}
