// Copyright 2023 DoltHub, Inc.
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

package main

import (
	"strconv"
	"strings"
)

func VersionSupportsTransitionToStandby(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	intparts := make([]int, 3)
	for i, p := range parts {
		var err error
		intparts[i], err = strconv.Atoi(p)
		if err != nil {
			return false
		}
	}
	if intparts[0] < 1 {
		return false
	}
	if intparts[0] > 1 {
		return true
	}
	if intparts[1] <= 5 {
		return false
	}
	return true
}
