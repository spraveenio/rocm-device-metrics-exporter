#!/usr/bin/env bash
dir=/usr/src/github.com/ROCm/device-metrics-exporter/libamdsmi
patchdir=/usr/src/github.com/ROCm/device-metrics-exporter/patch/amdsmi/
exporteroutdir=$dir/build/exporterout

cd /usr/src/github.com/ROCm/device-metrics-exporter/libamdsmi
git config --global --add safe.directory $dir
if [ -z $BRANCH ]; then
    echo "branch set to $BRANCH"
    git checkout $BRANCH || true
fi
if [ -z $COMMIT ]; then
    echo "commit set to $COMMIT"
    git reset --hard $COMMIT
fi

if [ -d "$patchdir" ]; then
    echo "Applying patches from $patchdir"
    for patch in "$patchdir"/*.patch; do
        if [ -f "$patch" ]; then
            echo "Applying patch: $patch"
            git apply --check "$patch" 2>/dev/null && git apply "$patch" || echo "Patch already applied or not applicable, skipping: $patch"
            if [ $? -ne 0 ]; then
                echo "Failed to apply patch: $patch"
                exit 1
            fi
        fi
    done
else
    echo "No patch directory found at $patchdir, skipping patches"
fi

rm -rf build 2>&1 || true
mkdir build
cd build
cmake -DCMAKE_C_COMPILER=gcc -DCMAKE_CXX_COMPILER=g++ -DENABLE_ESMI_LIB=OFF ..

make -j $(nproc)
make install

if [ $? -ne 0 ]; then
    echo "Build error"
    exit 1
fi

# come back to root directory
cd $dir

# find which os to look for artifacts in specific directories
os=`cat /etc/os-release | grep ^ID= | cut -d'=' -f 2`

#copy all required files for exporter to exporteroutput directory
mkdir -p $exporteroutdir || true


#ubuntu
if [ $os == "ubuntu" ]; then
    echo "Copying UBUNTU library..."
    cp -vr $dir/build/src/libamd_smi.so*  $exporteroutdir/
    cp -vr /opt/rocm/include/amd_smi/amdsmi.h $exporteroutdir/
    cp -vr /usr/lib/x86_64-linux-gnu/libdrm_amdgpu.so* $exporteroutdir/
    cp -vr /usr/lib/x86_64-linux-gnu/libdrm.so* $exporteroutdir/
#rhel, azurelinux
else
    echo "Copying $os library..."
    cp -vr $dir/build/src/libamd_smi.so*  $exporteroutdir/
    cp -vr /opt/rocm/include/amd_smi/amdsmi.h $exporteroutdir/
    cp -vr /usr/lib64/libdrm_amdgpu.so* $exporteroutdir/
    cp -vr /usr/lib64/libdrm.so* $exporteroutdir/
fi

ls -lart $exporteroutdir

echo "Successfully Build AMI SMI lib $os"
exit 0
