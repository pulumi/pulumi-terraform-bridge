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
 * Configuration options for a workspace for use with the remote enhanced backend.
 */
export interface RemoteBackendWorkspaceConfig {
    /**
     * The full name of one remote workspace. When configured, only the default workspace
     * can be used. This option conflicts with prefix.
     */
    name?: pulumi.Input<string>;

    /**
     * A prefix used in the names of one or more remote workspaces, all of which can be used
     * with this configuration. If unset, only the default workspace can be used. This option
     * conflicts with name.
     */
    prefix?: pulumi.Input<string>;
}

/**
 * The configuration options for a Terraform Remote State stored in the remote enhanced
 * backend.
 */
export interface RemoteBackendRemoteStateReferenceArgs {
    /**
     * A constant describing the name of the Terraform backend, used as the discriminant
     * for the union of backend configurations.
     */
    backendType: "remote";

    /**
     * The remote backend hostname to which to connect. Defaults to `app.terraform.io`.
     */
    hostname?: pulumi.Input<string>;

    /**
     * The name of the organization containing the targeted workspace(s).
     */
    organization: pulumi.Input<string>;

    /**
     * The token used to authenticate with the remote backend.
     */
    token?: pulumi.Input<string>;

    /**
     * A block specifying which remote workspace(s) to use.
     */
    workspaces?: pulumi.Input<RemoteBackendWorkspaceConfig>;
}
