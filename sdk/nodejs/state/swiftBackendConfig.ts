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
 * The configuration options for a Terraform Remote State stored in the Swift backend.
 */
export interface SwiftRemoteStateReferenceArgs {
    /**
     * A constant describing the name of the Terraform backend, used as the discriminant
     * for the union of backend configurations.
     */
    backendType: "swift";

    /**
     * The Identity authentication URL. Sourced from `OS_AUTH_URL` in the environment, if unset.
     */
    authUrl: pulumi.Input<string>;

    /**
     * The name of the container in which the Terraform state file is stored.
     */
    container: pulumi.Input<string>;

    /**
     * The username with which to log in. Sourced from `OS_USERNAME` in the environment, if
     * unset.
     */
    userName?: pulumi.Input<string>;

    /**
     * The user ID with which to log in. Sourced from `OS_USER_ID` in the environment, if
     * unset.
     */
    userId?: pulumi.Input<string>;

    /**
     * The password with which to log in. Sourced from `OS_PASSWORD` in the environment,
     * if unset.
     */
    password?: pulumi.Input<string>;

    /**
     * Access token with which to log in in stead of a username and password. Sourced from
     * `OS_AUTH_TOKEN` in the environment, if unset.
     */
    token?: pulumi.Input<string>;

    /**
     * The region in which the state file is stored. Sourced from `OS_REGION_NAME`, if
     * unset.
     */
    regionName: pulumi.Input<string>;

    /**
     * The ID of the tenant (for identity v2) or project (identity v3) which which to log in.
     * Sourced from `OS_TENANT_ID` or `OS_PROJECT_ID` in the environment, if unset.
     */
    tenantId?: pulumi.Input<string>;

    /**
     * The name of the tenant (for identity v2) or project (identity v3) which which to log in.
     * Sourced from `OS_TENANT_NAME` or `OS_PROJECT_NAME` in the environment, if unset.
     */
    tenantName?: pulumi.Input<string>;

    /**
     * The ID of the domain to scope the log in to (identity v3). Sourced from `OS_USER_DOMAIN_ID`,
     * `OS_PROJECT_DOMAIN_ID` or `OS_DOMAIN_ID` in the environment, if unset.
     */
    domainId?: pulumi.Input<string>;

    /**
     * The name of the domain to scope the log in to (identity v3). Sourced from
     * `OS_USER_DOMAIN_NAME`, `OS_PROJECT_DOMAIN_NAME` or `OS_DOMAIN_NAME` in the environment,
     * if unset.
     */
    domainName?: pulumi.Input<string>;

    /**
     * Whether to disable verification of the server TLS certificate. Sourced from
     * `OS_INSECURE` in the environment, if unset.
     */
    insecure?: pulumi.Input<boolean>;

    /**
     * A path to a CA root certificate for verifying the server TLS certificate. Sourced from
     * `OS_CACERT` in the environment, if unset.
     */
    cacertFile?: pulumi.Input<string>;

    /**
     * A path to a client certificate for TLS client authentication. Sourced from `OS_CERT`
     * in the environment, if unset.
     */
    cert?: pulumi.Input<string>;

    /**
     * A path to the private key corresponding to the client certificate for TLS client
     * authentication. Sourced from `OS_KEY` in the environment, if unset.
     */
    key?: pulumi.Input<string>;
}
