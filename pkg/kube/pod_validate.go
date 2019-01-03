package kube

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	"github.com/opencontainers/runtime-spec/specs-go"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

var (
	sysctlToNs = map[string]specs.LinuxNamespaceType{
		"kernel.shm": specs.IPCNamespace,
		"kernel.msg": specs.IPCNamespace,
		"kernel.sem": specs.IPCNamespace,
		"fs.mqueue.": specs.IPCNamespace,
		"net.":       specs.NetworkNamespace,
	}
)

const (
	defaultCgroup = "singularity-cri"
)

func (p *Pod) validateConfig() error {
	hasIPC := p.GetLinux().GetSecurityContext().GetNamespaceOptions().GetIpc() == k8s.NamespaceMode_POD
	hasNET := p.GetLinux().GetSecurityContext().GetNamespaceOptions().GetNetwork() == k8s.NamespaceMode_POD

	for sysctl := range p.GetLinux().GetSysctls() {
		for prefix, nsType := range sysctlToNs {
			if strings.HasPrefix(sysctl, prefix) {
				if (nsType == specs.IPCNamespace && !hasIPC) ||
					(nsType == specs.NetworkNamespace && !hasNET) {
					return fmt.Errorf("sysctl %s requires a separate %s namespace", sysctl, nsType)
				}
			}
		}
	}

	var err error
	hostname := p.GetHostname()
	if hostname == "" {
		hostname, err = os.Hostname()
		if err != nil {
			return fmt.Errorf("could not get default hostname: %v", err)
		}
		glog.Infof("setting pod hostname to default value %q", hostname)
		p.Hostname = hostname
	}

	cgroupsPath := p.GetLinux().GetCgroupParent()
	if cgroupsPath == "" {
		cgroupsPath = filepath.Join(defaultCgroup, p.ID())
		glog.Infof("setting pod cgroup parent to default value %q", cgroupsPath)
		if p.GetLinux() == nil {
			p.Linux = new(k8s.LinuxPodSandboxConfig)
		}
		p.Linux.CgroupParent = cgroupsPath
	}

	security := p.GetLinux().GetSecurityContext()
	if security != nil {
		scProfile, err := prepareSeccompPath(security.GetSeccompProfilePath())
		if err != nil {
			return fmt.Errorf("invalid Seccomp profile path: %v", err)
		}
		security.SeccompProfilePath = scProfile
	}

	return nil
}
