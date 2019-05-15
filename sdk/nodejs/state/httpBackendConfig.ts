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
 * The configuration options for a Terraform Remote State stored in the HTTP backend.
 */
export interface HttpRemoteStateReferenceArgs {
    /**
     * A constant describing the name of the Terraform backend, used as the discriminant
     * for the union of backend configurations.
     */
    backendType: "http";

    /**
     * The address of the HTTP endpoint.
     */
    address: pulumi.Input<string>;

    /**
     * HTTP method to use when updating state. Defaults to `POST`.
     */
    updateMethod?: pulumi.Input<string>;

    /**
     * The address of the lock REST endpoint. Not setting a value disables locking.
     */
    lockAddress?: pulumi.Input<string>;

    /**
     * The HTTP method to use when locking. Defaults to `LOCK`.
     */
    lockMethod?: pulumi.Input<string>;

    /**
     * The address of the unlock REST endpoint. Not setting a value disables locking.
     */
    unlockAddress?: pulumi.Input<string>;

    /**
     * The HTTP method to use when unlocking. Defaults to `UNLOCK`.
     */
    unlockMethod?: pulumi.Input<string>;

    /**
     * The username used for HTTP basic authentication.
     */
    username?: pulumi.Input<string>;

    /**
     * The password used for HTTP basic authentication.
     */
    password?: pulumi.Input<string>;

    /**
     * Whether to skip TLS verification. Defaults to false.
     */
    skipCertVerification?: pulumi.Input<boolean>;

    /**
     * The Terraform workspace from which to read state.
     */
    workspace?: pulumi.Input<string>;
}
