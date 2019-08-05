"""
Resources for consuming Terraform Remote State.
"""
import copy
from typing import Any, Mapping, Sequence, Union

import pulumi
import pulumi.runtime


class ArtifactoryBackendArgs:
    """
    The configuration options for a Terraform Remote State stored in the Artifactory backend.
    """

    def __init__(self,
                 repo: pulumi.Input[str],
                 subpath: pulumi.Input[str],
                 url: pulumi.Input[str] = None,
                 username: pulumi.Input[str] = None,
                 password: pulumi.Input[str] = None,
                 workspace: pulumi.Input[str] = None):
        """
        Constructs an ArtifactoryBackendArgs.

        :param repo: The username with which to authenticate to Artifactory. Sourced from `ARTIFACTORY_USERNAME` in the
         environment, if unset
        :param subpath: Path within the repository.
        :param url: The Artifactory URL. Note that this is the base URL to artifactory, not the full repo and subpath.
        However, it must include the path to the artifactory installation - likely this will end in `/artifactory`.
        Sourced from `ARTIFACTORY_URL` in the environment, if unset.
        :param username: The username with which to authenticate to Artifactory. Sourced from `ARTIFACTORY_USERNAME`
        in the environment, if unset.
        :param password: The password with which to authenticate to Artifactory. Sourced from `ARTIFACTORY_PASSWORD`
        in the environment, if unset.
        :param workspace: The Terraform workspace from which to read state.
        """
        self.props = dict()
        self.props["repo"] = repo
        self.props["repo"] = repo
        self.props["subpath"] = subpath
        self.props["url"] = url
        self.props["username"] = username
        self.props["password"] = password
        self.props["workspace"] = workspace


class AzureRMBackendArgs:
    """
    The configuration options for a Terraform Remote State stored in the AzureRM backend.
    """

    def __init__(self,
                 storage_account_name: pulumi.Input[str],
                 container_name: pulumi.Input[str],
                 key: pulumi.Input[str] = None,
                 environment: pulumi.Input[str] = None,
                 endpoint: pulumi.Input[str] = None,
                 use_msi: pulumi.Input[bool] = None,
                 subscription_id: pulumi.Input[str] = None,
                 tenant_id: pulumi.Input[str] = None,
                 msi_endpoint: pulumi.Input[str] = None,
                 sas_token: pulumi.Input[str] = None,
                 access_key: pulumi.Input[str] = None,
                 resource_group_name: pulumi.Input[str] = None,
                 client_id: pulumi.Input[str] = None,
                 client_secret: pulumi.Input[str] = None,
                 workspace: pulumi.Input[str] = None):
        """
        Constructs an AzureRMBackendArgs

        :param storage_account_name: The name of the storage account.
        :param container_name: The name of the storage container within the storage account.
        :param key: The name of the blob in representing the Terraform State file inside the storage container.
        :param environment: The Azure environment which should be used. Possible values are `public` (default), `china`,
       `german`, `stack` and `usgovernment`. Sourced from `ARM_ENVIRONMENT`, if unset.
        :param endpoint: The custom endpoint for Azure Resource Manager. Sourced from `ARM_ENDPOINT`, if unset.
        :param use_msi: Whether to authenticate using Managed Service Identity (MSI). Sourced from `ARM_USE_MSI` if
        unset. Defaults to false if no value is specified.
        :param subscription_id: The Subscription ID in which the Storage Account exists. Used when authenticating using
        the Managed Service Identity (MSI) or a service principal. Sourced from `ARM_SUBSCRIPTION_ID`, if unset.
        :param tenant_id: The Tenant ID in which the Subscription exists. Used when authenticating using the
        Managed Service Identity (MSI) or a service principal. Sourced from `ARM_TENANT_ID`, if unset.
        :param msi_endpoint: The path to a custom Managed Service Identity endpoint. Used when authenticating using the
        Managed Service Identity (MSI). Sourced from `ARM_MSI_ENDPOINT` in the environment, if unset. Automatically
        determined, if no value is provided.
        :param sas_token: The SAS Token used to access the Blob Storage Account. Used when authenticating using a SAS
        Token. Sourced from `ARM_SAS_TOKEN` in the environment, if unset.
        :param access_key: The Access Key used to access the blob storage account. Used when authenticating using an
        access key. Sourced from `ARM_ACCESS_KEY` in the environment, if unset.
        :param resource_group_name: The name of the resource group in which the storage account exists. Used when
        authenticating using a service principal.
        :param client_id: The client ID of the service principal. Used when authenticating using a service principal.
        Sourced from `ARM_CLIENT_ID` in the environment, if unset.
        :param client_secret: The client secret of the service principal. Used when authenticating using a service
        principal. Sourced from `ARM_CLIENT_SECRET` in the environment, if unset.
        :param workspace: The Terraform workspace from which to read state.
        """
        self.props = dict()
        self.props["storage_account_name"] = storage_account_name
        self.props["container_name"] = container_name
        self.props["key"] = key
        self.props["environment"] = environment
        self.props["endpoint"] = endpoint
        self.props["use_msi"] = use_msi
        self.props["subscription_id"] = subscription_id
        self.props["tenant_id"] = tenant_id
        self.props["msi_endpoint"] = msi_endpoint
        self.props["sas_token"] = sas_token
        self.props["access_key"] = access_key
        self.props["resource_group_name"] = resource_group_name
        self.props["client_id"] = client_id
        self.props["client_secret"] = client_secret
        self.props["workspace"] = workspace


