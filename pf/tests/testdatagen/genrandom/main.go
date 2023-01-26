package main

import (
	random "github.com/pulumi/pulumi-random/sdk/v4/go/random"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {

	pulumi.Run(func(ctx *pulumi.Context) error {
		conf := config.New(ctx, "")
		minn := conf.RequireInt("min")

		if minn > 0 {
			r, err := random.NewRandomInteger(ctx, "r1", &random.RandomIntegerArgs{
				Seed: pulumi.String("pseudo-random-seed"),
				Min:  pulumi.Int(minn),
				Max:  pulumi.Int(100),
			})
			if err != nil {
				return err
			}

			ctx.Export("r.result", r.Result)
		}
		return nil
	})

}
