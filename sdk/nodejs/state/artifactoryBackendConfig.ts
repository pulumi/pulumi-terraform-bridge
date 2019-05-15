// Copyright 2016-2019, Pulumi Corporation.
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

import * as pulumi from "@pulumi/pulumi";

/**
 * The configuration options for a Terraform Remote State stored in the Artifactory backend.
 */
export interface ArtifactoryRemoteStateReferenceArgs {
    /**
     * A constant describing the name of the Terraform backend, used as the discriminant
     * for the union of backend configurations.
     */
    backendType: "artifactory";

    /**
     * The username with which to authenticate to Artifactory. Sourced from `ARTIFACTORY_USERNAME`
     * in the environment, if unset.
     */
    username?: pulumi.Input<string>;

    /**
     * The password with which to authenticate to Artifactory. Sourced from `ARTIFACTORY_PASSWORD`
     * in the environment, if unset.
     */
    password?: pulumi.Input<string>;

    /**
     * The Artifactory URL. Note that this is the base URL to artifactory, not the full repo and
     * subpath. However, it must include the path to the artifactory installation - likely this
     * will end in `/artifactory`. Sourced from `ARTIFACTORY_URL` in the environment, if unset.
     */
    url?: pulumi.Input<string>;

    /**
     * The repository name.
     */
    repo: pulumi.Input<string>;

    /**
     * Path within the repository.
     */
    subpath: pulumi.Input<string>;

    /**
     * The Terraform workspace from which to read state.
     */
    workspace?: pulumi.Input<string>;
}
