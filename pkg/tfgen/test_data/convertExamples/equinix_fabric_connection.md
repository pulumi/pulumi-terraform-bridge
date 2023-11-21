This is a test.
## Example Usage
{{% example %}}

```typescript
import * as pulumi from "@pulumi/pulumi";
import * as equinix from "@equinix-labs/pulumi-equinix";

const config = new pulumi.Config();
const metro = config.get("metro") || "FR";
const speedInMbps = config.getNumber("speedInMbps") || 50;
const fabricPortName = config.require("fabricPortName");
const awsRegion = config.get("awsRegion") || "eu-central-1";
const awsAccountId = config.require("awsAccountId");
const serviceProfileId = equinix.fabric.getServiceProfiles({
    filter: {
        property: "/name",
        operator: "=",
        values: ["AWS Direct Connect"],
    },
}).then(invoke => invoke.data?.[0]?.uuid!);
const portId = equinix.fabric.getPorts({
    filter: {
        name: fabricPortName,
    },
}).then(invoke => invoke.data?.[0]?.uuid!);
const colo2Aws = new equinix.fabric.Connection("colo2Aws", {
    name: "Pulumi-colo2Aws",
    type: "EVPL_VC",
    notifications: [{
        type: "ALL",
        emails: ["example@equinix.com"],
    }],
    bandwidth: speedInMbps,
    redundancy: {
        priority: "PRIMARY",
    },
    aSide: {
        accessPoint: {
            type: "COLO",
            port: {
                uuid: portId,
            },
            linkProtocol: {
                type: "DOT1Q",
                vlanTag: 1234,
            },
        },
    },
    zSide: {
        accessPoint: {
            type: "SP",
            authenticationKey: awsAccountId,
            sellerRegion: awsRegion,
            profile: {
                type: "L2_PROFILE",
                uuid: serviceProfileId,
            },
            location: {
                metroCode: metro,
            },
        },
    },
});
export const connectionId = colo2Aws.id;
export const connectionStatus = colo2Aws.operation.apply(operation => operation.equinixStatus);
export const connectionProviderStatus = colo2Aws.operation.apply(operation => operation.providerStatus);
export const awsDirectConnectId = colo2Aws.zSide.apply(zSide => zSide.accessPoint?.providerConnectionId);
```
```python
import pulumi
import pulumi_equinix as equinix

config = pulumi.Config()
metro = config.get("metro")
if metro is None:
    metro = "FR"
speed_in_mbps = config.get_int("speedInMbps")
if speed_in_mbps is None:
    speed_in_mbps = 50
fabric_port_name = config.require("fabricPortName")
aws_region = config.get("awsRegion")
if aws_region is None:
    aws_region = "eu-central-1"
aws_account_id = config.require("awsAccountId")
service_profile_id = equinix.fabric.get_service_profiles(filter=equinix.fabric.GetServiceProfilesFilterArgs(
    property="/name",
    operator="=",
    values=["AWS Direct Connect"],
)).data[0].uuid
port_id = equinix.fabric.get_ports(filter=equinix.fabric.GetPortsFilterArgs(
    name=fabric_port_name,
)).data[0].uuid
colo2_aws = equinix.fabric.Connection("colo2Aws",
    name="Pulumi-colo2Aws",
    type="EVPL_VC",
    notifications=[equinix.fabric.ConnectionNotificationArgs(
        type="ALL",
        emails=["example@equinix.com"],
    )],
    bandwidth=speed_in_mbps,
    redundancy=equinix.fabric.ConnectionRedundancyArgs(
        priority="PRIMARY",
    ),
    a_side=equinix.fabric.ConnectionASideArgs(
        access_point=equinix.fabric.ConnectionASideAccessPointArgs(
            type="COLO",
            port=equinix.fabric.ConnectionASideAccessPointPortArgs(
                uuid=port_id,
            ),
            link_protocol=equinix.fabric.ConnectionASideAccessPointLinkProtocolArgs(
                type="DOT1Q",
                vlan_tag=1234,
            ),
        ),
    ),
    z_side=equinix.fabric.ConnectionZSideArgs(
        access_point=equinix.fabric.ConnectionZSideAccessPointArgs(
            type="SP",
            authentication_key=aws_account_id,
            seller_region=aws_region,
            profile=equinix.fabric.ConnectionZSideAccessPointProfileArgs(
                type="L2_PROFILE",
                uuid=service_profile_id,
            ),
            location=equinix.fabric.ConnectionZSideAccessPointLocationArgs(
                metro_code=metro,
            ),
        ),
    ))
pulumi.export("connectionId", colo2_aws.id)
pulumi.export("connectionStatus", colo2_aws.operation.equinix_status)
pulumi.export("connectionProviderStatus", colo2_aws.operation.provider_status)
pulumi.export("awsDirectConnectId", colo2_aws.z_side.access_point.provider_connection_id)
```
```go
package main

import (
	"github.com/equinix/pulumi-equinix/sdk/go/equinix/fabric"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		cfg := config.New(ctx, "")
		metro := "FR"
		if param := cfg.Get("metro"); param != "" {
			metro = param
		}
		speedInMbps := 50
		if param := cfg.GetInt("speedInMbps"); param != 0 {
			speedInMbps = param
		}
		fabricPortName := cfg.Require("fabricPortName")
		awsRegion := "eu-central-1"
		if param := cfg.Get("awsRegion"); param != "" {
			awsRegion = param
		}
		awsAccountId := cfg.Require("awsAccountId")
		serviceProfileId := fabric.GetServiceProfiles(ctx, &fabric.GetServiceProfilesArgs{
			Filter: fabric.GetServiceProfilesFilter{
				Property: pulumi.StringRef("/name"),
				Operator: pulumi.StringRef("="),
				Values: []string{
					"AWS Direct Connect",
				},
			},
		}, nil).Data[0].Uuid
		portId := fabric.GetPorts(ctx, &fabric.GetPortsArgs{
			Filter: fabric.GetPortsFilter{
				Name: pulumi.StringRef(fabricPortName),
			},
		}, nil).Data[0].Uuid
		colo2Aws, err := fabric.NewConnection(ctx, "colo2Aws", &fabric.ConnectionArgs{
			Name: pulumi.String("Pulumi-colo2Aws"),
			Type: pulumi.String("EVPL_VC"),
			Notifications: fabric.ConnectionNotificationArray{
				&fabric.ConnectionNotificationArgs{
					Type: pulumi.String("ALL"),
					Emails: pulumi.StringArray{
						pulumi.String("example@equinix.com"),
					},
				},
			},
			Bandwidth: pulumi.Int(speedInMbps),
			Redundancy: &fabric.ConnectionRedundancyArgs{
				Priority: pulumi.String("PRIMARY"),
			},
			ASide: &fabric.ConnectionASideArgs{
				AccessPoint: &fabric.ConnectionASideAccessPointArgs{
					Type: pulumi.String("COLO"),
					Port: &fabric.ConnectionASideAccessPointPortArgs{
						Uuid: *pulumi.String(portId),
					},
					LinkProtocol: &fabric.ConnectionASideAccessPointLinkProtocolArgs{
						Type:    pulumi.String("DOT1Q"),
						VlanTag: pulumi.Int(1234),
					},
				},
			},
			ZSide: &fabric.ConnectionZSideArgs{
				AccessPoint: &fabric.ConnectionZSideAccessPointArgs{
					Type:              pulumi.String("SP"),
					AuthenticationKey: pulumi.String(awsAccountId),
					SellerRegion:      pulumi.String(awsRegion),
					Profile: &fabric.ConnectionZSideAccessPointProfileArgs{
						Type: pulumi.String("L2_PROFILE"),
						Uuid: *pulumi.String(serviceProfileId),
					},
					Location: &fabric.ConnectionZSideAccessPointLocationArgs{
						MetroCode: pulumi.String(metro),
					},
				},
			},
		})
		if err != nil {
			return err
		}
		ctx.Export("connectionId", colo2Aws.ID())
		ctx.Export("connectionStatus", colo2Aws.Operation.ApplyT(func(operation fabric.ConnectionOperation) (*string, error) {
			return &operation.EquinixStatus, nil
		}).(pulumi.StringPtrOutput))
		ctx.Export("connectionProviderStatus", colo2Aws.Operation.ApplyT(func(operation fabric.ConnectionOperation) (*string, error) {
			return &operation.ProviderStatus, nil
		}).(pulumi.StringPtrOutput))
		ctx.Export("awsDirectConnectId", colo2Aws.ZSide.ApplyT(func(zSide fabric.ConnectionZSide) (*string, error) {
			return &zSide.AccessPoint.ProviderConnectionId, nil
		}).(pulumi.StringPtrOutput))
		return nil
	})
}
```
```csharp
using System.Collections.Generic;
using Pulumi;
using Equinix = Pulumi.Equinix;

return await Deployment.RunAsync(() => 
{
    var config = new Config();
    var metro = config.Get("metro") ?? "FR";
    var speedInMbps = config.GetNumber("speedInMbps") ?? 50;
    var fabricPortName = config.Require("fabricPortName");
    var awsRegion = config.Get("awsRegion") ?? "eu-central-1";
    var awsAccountId = config.Require("awsAccountId");
    var serviceProfileId = Equinix.Fabric.GetServiceProfiles.Invoke(new()
    {
        Filter = new Equinix.Fabric.Inputs.GetServiceProfilesFilterInputArgs
        {
            Property = "/name",
            Operator = "=",
            Values = new[]
            {
                "AWS Direct Connect",
            },
        },
    }).Apply(invoke => invoke.Data[0]?.Uuid);

    var portId = Equinix.Fabric.GetPorts.Invoke(new()
    {
        Filter = new Equinix.Fabric.Inputs.GetPortsFilterInputArgs
        {
            Name = fabricPortName,
        },
    }).Apply(invoke => invoke.Data[0]?.Uuid);

    var colo2Aws = new Equinix.Fabric.Connection("colo2Aws", new()
    {
        Name = "Pulumi-colo2Aws",
        Type = "EVPL_VC",
        Notifications = new[]
        {
            new Equinix.Fabric.Inputs.ConnectionNotificationArgs
            {
                Type = "ALL",
                Emails = new[]
                {
                    "example@equinix.com",
                },
            },
        },
        Bandwidth = speedInMbps,
        Redundancy = new Equinix.Fabric.Inputs.ConnectionRedundancyArgs
        {
            Priority = "PRIMARY",
        },
        ASide = new Equinix.Fabric.Inputs.ConnectionASideArgs
        {
            AccessPoint = new Equinix.Fabric.Inputs.ConnectionASideAccessPointArgs
            {
                Type = "COLO",
                Port = new Equinix.Fabric.Inputs.ConnectionASideAccessPointPortArgs
                {
                    Uuid = portId,
                },
                LinkProtocol = new Equinix.Fabric.Inputs.ConnectionASideAccessPointLinkProtocolArgs
                {
                    Type = "DOT1Q",
                    VlanTag = 1234,
                },
            },
        },
        ZSide = new Equinix.Fabric.Inputs.ConnectionZSideArgs
        {
            AccessPoint = new Equinix.Fabric.Inputs.ConnectionZSideAccessPointArgs
            {
                Type = "SP",
                AuthenticationKey = awsAccountId,
                SellerRegion = awsRegion,
                Profile = new Equinix.Fabric.Inputs.ConnectionZSideAccessPointProfileArgs
                {
                    Type = "L2_PROFILE",
                    Uuid = serviceProfileId,
                },
                Location = new Equinix.Fabric.Inputs.ConnectionZSideAccessPointLocationArgs
                {
                    MetroCode = metro,
                },
            },
        },
    });

    return new Dictionary<string, object?>
    {
        ["connectionId"] = colo2Aws.Id,
        ["connectionStatus"] = colo2Aws.Operation.Apply(operation => operation.EquinixStatus),
        ["connectionProviderStatus"] = colo2Aws.Operation.Apply(operation => operation.ProviderStatus),
        ["awsDirectConnectId"] = colo2Aws.ZSide.Apply(zSide => zSide.AccessPoint?.ProviderConnectionId),
    };
});
```
```java
package generated_program;

import com.pulumi.Context;
import com.pulumi.Pulumi;
import com.equinix.pulumi.fabric.Connection;
import com.equinix.pulumi.fabric.ConnectionArgs;
import com.equinix.pulumi.fabric.inputs.ConnectionNotificationArgs;
import com.equinix.pulumi.fabric.inputs.ConnectionRedundancyArgs;
import com.equinix.pulumi.fabric.inputs.ConnectionASideArgs;
import com.equinix.pulumi.fabric.inputs.ConnectionASideAccessPointArgs;
import com.equinix.pulumi.fabric.inputs.ConnectionASideAccessPointPortArgs;
import com.equinix.pulumi.fabric.inputs.ConnectionASideAccessPointLinkProtocolArgs;
import com.equinix.pulumi.fabric.inputs.ConnectionZSideArgs;
import com.equinix.pulumi.fabric.inputs.ConnectionZSideAccessPointArgs;
import com.equinix.pulumi.fabric.inputs.ConnectionZSideAccessPointProfileArgs;
import com.equinix.pulumi.fabric.inputs.ConnectionZSideAccessPointLocationArgs;
import com.equinix.pulumi.fabric.inputs.GetServiceProfilesArgs;
import com.equinix.pulumi.fabric.inputs.GetServiceProfilesFilterArgs;
import com.equinix.pulumi.fabric.inputs.GetPortsArgs;
import com.equinix.pulumi.fabric.inputs.GetPortsFilterArgs;
import com.equinix.pulumi.fabric.FabricFunctions;

public class App {
    public static void main(String[] args) {
        Pulumi.run(App::stack);
    }

    public static void stack(Context ctx) {
        final var config = ctx.config();
        final var metro = config.get("metro").orElse("FR");
        final var speedInMbps = Integer.parseInt(config.get("speedInMbps").orElse("50"));
        final var fabricPortName = config.get("fabricPortName").get().toString();
        final var awsRegion = config.get("awsRegion").orElse("eu-central-1");
        final var awsAccountId = config.get("awsAccountId").get().toString();
        System.out.println(System.getProperty("java.classpath"));
        final var serviceProfileId = FabricFunctions.getServiceProfiles(GetServiceProfilesArgs.builder()
            .filter(GetServiceProfilesFilterArgs.builder()
                .property("/name")
                .operator("=")
                .values("AWS Direct Connect")
                .build())
            .build()).applyValue(data -> data.data().get(0).uuid().get());

        final var portId = FabricFunctions.getPorts(GetPortsArgs.builder()
            .filter(GetPortsFilterArgs.builder()
                .name(fabricPortName)
                .build())
            .build()).applyValue(data -> data.data().get(0).uuid().get());

        var colo2Aws = new Connection("colo2Aws", ConnectionArgs.builder()        
            .name("Pulumi-colo2Aws")
            .type("EVPL_VC")
            .notifications(ConnectionNotificationArgs.builder()
                .type("ALL")
                .emails("example@equinix.com")
                .build())
            .bandwidth(speedInMbps)
            .redundancy(ConnectionRedundancyArgs.builder()
                .priority("PRIMARY")
                .build())
            .aSide(ConnectionASideArgs.builder()
                .accessPoint(ConnectionASideAccessPointArgs.builder()
                    .type("COLO")
                    .port(ConnectionASideAccessPointPortArgs.builder()
                        .uuid(portId)
                        .build())
                    .linkProtocol(ConnectionASideAccessPointLinkProtocolArgs.builder()
                        .type("DOT1Q")
                        .vlanTag(1234)
                        .build())
                    .build())
                .build())
            .zSide(ConnectionZSideArgs.builder()
                .accessPoint(ConnectionZSideAccessPointArgs.builder()
                    .type("SP")
                    .authenticationKey(awsAccountId)
                    .sellerRegion(awsRegion)
                    .profile(ConnectionZSideAccessPointProfileArgs.builder()
                        .type("L2_PROFILE")
                        .uuid(serviceProfileId)
                        .build())
                    .location(ConnectionZSideAccessPointLocationArgs.builder()
                        .metroCode(metro)
                        .build())
                    .build())
                .build())
            .build());

        ctx.export("connectionId", colo2Aws.id());
        ctx.export("connectionStatus", colo2Aws.operation().applyValue(operation -> operation.equinixStatus()));
        ctx.export("connectionProviderStatus", colo2Aws.operation().applyValue(operation -> operation.providerStatus()));
        ctx.export("awsDirectConnectId", colo2Aws.zSide().applyValue(zSide -> zSide.accessPoint().get().providerConnectionId()));
    }
}
```
```yaml
config:
  metro:
    type: string
    default: FR
  speedInMbps:
    type: integer
    default: 50
  fabricPortName:
    type: string
  awsRegion:
    type: string
    default: eu-central-1
  awsAccountId:
    type: string
variables:
  serviceProfileId:
    fn::invoke:
      function: equinix:fabric:getServiceProfiles
      arguments:
        filter:
          property: /name
          operator: "="
          values:
          - AWS Direct Connect
      return: data[0].uuid
  portId:
    fn::invoke:
      function: equinix:fabric:getPorts
      arguments:
        filter:
          name: ${fabricPortName}
      return: data[0].uuid
resources:
  colo2Aws:
    type: equinix:fabric:Connection
    properties:
      name: Pulumi-colo2Aws
      type: EVPL_VC
      notifications:
      - type: ALL
        emails:
        - example@equinix.com
      bandwidth: ${speedInMbps}
      redundancy:
        priority: PRIMARY
      aSide:
        accessPoint:
          type: COLO
          port:
            uuid: ${portId}
          linkProtocol:
            type: DOT1Q
            vlanTag: 1234
      zSide:
        accessPoint:
          type: SP
          authenticationKey: ${awsAccountId}
          sellerRegion: ${awsRegion}
          profile:
            type: L2_PROFILE
            uuid: ${serviceProfileId}
          location:
            metroCode: ${metro}
outputs:
  connectionId: ${colo2Aws.id}
  connectionStatus: ${colo2Aws.operation.equinixStatus}
  connectionProviderStatus: ${colo2Aws.operation.providerStatus}
  awsDirectConnectId: ${colo2Aws.zSide.accessPoint.providerConnectionId}
```
{{% /example %}}