class ConsulBackendArgs:
    """
    The configuration options for a Terraform Remote State stored in the Consul backend.
    """

    def __init__(self,
                 path: pulumi.Input[str],
                 access_token: pulumi.Input[str],
                 address: pulumi.Input[str] = None,
                 scheme: pulumi.Input[str] = None,
                 datacenter: pulumi.Input[str] = None,
                 http_auth: pulumi.Input[str] = None,
                 gzip: pulumi.Input[bool] = None,
                 ca_file: pulumi.Input[str] = None,
                 cert_file: pulumi.Input[str] = None,
                 key_file: pulumi.Input[str] = None,
                 workspace: pulumi.Input[str] = None):
        """
        Constructs a ConsulBackendArgs.

        :param path: Path in the Consul KV store.
        :param access_token: Consul Access Token. Sourced from `CONSUL_HTTP_TOKEN` in the environment, if unset.
        :param address: DNS name and port of the Consul HTTP endpoint specified in the format `dnsname:port`. Defaults
        to the local agent HTTP listener.
        :param scheme: Specifies which protocol to use when talking to the given address - either `http` or `https`. TLS
        support can also be enabled by setting the environment variable `CONSUL_HTTP_SSL` to `true`.
        :param datacenter: The datacenter to use. Defaults to that of the agent.
        :param http_auth: HTTP Basic Authentication credentials to be used when communicating with Consul, in the format
        of either `user` or `user:pass`. Sourced from `CONSUL_HTTP_AUTH`, if unset.
        :param gzip: Whether to compress the state data using gzip. Set to `true` to compress the state data using gzip,
        or `false` (default) to leave it uncompressed.
        :param ca_file: A path to a PEM-encoded certificate authority used to verify the remote agent's certificate.
        Sourced from `CONSUL_CAFILE` in the environment, if unset.
        :param cert_file: A path to a PEM-encoded certificate provided to the remote agent; requires use of key_file.
        Sourced from `CONSUL_CLIENT_CERT` in the environment, if unset.
        :param key_file: A path to a PEM-encoded private key, required if cert_file is specified. Sourced from
        `CONSUL_CLIENT_KEY` in the environment, if unset.
        :param workspace: The Terraform workspace from which to read state.
        """
        self.props = dict()
        self.props["path"] = path
        self.props["access_token"] = access_token
        self.props["address"] = address
        self.props["scheme"] = scheme
        self.props["datacenter"] = datacenter
        self.props["http_auth"] = http_auth
        self.props["gzip"] = gzip
        self.props["ca_file"] = ca_file
        self.props["cert_file"] = cert_file
        self.props["key_file"] = key_file
        self.props["workspace"] = workspace


class EtcdV2BackendArgs:
    """
    The configuration options for a Terraform Remote State stored in the etcd v2 backend. Note that there is a separate
    configuration class for state stored in the ectd v3 backend.
    """

    def __init__(self,
                 path: pulumi.Input[str],
                 endpoints: pulumi.Input[str],
                 username: pulumi.Input[str] = None,
                 password: pulumi.Input[str] = None,
                 workspace: pulumi.Input[str] = None):
        """
        Constructs an EtcdV2BackendArgs.

        :param path: The path at which to store the state.
        :param endpoints: A space-separated list of the etcd endpoints.
        :param username: The username with which to authenticate to etcd.
        :param password: The username with which to authenticate to etcd.
        :param workspace: The Terraform workspace from which to read state.
        """
        self.props = dict()
        self.props["path"] = path
        self.props["endpoints"] = endpoints
        self.props["username"] = username
        self.props["password"] = password
        self.props["workspace"] = workspace


