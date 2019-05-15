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
 * The configuration options for a Terraform Remote State stored in the Consul backend.
 */
export interface ConsulRemoteStateReferenceArgs {
    /**
     * A constant describing the name of the Terraform backend, used as the discriminant
     * for the union of backend configurations.
     */
    backendType: "consul";

    /**
     * Path in the Consul KV store.
     */
    path: pulumi.Input<string>;

    /**
     * Consul Access Token. Sourced from `CONSUL_HTTP_TOKEN` in the environment, if unset.
     */
    accessToken: pulumi.Input<string>;

    /**
     * DNS name and port of the Consul HTTP endpoint specified in the format `dnsname:port`. Defaults
     * to the local agent HTTP listener.
     */
    address?: pulumi.Input<string>;

    /**
     * Specifies which protocol to use when talking to the given address - either `http` or `https`. TLS
     * support can also be enabled by setting the environment variable `CONSUL_HTTP_SSL` to `true`.
     */
    scheme?: pulumi.Input<string>;

    /**
     * The datacenter to use. Defaults to that of the agent.
     */
    datacenter?: pulumi.Input<string>;

    /**
     * HTTP Basic Authentication credentials to be used when communicating with Consul, in the format of
     * either `user` or `user:pass`. Sourced from `CONSUL_HTTP_AUTH`, if unset.
     */
    httpAuth?: pulumi.Input<string>;

    /**
     * Whether to compress the state data using gzip. Set to `true` to compress the state data using gzip,
     * or `false` (default) to leave it uncompressed.
     */
    gzip?: pulumi.Input<boolean>;

    /**
     * A path to a PEM-encoded certificate authority used to verify the remote agent's certificate. Sourced
     * from `CONSUL_CAFILE` in the environment, if unset.
     */
    caFile?: pulumi.Input<string>;

    /**
     * A path to a PEM-encoded certificate provided to the remote agent; requires use of key_file. Sourced
     * from `CONSUL_CLIENT_CERT` in the environment, if unset.
     */
    certFile?: pulumi.Input<string>;

    /**
     * A path to a PEM-encoded private key, required if cert_file is specified. Sourced from `CONSUL_CLIENT_KEY`
     * in the environment, if unset.
     */
    keyFile?: pulumi.Input<string>;

    /**
     * The Terraform workspace from which to read state.
     */
    workspace?: pulumi.Input<string>;
}
