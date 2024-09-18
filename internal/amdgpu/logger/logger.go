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

package logger

import (
	"log"
	"os"
	"sync"
)

var (
	Log     *log.Logger
	logpath = "/var/run/exporter.log"
	once    sync.Once
)

func initLogger() {
	outfile, _ := os.Create(logpath)
	Log = log.New(outfile, "", 0)
	Log.SetFlags(log.LstdFlags | log.Lshortfile)
}

func Init() {
	once.Do(initLogger)
}
