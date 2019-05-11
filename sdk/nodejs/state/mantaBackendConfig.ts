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
 * The configuration options for a Terraform Remote State stored in the Manta backend.
 */
export interface MantaRemoteStateReferenceArgs {
    /**
     * A constant describing the name of the Terraform backend, used as the discriminant
     * for the union of backend configurations.
     */
    backendType: "manta";

    /**
     * The name of the Manta account. Sourced from `SDC_ACCOUNT` or `_ACCOUNT` in the
     * environment, if unset.
     */
    account: pulumi.Input<string>;

    /**
     * The username of the Manta account with which to authenticate.
     */
    user?: pulumi.Input<string>;

    /**
     * The Manta API Endpoint. Sourced from `MANTA_URL` in the environment, if unset.
     * Defaults to `https://us-east.manta.joyent.com`.
     */
    url?: pulumi.Input<string>;

    /**
     * The private key material corresponding with the SSH key whose fingerprint is
     * specified in keyId. Sourced from `SDC_KEY_MATERIAL` or `TRITON_KEY_MATERIAL`
     * in the environment, if unset. If no value is specified, the local SSH agent
     * is used for signing requests.
     */
    keyMaterial?: pulumi.Input<string>;

    /**
     * The fingerprint of the public key matching the key material specified in
     * keyMaterial, or in the local SSH agent.
     */
    keyId: pulumi.Input<string>;

    /**
     * The path relative to your private storage directory (`/$MANTA_USER/stor`)
     * where the state file will be stored.
     */
    path: pulumi.Input<string>;

    /**
     * Skip verifying the TLS certificate presented by the Manta endpoint. This can
     * be useful for installations which do not have a certificate signed by a trusted
     * root CA. Defaults to false.
     */
    insecureSkipTlsVerify: pulumi.Input<boolean>;

    /**
     * The Terraform workspace from which to read state.
     */
    workspace?: pulumi.Input<string>;
}