class EtcdV3BackendArgs:
    """
    The configuration options for a Terraform Remote State stored in the etcd v3 backend. Note that there is a separate
    configuration class for state stored in the ectd v2 backend.
    """

    def __init__(self,
                 endpoints: pulumi.Input[Sequence[pulumi.Input[str]]],
                 username: pulumi.Input[str] = None,
                 password: pulumi.Input[str] = None,
                 prefix: pulumi.Input[str] = None,
                 cacert_path: pulumi.Input[str] = None,
                 cert_path: pulumi.Input[str] = None,
                 key_path: pulumi.Input[str] = None,
                 workspace: pulumi.Input[str] = None):
        """
        Constructs an EtcdV3BackendArgs.

        :param endpoints: A list of the etcd endpoints.
        :param username: The username with which to authenticate to etcd. Sourced from `ETCDV3_USERNAME` in the
        environment, if unset.
        :param password: The username with which to authenticate to etcd. Sourced from `ETCDV3_PASSWORD` in the
        environment, if unset.
        :param prefix: An optional prefix to be added to keys when storing state in etcd.
        :param cacert_path: Path to a PEM-encoded certificate authority bundle with which to verify certificates of
        TLS-enabled etcd servers.
        :param cert_path: Path to a PEM-encoded certificate to provide to etcd for client authentication.
        :param key_path: Path to a PEM-encoded key to use for client authentication.
        :param workspace: The Terraform workspace from which to read state.
        """
        self.props = dict()
        self.props["endpoints"] = endpoints
        self.props["username"] = username
        self.props["password"] = password
        self.props["prefix"] = prefix
        self.props["cacert_path"] = cacert_path
        self.props["cert_path"] = cert_path
        self.props["key_path"] = key_path
        self.props["workspace"] = workspace


class GcsBackendArgs:
    """
    The configuration options for a Terraform Remote State stored in the Google Cloud Storage backend.
    """

    def __init__(self,
                 bucket: pulumi.Input[str],
                 credentials: pulumi.Input[str] = None,
                 prefix: pulumi.Input[str] = None,
                 encryption_key: pulumi.Input[str] = None,
                 workspace: pulumi.Input[str] = None):
        """
        Constructs a GcsBackendArgs.

        :param bucket: The name of the Google Cloud Storage bucket.
        :param credentials: Local path to Google Cloud Platform account credentials in JSON format. Sourced from
        `GOOGLE_CREDENTIALS` in the environment if unset. If no value is provided Google Application Default Credentials
        are used.
        :param prefix: Prefix used inside the Google Cloud Storage bucket. Named states for workspaces are stored in an
        object named `<prefix>/<name>.tfstate`.
        :param encryption_key: A 32 byte, base64-encoded customer supplied encryption key used to encrypt the state.
        Sourced from `GOOGLE_ENCRYPTION_KEY` in the environment, if unset.
        :param workspace: The Terraform workspace from which to read state.
        """
        self.props = dict()
        self.props["bucket"] = bucket
        self.props["credentials"] = credentials
        self.props["prefix"] = prefix
        self.props["encryption_key"] = encryption_key
        self.props["workspace"] = workspace


class HttpBackendArgs:
    """
    The configuration options for a Terraform Remote State stored in the HTTP backend.
    """

    def __init__(self,
                 address: pulumi.Input[str],
                 update_method: pulumi.Input[str] = None,
                 lock_address: pulumi.Input[str] = None,
                 lock_method: pulumi.Input[str] = None,
                 unlock_address: pulumi.Input[str] = None,
                 unlock_method: pulumi.Input[str] = None,
                 username: pulumi.Input[str] = None,
                 password: pulumi.Input[str] = None,
                 skip_cert_validation: pulumi.Input[bool] = None,
                 workspace: pulumi.Input[str] = None):
        """
        Constructs an HttpBackendArgs.

        :param address: The address of the HTTP endpoint.
        :param update_method: HTTP method to use when updating state. Defaults to `POST`.
        :param lock_address: The address of the lock REST endpoint. Not setting a value disables locking.
        :param lock_method: The HTTP method to use when locking. Defaults to `LOCK`.
        :param unlock_address: The address of the unlock REST endpoint. Not setting a value disables locking.
        :param unlock_method: The HTTP method to use when unlocking. Defaults to `UNLOCK`.
        :param username: The username used for HTTP basic authentication.
        :param password: The password used for HTTP basic authentication.
        :param skip_cert_validation: Whether to skip TLS verification. Defaults to false.
        :param workspace: The Terraform workspace from which to read state.
        """
        self.props = dict()
        self.props["address"] = address
        self.props["update_method"] = update_method
        self.props["lock_address"] = lock_address
        self.props["lock_method"] = lock_method
        self.props["unlock_address"] = unlock_address
        self.props["unlock_method"] = unlock_method
        self.props["username"] = username
        self.props["password"] = password
        self.props["skip_cert_validation"] = skip_cert_validation
        self.props["workspace"] = workspace


