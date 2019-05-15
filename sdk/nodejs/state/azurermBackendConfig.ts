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

export type AzureEnvironment = "public"
    | "china"
    | "german"
    | "stack"
    | "usgovernment";

/**
 * The configuration options for a Terraform Remote State stored in the AzureRM backend.
 */
export interface AzureRMRemoteStateReferenceArgs {
    /**
     * A constant describing the name of the Terraform backend, used as the discriminant
     * for the union of backend configurations.
     */
    backendType: "azurerm";

    /**
     * The name of the storage account.
     */
    storageAccountName: pulumi.Input<string>;

    /**
     * The name of the storage container within the storage account.
     */
    containerName: pulumi.Input<string>;

    /**
     * The name of the blob in representing the Terraform State file inside the storage container.
     */
    key?: pulumi.Input<string>;

    /**
     * The Azure environment which should be used. Possible values are `public` (default), `china`,
     * `german`, `stack` and `usgovernment`. Sourced from `ARM_ENVIRONMENT`, if unset.
     */
    environment?: pulumi.Input<AzureEnvironment>;

    /**
     * The custom endpoint for Azure Resource Manager. Sourced from `ARM_ENDPOINT`, if unset.
     */
    endpoint?: pulumi.Input<string>;

    /**
     * Whether to authenticate using Managed Service Identity (MSI). Sourced from `ARM_USE_MSI`
     * if unset. Defaults to false if no value is specified.
     */
    useMsi?: pulumi.Input<boolean>;

    /**
     * The Subscription ID in which the Storage Account exists. Used when authenticating using
     * the Managed Service Identity (MSI) or a service principal. Sourced from `ARM_SUBSCRIPTION_ID`,
     * if unset.
     */
    subscriptionId?: pulumi.Input<string>;

    /**
     * The Tenant ID in which the Subscription exists. Used when authenticating using the
     * Managed Service Identity (MSI) or a service principal. Sourced from `ARM_TENANT_ID`,
     * if unset.
     */
    tenantId?: pulumi.Input<string>;

    /**
     * The path to a custom Managed Service Identity endpoint. Used when authenticating using
     * the Managed Service Identity (MSI). Sourced from `ARM_MSI_ENDPOINT` in the environment,
     * if unset. Automatically determined, if no value is provided.
     */
    msiEndpoint?: pulumi.Input<string>;

    /**
     * The SAS Token used to access the Blob Storage Account. Used when authenticating using
     * a SAS Token. Sourced from `ARM_SAS_TOKEN` in the environment, if unset.
     */
    sasToken?: pulumi.Input<string>;

    /**
     * The Access Key used to access the blob storage account. Used when authenticating using
     * an access key. Sourced from `ARM_ACCESS_KEY` in the environment, if unset.
     */
    accessKey?: pulumi.Input<string>;

    /**
     * The name of the resource group in which the storage account exists. Used when authenticating
     * using a service principal.
     */
    resourceGroupName?: pulumi.Input<string>;

    /**
     * The client ID of the service principal. Used when authenticating using a service principal.
     * Sourced from `ARM_CLIENT_ID` in the environment, if unset.
     */
    clientId?: pulumi.Input<string>;

    /**
     * The client secret of the service principal. Used when authenticating using a service principal.
     * Sourced from `ARM_CLIENT_SECRET` in the environment, if unset.
     */
    clientSecret?: pulumi.Input<string>;

    /**
     * The Terraform workspace from which to read state.
     */
    workspace?: pulumi.Input<string>;
}
