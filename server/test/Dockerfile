# A throwaway "empty VPS" for testing server/bcserver.sh: Ubuntu 24.04 booting
# systemd, reachable over SSH. Deliberately close to a fresh cloud image:
# no curl (bcserver must install it), ufw available (run.ps1 enables it so the
# script has to open its port), and a sudo user besides root.
FROM ubuntu:24.04

ENV container=docker DEBIAN_FRONTEND=noninteractive

RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      systemd systemd-sysv dbus openssh-server sudo iproute2 ufw iptables kmod \
 && rm -rf /var/lib/apt/lists/* \
 && systemctl enable ssh \
 && (id ubuntu >/dev/null 2>&1 || useradd -m -s /bin/bash ubuntu) \
 && echo 'ubuntu ALL=(ALL) NOPASSWD:ALL' > /etc/sudoers.d/ubuntu \
 && chmod 0440 /etc/sudoers.d/ubuntu \
 && mkdir -p /root/.ssh /home/ubuntu/.ssh \
 && chmod 700 /root/.ssh /home/ubuntu/.ssh \
 && chown ubuntu:ubuntu /home/ubuntu/.ssh

STOPSIGNAL SIGRTMIN+3
CMD ["/sbin/init"]