class LocalBackendArgs:
    """
    The configuration options for a Terraform Remote State stored in the local enhanced backend.
    """

    def __init__(self,
                 path: pulumi.Input[str]):
        """
        Constructs a LocalBackendArgs.

        :param path: The path to the Terraform state file.
        """
        self.props = dict()
        self.props["path"] = path


class MantaBackendArgs:
    """
    The configuration options for a Terraform Remote State stored in the Manta backend.
    """

    def __init__(self,
                 account: pulumi.Input[str],
                 user: pulumi.Input[str] = None,
                 url: pulumi.Input[str] = None,
                 key_material: pulumi.Input[str] = None,
                 key_id: pulumi.Input[str] = None,
                 path: pulumi.Input[str] = None,
                 insecure_skip_tls_verify: pulumi.Input[bool] = None,
                 workspace: pulumi.Input[str] = None):
        """
        Constructs a MantaBackendArgs.

        :param account: The name of the Manta account. Sourced from `SDC_ACCOUNT` or `_ACCOUNT` in the environment, if
        unset.
        :param user: The username of the Manta account with which to authenticate.
        :param url: The Manta API Endpoint. Sourced from `MANTA_URL` in the environment, if unset. Defaults to
        `https://us-east.manta.joyent.com`.
        :param key_material: The private key material corresponding with the SSH key whose fingerprint is specified in
        keyId. Sourced from `SDC_KEY_MATERIAL` or `TRITON_KEY_MATERIAL` in the environment, if unset. If no value is
        specified, the local SSH agent is used for signing requests.
        :param key_id: The fingerprint of the public key matching the key material specified in keyMaterial, or in the
        local SSH agent.
        :param path: The path relative to your private storage directory (`/$MANTA_USER/stor`) where the state file
        will be stored.
        :param insecure_skip_tls_verify: Skip verifying the TLS certificate presented by the Manta endpoint. This can be
         useful for installations which do not have a certificate signed by a trusted root CA. Defaults to false.
        :param workspace: The Terraform workspace from which to read state.
        """
        self.props = dict()
        self.props["account"] = account
        self.props["user"] = user
        self.props["url"] = url
        self.props["key_material"] = key_material
        self.props["key_id"] = key_id
        self.props["path"] = path
        self.props["insecure_skip_tls_verify"] = insecure_skip_tls_verify
        self.props["workspace"] = workspace


class PostgresBackendArgs:
    """
    The configuration options for a Terraform Remote State stored in the Postgres backend.
    """

    def __init__(self,
                 conn_str: pulumi.Input[str],
                 schema_name: pulumi.Input[str] = None,
                 workspace: pulumi.Input[str] = None):
        """
        Constructs a PostgresBackendArgs.

        :param conn_str: Postgres connection string; a `postgres://` URL.
        :param schema_name: Name of the automatically-managed Postgres schema. Defaults to `terraform_remote_state`.
        :param workspace: The Terraform workspace from which to read state.
        """
        self.props = dict()
        self.props["conn_str"] = conn_str
        self.props["schema_name"] = schema_name
        self.props["workspace"] = workspace


class RemoteBackendArgs:
    """
    Configuration options for a workspace for use with the remote enhanced backend.
    """

    def __init__(self,
                 organization: pulumi.Input[str],
                 token: pulumi.Input[str] = None,
                 hostname: pulumi.Input[str] = None,
                 workspace_name: pulumi.Input[str] = None,
                 workspace_prefix: pulumi.Input[str] = None):
        """
        Constructs a RemoteBackendArgs.

        :param organization: The name of the organization containing the targeted workspace(s).
        :param token: The token used to authenticate with the remote backend.
        :param hostname: The remote backend hostname to which to connect. Defaults to `app.terraform.io`.
        :param workspace_name: The full name of one remote workspace. When configured, only the default workspace can
        be used. This option conflicts with workspace_prefix.
        :param workspace_prefix: A prefix used in the names of one or more remote workspaces, all of which can be used
        with this configuration. If unset, only the default workspace can be used. This option conflicts with
        workspace_name.
        """
        if not organization:
            raise TypeError('Missing organization argument')
        if workspace_name and workspace_prefix:
            raise TypeError('Only workspace_name or workspace_prefix may be given')
        if not workspace_name and not workspace_prefix:
            raise TypeError('Either workspace_name or workspace_prefix is required')
        self.props = dict()
        self.props["organization"] = organization
        self.props["token"] = token
        self.props["hostname"] = hostname
        self.props["workspaces"] = dict()
        self.props["workspaces"]["workspace_name"] = workspace_name
        self.props["workspaces"]["workspace_prefix"] = workspace_prefix


