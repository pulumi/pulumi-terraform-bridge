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
 * The configuration options for a Terraform Remote State stored in the etcd v2 backend. Note
 * that there is a separate configuration class for state stored in the ectd v3 backend.
 */
export interface EtcdV2RemoteStateReferenceArgs {
    /**
     * A constant describing the name of the Terraform backend, used as the discriminant
     * for the union of backend configurations.
     */
    backendType: "etcd";

    /**
     * The path at which to store the state.
     */
    path: pulumi.Input<string>;

    /**
     * A space-separated list of the etcd endpoints.
     */
    endpoints: pulumi.Input<string>;

    /**
     * The username with which to authenticate to etcd.
     */
    username?: pulumi.Input<string>;

    /**
     * The username with which to authenticate to etcd.
     */
    password?: pulumi.Input<string>;

    /**
     * The Terraform workspace from which to read state.
     */
    workspace?: pulumi.Input<string>;
}
