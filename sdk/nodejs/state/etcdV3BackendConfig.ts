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
 * The configuration options for a Terraform Remote State stored in the etcd v3 backend. Note
 * that there is a separate configuration class for state stored in the ectd v2 backend.
 */
export interface EtcdV3RemoteStateReferenceArgs {
    /**
     * A constant describing the name of the Terraform backend, used as the discriminant
     * for the union of backend configurations.
     */
    backendType: "etcdv3";

    /**
     * A list of the etcd endpoints.
     */
    endpoints: pulumi.Input<pulumi.Input<string>[]>;

    /**
     * The username with which to authenticate to etcd. Sourced from `ETCDV3_USERNAME` in
     * the environment, if unset.
     */
    username?: pulumi.Input<string>;

    /**
     * The username with which to authenticate to etcd. Sourced from `ETCDV3_PASSWORD` in
     * the environment, if unset.
     */
    password?: pulumi.Input<string>;

    /**
     * An optional prefix to be added to keys when storing state in etcd.
     */
    prefix?: pulumi.Input<string>;

    /**
     * Path to a PEM-encoded certificate authority bundle with which to verify certificates
     * of TLS-enabled etcd servers.
     */
    cacertPath?: pulumi.Input<string>;

    /**
     * Path to a PEM-encoded certificate to provide to etcd for client authentication.
     */
    certPath?: pulumi.Input<string>;

    /**
     * Path to a PEM-encoded key to use for client authentication.
     */
    keyPath?: pulumi.Input<string>;

    /**
     * The Terraform workspace from which to read state.
     */
    workspace?: pulumi.Input<string>;
}