class S3BackendArgs:
    """
    The configuration options for a Terraform Remote State stored in the S3 backend.
    """

    def __init__(self,
                 bucket: pulumi.Input[str],
                 key: pulumi.Input[str],
                 region: pulumi.Input[str] = None,
                 endpoint: pulumi.Input[str] = None,
                 access_key: pulumi.Input[str] = None,
                 secret_key: pulumi.Input[str] = None,
                 profile: pulumi.Input[str] = None,
                 shared_credentials_file: pulumi.Input[str] = None,
                 token: pulumi.Input[str] = None,
                 role_arn: pulumi.Input[str] = None,
                 external_id: pulumi.Input[str] = None,
                 session_name: pulumi.Input[str] = None,
                 workspace_key_prefix: pulumi.Input[str] = None,
                 iam_endpoint: pulumi.Input[str] = None,
                 sts_endpoint: pulumi.Input[str] = None,
                 workspace: pulumi.Input[str] = None):
        """
        Constructs an S3BackendArgs.

        :param bucket: The name of the S3 bucket.
        :param key: The path to the state file inside the bucket. When using a non-default workspace, the state path
        will be `/workspace_key_prefix/workspace_name/key`.
        :param region: The region of the S3 bucket. Also sourced from `AWS_DEFAULT_REGION` in the environment, if unset.
        :param endpoint: A custom endpoint for the S3 API. Also sourced from `AWS_S3_ENDPOINT` in the environment, if
        unset.
        :param access_key: AWS Access Key. Sourced from the standard credentials pipeline, if unset.
        :param secret_key: AWS Secret Access Key. Sourced from the standard credentials pipeline, if unset.
        :param profile: The AWS profile name as set in the shared credentials file.
        :param shared_credentials_file: The path to the shared credentials file. If this is not set and a profile is
        specified, `~/.aws/credentials` will be used by default.
        :param token: An MFA token. Sourced from `AWS_SESSION_TOKEN` in the environment if needed and unset.
        :param role_arn: The ARN of an IAM Role to be assumed in order to read the state from S3.
        :param external_id: The external ID to use when assuming the IAM role.
        :param session_name: The session name to use when assuming the IAM role.
        :param workspace_key_prefix: The prefix applied to the state path inside the bucket. This is only relevant when
        using a non-default workspace, and defaults to `env:`.
        :param iam_endpoint: A custom endpoint for the IAM API. Sourced from `AWS_IAM_ENDPOINT`, if unset.
        :param sts_endpoint: A custom endpoint for the STS API. Sourced from `AWS_STS_ENDPOINT`, if unset.
        :param workspace: The Terraform workspace from which to read state.
        """
        self.props = dict()
        self.props["bucket"] = bucket
        self.props["key"] = key
        self.props["region"] = region
        self.props["endpoint"] = endpoint
        self.props["access_key"] = access_key
        self.props["secret_key"] = secret_key
        self.props["profile"] = profile
        self.props["shared_credentials_file"] = shared_credentials_file
        self.props["token"] = token
        self.props["role_arn"] = role_arn
        self.props["external_id"] = external_id
        self.props["session_name"] = session_name
        self.props["workspace_key_prefix"] = workspace_key_prefix
        self.props["iam_endpoint"] = iam_endpoint
        self.props["sts_endpoint"] = sts_endpoint
        self.props["workspace"] = workspace


