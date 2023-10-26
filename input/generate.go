//go:build generate
// +build generate

// NOTE(negz): See the below link for details on what is happening here.
// https://github.com/golang/go/wiki/Modules#how-can-i-track-tool-dependencies-for-a-module

// Remove existing and generate new input manifests
//go:generate rm -rf ../package/input/
//go:generate go run -tags generate sigs.k8s.io/controller-tools/cmd/controller-gen paths=./v1beta1 object crd:crdVersions=v1 output:artifacts:config=../package/input

package input

import (
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen" //nolint:typecheck
)
