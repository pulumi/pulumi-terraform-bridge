import * as hcl from "hcl";

const vpc = new hcl.VpcAws("my-vpc", {
    cidr: "10.0.0.0/16",
});

export const defaultVpcId = vpc.defaultVpcId;