class SwiftBackendArgs:
    """
    The configuration options for a Terraform Remote State stored in the Swift backend.
    """

    def __init__(self,
                 auth_url: pulumi.Input[str],
                 container: pulumi.Input[str],
                 username: pulumi.Input[str] = None,
                 user_id: pulumi.Input[str] = None,
                 password: pulumi.Input[str] = None,
                 token: pulumi.Input[str] = None,
                 region_name: pulumi.Input[str] = None,
                 tenant_id: pulumi.Input[str] = None,
                 tenant_name: pulumi.Input[str] = None,
                 domain_id: pulumi.Input[str] = None,
                 domain_name: pulumi.Input[str] = None,
                 insecure: pulumi.Input[bool] = None,
                 cacert_file: pulumi.Input[str] = None,
                 cert: pulumi.Input[str] = None,
                 key: pulumi.Input[str] = None):
        """
        Constructs a SwiftBackendArgs.

        :param auth_url: The Identity authentication URL. Sourced from `OS_AUTH_URL` in the environment, if unset.
        :param container: The name of the container in which the Terraform state file is stored.
        :param username: The username with which to log in. Sourced from `OS_USERNAME` in the environment, if unset.
        :param user_id: The user ID with which to log in. Sourced from `OS_USER_ID` in the environment, if unset.
        :param password:  The password with which to log in. Sourced from `OS_PASSWORD` in the environment, if unset.
        :param token: Access token with which to log in in stead of a username and password. Sourced from
        `OS_AUTH_TOKEN` in the environment, if unset.
        :param region_name: The region in which the state file is stored. Sourced from `OS_REGION_NAME`, if unset.
        :param tenant_id: The ID of the tenant (for identity v2) or project (identity v3) which which to log in. Sourced
        from `OS_TENANT_ID` or `OS_PROJECT_ID` in the environment, if unset.
        :param tenant_name: The name of the tenant (for identity v2) or project (identity v3) which which to log in.
        Sourced from `OS_TENANT_NAME` or `OS_PROJECT_NAME` in the environment, if unset.
        :param domain_id: The ID of the domain to scope the log in to (identity v3). Sourced from `OS_USER_DOMAIN_ID`,
        `OS_PROJECT_DOMAIN_ID` or `OS_DOMAIN_ID` in the environment, if unset.
        :param domain_name:  The name of the domain to scope the log in to (identity v3). Sourced from
        `OS_USER_DOMAIN_NAME`, `OS_PROJECT_DOMAIN_NAME` or `OS_DOMAIN_NAME` in the environment, if unset.
        :param insecure: Whether to disable verification of the server TLS certificate. Sourced from `OS_INSECURE` in
        the environment, if unset.
        :param cacert_file: A path to a CA root certificate for verifying the server TLS certificate. Sourced from
        `OS_CACERT` in the environment, if unset.
        :param cert: A path to a client certificate for TLS client authentication. Sourced from `OS_CERT` in the
        environment, if unset.
        :param key: A path to the private key corresponding to the client certificate for TLS client authentication.
        Sourced from `OS_KEY` in the environment, if unset.
        """
        self.props = dict()
        self.props["auth_url"] = auth_url
        self.props["container"] = container
        self.props["username"] = username
        self.props["user_id"] = user_id
        self.props["password"] = password
        self.props["token"] = token
        self.props["region_name"] = region_name
        self.props["tenant_id"] = tenant_id
        self.props["tenant_name"] = tenant_name
        self.props["domain_id"] = domain_id
        self.props["domain_name"] = domain_name
        self.props["insecure"] = insecure
        self.props["cacert_file"] = cacert_file
        self.props["cert"] = cert
        self.props["key"] = key


BackendArgs = Union[ArtifactoryBackendArgs, AzureRMBackendArgs, ConsulBackendArgs, EtcdV2BackendArgs,
                    EtcdV3BackendArgs, GcsBackendArgs, HttpBackendArgs, LocalBackendArgs, MantaBackendArgs,
                    PostgresBackendArgs, RemoteBackendArgs, S3BackendArgs, SwiftBackendArgs]
"""
BackendArgs is a union type representing all possible types of backend configuration.
"""


