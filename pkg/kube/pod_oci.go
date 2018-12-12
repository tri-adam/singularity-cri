// Copyright (c) 2018 Sylabs, Inc. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kube

import (
	"fmt"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/runtime-tools/generate"
	"github.com/opencontainers/selinux/go-selinux/label"
	k8s "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

type podTranslator struct {
	pod *Pod
	g   generate.Generator
}

// translatePod translates Pod instance into OCI container specification.
func translatePod(pod *Pod) (*specs.Spec, error) {
	g := generate.Generator{
		Config: &specs.Spec{
			Version: specs.Version,
		},
	}
	t := podTranslator{
		g:   g,
		pod: pod,
	}
	return t.translate()
}

func (t *podTranslator) translate() (*specs.Spec, error) {
	t.g.SetRootPath(t.pod.rootfsPath())
	t.g.SetRootReadonly(false)

	t.g.SetHostname(t.pod.GetHostname())
	t.g.AddMount(specs.Mount{
		Destination: "/proc",
		Source:      "proc",
		Type:        "proc",
	})
	t.g.SetProcessCwd("/")
	t.g.SetProcessArgs([]string{"true"})

	for _, ns := range t.pod.namespaces {
		t.g.AddOrReplaceLinuxNamespace(string(ns.Type), ns.Path)
	}
	t.g.AddOrReplaceLinuxNamespace(string(specs.MountNamespace), "")

	for k, v := range t.pod.GetAnnotations() {
		t.g.AddAnnotation(k, v)
	}
	for k, v := range t.pod.GetLinux().GetSysctls() {
		t.g.AddLinuxSysctl(k, v)
	}

	security := t.pod.GetLinux().GetSecurityContext()
	if err := setupSELinux(&t.g, security.GetSelinuxOptions()); err != nil {
		return nil, err
	}

	t.g.SetupPrivileged(security.GetPrivileged())
	t.g.SetRootReadonly(security.GetReadonlyRootfs())
	t.g.SetProcessUID(uint32(security.GetRunAsUser().GetValue()))
	t.g.SetProcessGID(uint32(security.GetRunAsGroup().GetValue()))
	for _, gid := range security.GetSupplementalGroups() {
		t.g.AddProcessAdditionalGid(uint32(gid))
	}

	return t.g.Config, nil
}

func setupSELinux(g *generate.Generator, options *k8s.SELinuxOption) error {
	if options == nil {
		return nil
	}

	var labels []string
	if options.GetUser() != "" {
		labels = append(labels, "user:"+options.GetUser())
	}
	if options.GetRole() != "" {
		labels = append(labels, "role:"+options.GetRole())
	}
	if options.GetType() != "" {
		labels = append(labels, "type:"+options.GetType())
	}
	if options.GetLevel() != "" {
		labels = append(labels, "level:"+options.GetLevel())
	}
	processLabel, mountLabel, err := label.InitLabels(labels)
	if err != nil {
		return fmt.Errorf("could not init selinux labels: %v", err)
	}
	g.SetLinuxMountLabel(mountLabel)
	g.SetProcessSelinuxLabel(processLabel)
	return nil
}
