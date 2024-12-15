ARG BASE_IMAGE=registry.access.redhat.com/ubi9/ubi:9.4
FROM ${BASE_IMAGE}

LABEL maintainer="AMD Inc"
LABEL OS="registry.access.redhat.com/ubi9/ubi:9.4"

RUN dnf install https://dl.fedoraproject.org/pub/epel/epel-release-latest-9.noarch.rpm -y && \
    crb enable && \
    echo -e "[amdgpu] \n\
name=amdgpu \n\
baseurl=https://repo.radeon.com/rocm/el9/6.3/main/ \n\
enabled=1 \n\
priority=50 \n\
gpgcheck=1 \n\
gpgkey=https://repo.radeon.com/rocm/rocm.gpg.key" > /etc/yum.repos.d/amdgpu.repo && \
    dnf clean all

RUN dnf install yaml-cpp rocm-smi-lib rocrand -y && \
    rm -rf /var/cache/yum && \
    rm -rf /var/cache/dnf && \
    dnf clean all

RUN dnf install rocm-validation-suite -y && \
    rm -rf /var/cache/yum && \
    rm -rf /var/cache/dnf && \
    dnf clean all

ADD amd-test-runner /amd-test-runner
RUN chmod +x /amd-test-runner

ENTRYPOINT ["/amd-test-runner"]