class RemoteStateReference(pulumi.CustomResource):
    """
    Manages a reference to a Terraform Remote State. The root outputs of the remote state are available via the
    `outputs` property or the `getOutput` method.
    """

    outputs: pulumi.Output[Mapping[str, Any]]
    """
    The root outputs of the referenced Terraform state.
    """

    def __init__(self,
                 resource_name: str,
                 backend_type: str,
                 args: BackendArgs,
                 opts: pulumi.ResourceOptions = None):
        """
        Create a RemoteStateReference resource with the given unique name, props, and options.

        :param str resource_name: The name of the resource.
        :param pulumi.ResourceOptions opts: Options for the resource.
        :param str backend_type: The name of the Remote State Backend
        :param BackendArgs args: The arguments for the Remote State backend, which must match the given backend_type.
        """
        if not resource_name:
            raise TypeError('Missing resource name argument (for URN creation)')
        if not isinstance(resource_name, str):
            raise TypeError('Expected resources name to be a string')
        if opts and not isinstance(opts, pulumi.ResourceOptions):
            raise TypeError('Expected resource options to be a ResourceOptions instance')
        if not args:
            raise TypeError('Expected args to be an object')

        if not isinstance(backend_type, str):
            raise TypeError('Expected backend_type to be a string')

        backend_validate = RemoteStateReference.__VALIDATE_ARGS.get(backend_type, None)
        if backend_validate is None:
            raise TypeError('Expected a valid backend type as value of backend_type')

        if not backend_validate[0]:
            raise TypeError(
                f'Expected args to be an instance of {backend_validate[1]} '
                f'when backend_type is "{backend_type}"')

        __props__ = dict()
        __props__['backend_type'] = backend_type
        __props__['outputs'] = None

        for key, value in args.props.items():
            __props__[key] = value

        # Force Read operations
        if opts is None:
            opts_with_id = pulumi.ResourceOptions()
        else:
            opts_with_id = copy.deepcopy(opts)
        opts_with_id.id = resource_name

        super().__init__('terraform:state:RemoteStateReference',
                         resource_name,
                         __props__,
                         opts_with_id)

    def get_output(self, name: pulumi.Input[str]):
        """
        Fetches the value of a root output from the Terraform Remote State.

        :param name: The name of the output to fetch. The name is formatted exactly as per the "output" block in the
        Terraform configuration.
        :return: A pulumi.Output representing the value.
        """
        return pulumi.Output.all(pulumi.Output.from_input(name), self.outputs).apply(lambda args: args[1][args[0]])

    __VALIDATE_ARGS = {
        "artifactory": (lambda args: isinstance(args, ArtifactoryBackendArgs), "ArtifactoryBackendArgs"),
        "azurerm": (lambda args: isinstance(args, AzureRMBackendArgs), "AzureRMBackendArgs"),
        "consul": (lambda args: isinstance(args, ConsulBackendArgs), "ConsulBackendArgs"),
        "etcd": (lambda args: isinstance(args, EtcdV2BackendArgs), "EtcdV2BackendArgs"),
        "etcdv3": (lambda args: isinstance(args, EtcdV3BackendArgs), "EtcdV3BackendArgs"),
        "gcs": (lambda args: isinstance(args, GcsBackendArgs), "GcsBackendArgs"),
        "http": (lambda args: isinstance(args, HttpBackendArgs), "HttpBackendArgs"),
        "local": (lambda args: isinstance(args, LocalBackendArgs), "LocalBackendArgs"),
        "manta": (lambda args: isinstance(args, MantaBackendArgs), "MantaBackendArgs"),
        "postgres": (lambda args: isinstance(args, PostgresBackendArgs), "PostgresBackendArgs"),
        "remote": (lambda args: isinstance(args, RemoteBackendArgs), "RemoteBackendArgs"),
        "s3": (lambda args: isinstance(args, S3BackendArgs), "S3BackendArgs"),
        "swift": (lambda args: isinstance(args, SwiftBackendArgs), "SwiftBackendArgs"),
    }

    def translate_output_property(self, prop):
        return RemoteStateReference.__CAMEL_TO_SNAKE_CASE_TABLE.get(prop) or prop

    def translate_input_property(self, prop):
        return RemoteStateReference.__SNAKE_TO_CAMEL_CASE_TABLE.get(prop) or prop

    __SNAKE_TO_CAMEL_CASE_TABLE = {
        "backend_type": "backendType",
        "access_key": "accessKey",
        "access_token": "accessToken",
        "account": "account",
        "address": "address",
        "auth_url": "authUrl",
        "bucket": "bucket",
        "ca_file": "caFile",
        "cacert_file": "cacertFile",
        "cacert_path": "cacertPath",
        "cert": "cert",
        "cert_file": "certFile",
        "cert_path": "certPath",
        "client_id": "clientId",
        "client_secret": "clientSecret",
        "conn_str": "connStr",
        "container": "container",
        "container_name": "containerName",
        "credentials": "credentials",
        "datacenter": "datacenter",
        "domain_id": "domainId",
        "domain_name": "domainName",
        "encryption_key": "encryptionKey",
        "endpoint": "endpoint",
        "endpoints": "endpoints",
        "environment": "environment",
        "external_id": "externalId",
        "gzip": "gzip",
        "hostname": "hostname",
        "http_auth": "http_auth",
        "iam_endpoint": "iamEndpoint",
        "insecure": "insecure",
        "insecure_skip_tls_verify": "insecureSkipTlsVerify",
        "key": "key",
        "key_file": "keyFile",
        "key_id": "keyId",
        "key_material": "keyMaterial",
        "key_path": "keyPath",
        "lock_address": "lockAddress",
        "lock_method": "lockMethod",
        "msi_endpoint": "msiEndpoint",
        "organization": "organization",
        "outputs": "outputs",
        "password": "password",
        "path": "path",
        "prefix": "prefix",
        "profile": "profile",
        "region": "region",
        "region_name": "regionName",
        "repo": "repo",
        "resource_group_name": "resourceGroupName",
        "role_arn": "roleArn",
        "sas_token": "sasToken",
        "schema_name": "schemaName",
        "scheme": "scheme",
        "secret_key": "secretKey",
        "session_name": "sessionName",
        "shared_credentials_file": "sharedCredentialsFile",
        "skip_cert_validation": "skipCertValidation",
        "storage_account_name": "storageAccountName",
        "sts_endpoint": "stsEndpoint",
        "subpath": "subpath",
        "subscription_id": "subscriptionId",
        "tenant_id": "tenantId",
        "tenant_name": "tenantName",
        "token": "token",
        "unlock_address": "unlockAddress",
        "unlock_method": "unlockMethod",
        "update_method": "updateMethod",
        "url": "url",
        "use_msi": "useMsi",
        "user": "user",
        "user_id": "userId",
        "username": "username",
        "workspace": "workspace",
        "workspace_key_prefix": "workspaceKeyPrefix",
        "workspace_name": "workspaceName",
        "workspace_prefix": "workspacePrefix",
    }

    __CAMEL_TO_SNAKE_CASE_TABLE = {
        "backendType": "backend_type",
        "accessKey": "access_key",
        "accessToken": "access_token",
        "account": "account",
        "address": "address",
        "authUrl": "auth_url",
        "bucket": "bucket",
        "caFile": "ca_file",
        "cacertFile": "cacert_file",
        "cacertPath": "cacert_path",
        "cert": "cert",
        "certFile": "cert_file",
        "certPath": "cert_path",
        "clientId": "client_id",
        "clientSecret": "client_secret",
        "connStr": "conn_str",
        "container": "container",
        "containerName": "container_name",
        "credentials": "credentials",
        "datacenter": "datacenter",
        "domainId": "domain_id",
        "domainName": "domain_name",
        "encryptionKey": "encryption_key",
        "endpoint": "endpoint",
        "endpoints": "endpoints",
        "environment": "environment",
        "externalId": "external_id",
        "gzip": "gzip",
        "hostname": "hostname",
        "http_auth": "http_auth",
        "iamEndpoint": "iam_endpoint",
        "insecure": "insecure",
        "insecureSkipTlsVerify": "insecure_skip_tls_verify",
        "key": "key",
        "keyFile": "key_file",
        "keyId": "key_id",
        "keyMaterial": "key_material",
        "keyPath": "key_path",
        "lockAddress": "lock_address",
        "lockMethod": "lock_method",
        "msiEndpoint": "msi_endpoint",
        "organization": "organization",
        "outputs": "outputs",
        "password": "password",
        "path": "path",
        "prefix": "prefix",
        "profile": "profile",
        "region": "region",
        "regionName": "region_name",
        "repo": "repo",
        "resourceGroupName": "resource_group_name",
        "roleArn": "role_arn",
        "sasToken": "sas_token",
        "schemaName": "schema_name",
        "scheme": "scheme",
        "secretKey": "secret_key",
        "sessionName": "session_name",
        "sharedCredentialsFile": "shared_credentials_file",
        "skipCertValidation": "skip_cert_validation",
        "storageAccountName": "storage_account_name",
        "stsEndpoint": "sts_endpoint",
        "subpath": "subpath",
        "subscriptionId": "subscription_id",
        "tenantId": "tenant_id",
        "tenantName": "tenant_name",
        "token": "token",
        "unlockAddress": "unlock_address",
        "unlockMethod": "unlock_method",
        "updateMethod": "update_method",
        "url": "url",
        "useMsi": "use_msi",
        "user": "user",
        "userId": "user_id",
        "username": "username",
        "workspace": "workspace",
        "workspaceKeyPrefix": "workspace_key_prefix",
        "workspaceName": "workspace_name",
        "workspacePrefix": "workspace_prefix",
    }
