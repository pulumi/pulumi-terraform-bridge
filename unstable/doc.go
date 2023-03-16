// Copyright 2016-2023, Pulumi Corporation.
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

// A package that exposes an unstable interface across go module boundaries. This is
// necessary to get around granularity limitations of the `internal` path that go modules
// provides. Conceptually, the content of `unstable` is an implementation detail of this
// repository.
//
// It is not recommended to rely on exports from this package. We do not provide stability
// guarantees for this package. Unlike experimental `x` packages, APIs in `unstable` are
// not intended to be stabilized.
package unstable
