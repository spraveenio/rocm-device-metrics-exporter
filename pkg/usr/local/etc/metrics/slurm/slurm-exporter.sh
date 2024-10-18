#!/bin/bash

#
#Copyright (c) Advanced Micro Devices, Inc. All rights reserved.

#Licensed under the Apache License, Version 2.0 (the \"License\");
#you may not use this file except in compliance with the License.
#You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
#Unless required by applicable law or agreed to in writing, software
#distributed under the License is distributed on an \"AS IS\" BASIS,
#WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#See the License for the specific language governing permissions and
#limitations under the License.
#

SOCK="/var/run/slurm/slurm.sock"
MSG=$(cat << EOF
    {
    "SLURM_JOB_UID": "${SLURM_JOB_UID}",
    "SLURM_UID": "${SLURM_UID}",
    "SLURM_JOBID": "${SLURM_JOBID}",
    "SLURM_JOB_GPUS": "${SLURM_JOB_GPUS}",
    "CUDA_VISIBLE_DEVICES": "${CUDA_VISIBLE_DEVICES}",
    "SLURM_SCRIPT_CONTEXT": "${SLURM_SCRIPT_CONTEXT}"
   }
EOF
)
[ -S ${SOCK} ] && echo ${MSG} | nc -UN -w 1 ${SOCK} || true
