ARG BASE_IMAGE=docker.io/ubuntu:22.04
FROM ${BASE_IMAGE}

# Install necessary packages

RUN apt-get update && \
    apt-get install wget gpg ca-certificates -y --no-install-recommends && \
    apt install "linux-headers-$(uname -r)" "linux-modules-extra-$(uname -r)" -y && \
    apt-get clean

RUN mkdir --parents --mode=0755 /etc/apt/keyrings && \
    wget https://repo.radeon.com/rocm/rocm.gpg.key -O - | gpg --dearmor \
    | tee /etc/apt/keyrings/rocm.gpg > /dev/null && \
    echo "deb [arch=amd64 signed-by=/etc/apt/keyrings/rocm.gpg] https://repo.radeon.com/rocm/apt/6.3 jammy main" \
    | tee --append /etc/apt/sources.list.d/rocm.list && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
    libpci3 libpci-dev doxygen unzip cmake git libyaml-cpp-dev rocblas rocm-smi-lib rocm-validation-suite hiprand rocrand && \
    apt-get clean

ADD testrunner /testrunner
RUN chmod +x /testrunner

ENTRYPOINT ["/testrunner"]