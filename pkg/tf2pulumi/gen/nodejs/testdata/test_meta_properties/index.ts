import * as pulumi from "@pulumi/pulumi";
import * as aws from "@pulumi/aws";

const r1 = new aws.ec2.Instance("r1", {}, { timeouts: {
    create: "20m",
    delete: "1h",
    update: "5m",
} });
const r2 = new aws.ec2.Instance("r2", {}, { ignoreChanges: ["ami", "arn", "associatePublicIpAddress", "availabilityZone", "cpuCoreCount", "cpuThreadsPerCore", "creditSpecification", "disableApiTermination", "ebsBlockDevices", "ebsOptimized", "ephemeralBlockDevices", "getPasswordData", "hibernation", "hostId", "iamInstanceProfile", "instanceInitiatedShutdownBehavior", "instanceState", "instanceType", "ipv6AddressCount", "ipv6Addresses", "keyName", "metadataOptions", "monitoring", "networkInterfaces", "outpostArn", "passwordData", "placementGroup", "primaryNetworkInterfaceId", "privateDns", "privateIp", "publicDns", "publicIp", "rootBlockDevice", "secondaryPrivateIps", "securityGroups", "sourceDestCheck", "subnetId", "tags", "tenancy", "userData", "userDataBase64", "volumeTags", "vpcSecurityGroupIds"] });
const r3 = new aws.ec2.Instance("r3", {}, { ignoreChanges: ["ami", "networkInterfaces[0].networkInterfaceId", "rootBlockDevice.encrypted", "tags.Creator", "userData", "userDataBase64"] });
