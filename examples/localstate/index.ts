import * as tf from "@pulumi/terraform";
import * as path from "path";

const remotestate = new tf.state.RemoteStateReference("localstate", {
   backendType: "local",
   path: path.join(__dirname, "terraform.tfstate"),
});

export const vpcId= remotestate.getOutput("vpc_id");
export const publicSubnetIds = remotestate.getOutput("public_subnet_ids");
export const bucketArn = remotestate.getOutput("bucket_arn");
