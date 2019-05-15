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
import {ArtifactoryRemoteStateReferenceArgs} from "./artifactoryBackendConfig";
import {AzureRMRemoteStateReferenceArgs} from "./azurermBackendConfig";
import {ConsulRemoteStateReferenceArgs} from "./consulBackendConfig";
import {EtcdV2RemoteStateReferenceArgs} from "./etcdBackendConfig";
import {EtcdV3RemoteStateReferenceArgs} from "./etcdV3BackendConfig";
import {GCSRemoteStateReferenceArgs} from "./gcsBackendConfig";
import {HttpRemoteStateReferenceArgs} from "./httpBackendConfig";
import {LocalBackendRemoteStateReferenceArgs} from "./localBackendConfig";
import {MantaRemoteStateReferenceArgs} from "./mantaBackendConfig";
import {PostgresRemoteStateReferenceArgs} from "./pgBackendConfig";
import {RemoteBackendRemoteStateReferenceArgs} from "./remoteBackendConfig";
import {S3RemoteStateReferenceArgs} from "./s3BackendConfig";
import {SwiftRemoteStateReferenceArgs} from "./swiftBackendConfig";

/**
 * The set of arguments for constructing a RemoteStateReference resource.
 */
export type RemoteStateReferenceArgs = ArtifactoryRemoteStateReferenceArgs
    | AzureRMRemoteStateReferenceArgs
    | ConsulRemoteStateReferenceArgs
    | EtcdV2RemoteStateReferenceArgs
    | EtcdV3RemoteStateReferenceArgs
    | GCSRemoteStateReferenceArgs
    | HttpRemoteStateReferenceArgs
    | LocalBackendRemoteStateReferenceArgs
    | MantaRemoteStateReferenceArgs
    | PostgresRemoteStateReferenceArgs
    | RemoteBackendRemoteStateReferenceArgs
    | S3RemoteStateReferenceArgs
    | SwiftRemoteStateReferenceArgs;

/**
 * Manages a reference to a Terraform Remote State.. The root outputs of the remote state are available
 * via the `outputs` property or the `getOutput` method.
 */
export class RemoteStateReference extends pulumi.CustomResource {
    /**
     * The root outputs of the referenced Terraform state.
     */
    public readonly outputs: pulumi.Output<{ [name: string]: any }>;

    /**
     * Create a RemoteStateReference resource with the given unique name, arguments, and options.
     *
     * @param name The _unique_ name of the remote state reference.
     * @param args The arguments to use to populate this resource's properties.
     * @param opts A bag of options that control this resource's behavior.
     */
    constructor(name: string, args: RemoteStateReferenceArgs, opts?: pulumi.CustomResourceOptions) {
        super("terraform:state:RemoteStateReference", name, {
            outputs: undefined,
            ...args
        }, {...opts, id: name,});
    }

    /**
     * Fetches the value of a root output from the Terraform Remote State.
     *
     * @param name The name of the output to fetch. The name is formatted exactly as per
     * the "output" block in the Terraform configuration.
     */
    public getOutput(name: pulumi.Input<string>): pulumi.Output<any> {
        return pulumi.all([pulumi.output(name), this.outputs]).apply(([n, os]) => os[n]);
    }
}

